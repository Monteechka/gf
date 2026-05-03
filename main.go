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
	ID          int64   `json:"id"`
	Username    string  `json:"username"`
	Balance     float64 `json:"balance"`
	UsedPromos  []string `json:"used_promos"` // Чтобы не абузили промо
	IsBanned    bool    `json:"is_banned"`
	Role        int     `json:"role"`
}

type Promo struct {
	Code   string  `json:"code"`
	Amount float64 `json:"amount"`
	IsPerc bool    `json:"is_perc"`
	Uses   int     `json:"uses"`
}

var (
	Storage = make(map[int64]*UserData)
	Promos  = make(map[string]*Promo)
	mu      sync.RWMutex
	dbFile  = "users.json"
	pmFile  = "promos.json"
)

const (
	pBotRUB = 120.0
	pPrjRUB = 350.0
)

func loadAll() {
	mu.Lock()
	defer mu.Unlock()
	f1, _ := os.ReadFile(dbFile); json.Unmarshal(f1, &Storage)
	f2, _ := os.ReadFile(pmFile); json.Unmarshal(f2, &Promos)
}

func saveAll() {
	mu.RLock()
	defer mu.RUnlock()
	d1, _ := json.MarshalIndent(Storage, "", "  ")
	os.WriteFile(dbFile, d1, 0644)
	d2, _ := json.MarshalIndent(Promos, "", "  ")
	os.WriteFile(pmFile, d2, 0644)
}

