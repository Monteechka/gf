package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/telebot.v3"
)

// --- СТРУКТУРЫ ---

type UserData struct {
	ID          int64    `json:"id"`           // Telegram ID (для отправки сообщений)
	BotID       int      `json:"bot_id"`       // Внутренний ID (например, 1001)
	Username    string   `json:"username"`
	Balance     float64  `json:"balance"`
	Role        int      `json:"role"`         // 0-юзер, 2-Старший Админ, 3-Владелец
}

var (
	Storage = make(map[int64]*UserData)
	mu      sync.RWMutex
	dbFile  = "users.json"
)

const (
	pBotRUB = 120.0
	pPrjRUB = 350.0
)

func loadAll() {
	mu.Lock()
	defer mu.Unlock()
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		os.WriteFile(dbFile, []byte("{}"), 0644)
	}
	f, _ := os.ReadFile(dbFile)
	json.Unmarshal(f, &Storage)
}

func saveAll() {
	mu.RLock()
	defer mu.RUnlock()
	d, _ := json.MarshalIndent(Storage, "", "  ")
	os.WriteFile(dbFile, d, 0644)
}

// Получение юзера или создание нового с уникальным BotID
func getU(id int64, username string) *UserData {
	mu.Lock()
	defer mu.Unlock()
	if u, ok := Storage[id]; ok {
		u.Username = username
		return u
	}
	
	// Генерируем новый BotID (1000 + количество юзеров)
	newBotID := 1001 + len(Storage)
	newU := &UserData{
		ID:       id,
		BotID:    newBotID,
		Username: username,
		Role:     0,
		Balance:  0,
	}
	Storage[id] = newU
	return newU
}

// Поиск юзера по BotID (для пополнения)
func findByBotID(botID int) *UserData {
	mu.RLock()
	defer mu.RUnlock()
	for _, u := range Storage {
		if u.BotID == botID {
			return u
		}
	}
	return nil
}

func main() {
	loadAll()

	token := "8749363555:AAFM3L2Yj61gFuvNZz37SsnDx3u3B5dwdIM"
	ownerID := int64(8699513395)

	b, err := telebot.NewBot(telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil { log.Fatal(err) }

	owner := getU(ownerID, "Admin")
	owner.Role = 3
	saveAll()

	// Кнопки
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}
	btnOrder := menu.Text("🚀 Заказать проект")
	btnDeposit := menu.Text("💳 Пополнить баланс")
	btnCabinet := menu.Text("💰 Кабинет")
	btnSupport := menu.Text("👨‍💻 Поддержка")
	
	menu.Reply(menu.Row(btnOrder), menu.Row(btnDeposit, btnCabinet), menu.Row(btnSupport))

	// --- ОБРАБОТЧИКИ ---

	b.Handle("/start", func(c telebot.Context) error {
		getU(c.Sender().ID, c.Sender().Username)
		saveAll()
		txt := "👋 Привет в **XDrezzx Studio**!"
		if _, err := os.Stat("banner.jpg"); err == nil {
			return c.Send(&telebot.Photo{File: telebot.FromDisk("banner.jpg"), Caption: txt}, menu, telebot.ModeMarkdown)
		}
		return c.Send(txt, menu, telebot.ModeMarkdown)
	})

	b.Handle(&btnCabinet, func(c telebot.Context) error {
		u := getU(c.Sender().ID, c.Sender().Username)
		return c.Send(fmt.Sprintf("👤 **Ваш личный кабинет**\n\n🆔 Ваш ID: `%d`\n💰 Баланс: `%.2f ₽`", u.BotID, u.Balance), telebot.ModeMarkdown)
	})

	b.Handle(&btnDeposit, func(c telebot.Context) error {
		u := getU(c.Sender().ID, c.Sender().Username)
		msg := fmt.Sprintf("💳 **Пополнение баланса**\n\nВаш номер аккаунта: `%d`\n\nКурсы:\n⭐ 50 Stars = 100₽\n🇺🇦 50₴ = 120₽\n🇷🇺 100₽ = 100₽\n💎 1 TON = 90₽\n\nДля оплаты напишите @xDrezzx23", u.BotID)
		return c.Send(msg, telebot.ModeMarkdown)
	})

	// --- АДМИНКА ---

	// Команда пополнения: /addbal [BotID] [Сумма]
	b.Handle("/addbal", func(c telebot.Context) error {
		admin := getU(c.Sender().ID, c.Sender().Username)
		if admin.Role < 2 { return nil }

		f := strings.Fields(c.Message().Payload)
		if len(f) < 2 { return c.Send("⚠️ Формат: `/addbal [ID бота] [сумма]`\nПример: `/addbal 1001 500`") }

		bid, _ := strconv.Atoi(f[0])
		amount, _ := strconv.ParseFloat(f[1], 64)

		target := findByBotID(bid)
		if target == nil {
			return c.Send("❌ Пользователь с таким ID не найден!")
		}

		mu.Lock()
		target.Balance += amount
		mu.Unlock()
		saveAll()

		b.Send(telebot.ChatID(target.ID), fmt.Sprintf("💰 Ваш баланс пополнен на **%.2f ₽**!", amount), telebot.ModeMarkdown)
		return c.Send(fmt.Sprintf("✅ Баланс аккаунта %d ( @%s ) успешно пополнен.", bid, target.Username))
	})

	b.Handle(&btnSupport, func(c telebot.Context) error {
		return c.Send("👨‍💻 **Техподдержка:** @xDrezzx23, @sshadow_k1ngg")
	})

	// Остальная логика заказов остается такой же...
	
	log.Println("Studio Bot запущен на внутренних ID!")
	b.Start()
}