func getU(id int64, username string) *UserData {
	mu.Lock()
	defer mu.Unlock()
	if u, ok := Storage[id]; ok {
		u.Username = username
		return u
	}
	newU := &UserData{ID: id, Username: username, Role: 0, UsedPromos: []string{}}
	Storage[id] = newU
	return newU
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

	// Назначаем владельца при запуске
	uOwner := getU(ownerID, "Owner")
	uOwner.Role = 3
	saveAll()

	// Клавиатуры
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}
	btnOrder := menu.Text("🚀 Заказать проект")
	btnCabinet := menu.Text("💰 Кабинет")
	btnPromo := menu.Text("🎁 Промокод")
	btnSupport := menu.Text("👨‍💻 Поддержка")
	menu.Reply(menu.Row(btnOrder), menu.Row(btnCabinet, btnPromo), menu.Row(btnSupport))

	orderPayMenu := &telebot.ReplyMarkup{}
	btnPayBot := orderPayMenu.Data("🤖 Оплатить Бота (120₽)", "buy_bot")
	btnPayPrj := orderPayMenu.Data("🏢 Оплатить Проект (350₽)", "buy_prj")
	orderPayMenu.Inline(orderPayMenu.Row(btnPayBot), orderPayMenu.Row(btnPayPrj))

	var userStates sync.Map

	// --- КОМАНДЫ ---

	b.Handle("/start", func(c telebot.Context) error {
		banner := &telebot.Photo{File: telebot.FromDisk("banner.jpg"), Caption: "👋 Привет в **XDrezzx Studio**!\n\nИспользуй меню ниже для работы."}
		return c.Send(banner, menu, telebot.ModeMarkdown)
	})

	b.Handle("/gen", func(c telebot.Context) error {
		if getU(c.Sender().ID, c.Sender().Username).Role < 3 { return nil }
		f := strings.Fields(c.Message().Payload)
		if len(f) < 4 { return c.Send("⚠️ `/gen [код] [сумма] [тип p/v] [кол-во]`") }
		amt, _ := strconv.ParseFloat(f[1], 64)
		uses, _ := strconv.Atoi(f[3])
		mu.Lock()
		Promos[f[0]] = &Promo{Code: f[0], Amount: amt, IsPerc: f[2] == "p", Uses: uses}
		mu.Unlock()
		saveAll()
		return c.Send(fmt.Sprintf("✅ Промокод `%s` создан на %d юзов!", f[0], uses))
	})

	b.Handle("/addbal", func(c telebot.Context) error {
		if getU(c.Sender().ID, c.Sender().Username).Role < 3 { return nil }
		f := strings.Fields(c.Message().Payload)
		if len(f) < 2 { return c.Send("⚠️ `/addbal [ID] [сумма]`") }
		id, _ := strconv.ParseInt(f[0], 10, 64)
		val, _ := strconv.ParseFloat(f[1], 64)
		mu.Lock()
		if u, ok := Storage[id]; ok { u.Balance += val }
		mu.Unlock()
		saveAll()
		b.Send(telebot.ChatID(id), fmt.Sprintf("💰 Баланс пополнен на %.2f ₽!", val))
		return c.Send("✅ Готово.")
	})

	// --- ОБРАБОТЧИКИ ---

	b.Handle(&btnCabinet, func(c telebot.Context) error {
		u := getU(c.Sender().ID, c.Sender().Username)
		return c.Send(fmt.Sprintf("👤 **Кабинет**\n\n🆔 ID: `%d`\n💰 Баланс: `%.2f ₽`", u.ID, u.Balance), telebot.ModeMarkdown)
	})

	b.Handle(&btnSupport, func(c telebot.Context) error {
		return c.Send("👨‍💻 **Техподдержка:**\n1. @xDrezzx23\n2. @sshadow_k1ngg\n\nПишите по любым вопросам!", telebot.ModeMarkdown)
	})

	b.Handle(&btnPromo, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "promo")
		return c.Send("🎟 Введите промокод:")
	})

	b.Handle(&btnOrder, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "order")
		return c.Send("📝 Напишите ТЗ вашего проекта (что должен делать бот/сайт):")
	})

	// Оплата Бота
	b.Handle(&btnPayBot, func(c telebot.Context) error {
		u := getU(c.Sender().ID, c.Sender().Username)
		if u.Balance < pBotRUB {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Недостаточно денег!", ShowAlert: true})
		}
		mu.Lock()
		u.Balance -= pBotRUB
		mu.Unlock()
		saveAll()
		b.Send(telebot.ChatID(ownerID), fmt.Sprintf("🔔 **НОВЫЙ ЗАКАЗ!**\nЮзер: @%s (%d)\nТип: Бот (120₽)", u.Username, u.ID))
		return c.Edit("✅ Оплачено! Админ скоро напишет вам.")
	})

	// Текст (ТЗ и Промо)
	b.Handle(telebot.OnText, func(c telebot.Context) error {
		u := getU(c.Sender().ID, c.Sender().Username)
		state, _ := userStates.Load(u.ID)

		if state == "order" {
			userStates.Delete(u.ID)
			b.Send(telebot.ChatID(ownerID), fmt.Sprintf("📩 **НОВОЕ ТЗ**\nОт: @%s\nТекст: %s", u.Username, c.Text()))
			return c.Send("✅ ТЗ отправлено админу. Теперь выберите вариант оплаты:", orderPayMenu)
		}

		if state == "promo" {
			userStates.Delete(u.ID)
			code := c.Text()
			mu.Lock()
			p, ok := Promos[code]
			if !ok || p.Uses <= 0 {
				mu.Unlock()
				return c.Send("❌ Промокод не существует или закончился.")
			}

			// Проверка на повторное использование
			for _, used := range u.UsedPromos {
				if used == code {
					mu.Unlock()
					return c.Send("❌ Вы уже вводили этот код!")
				}
			}

			u.Balance += p.Amount
			u.UsedPromos = append(u.UsedPromos, code)
			p.Uses--
			if p.Uses <= 0 { delete(Promos, code) }
			mu.Unlock()
			saveAll()
			return c.Send(fmt.Sprintf("✅ Успешно! Зачислено %.2f ₽", p.Amount))
		}
		return nil
	})

	log.Println("Studio Bot запущен...")
	b.Start()
}
