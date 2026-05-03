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
	ID          int64    `json:"id"`
	Username    string   `json:"username"`
	Balance     float64  `json:"balance"`
	UsedPromos  []string `json:"used_promos"`
	Role        int      `json:"role"` // 0-юзер, 2-Старший Админ, 3-Владелец
}

var (
	Storage = make(map[int64]*UserData)
	mu      sync.RWMutex
	dbFile  = "users.json"
)

// Цены на услуги
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

	// Настройка владельца
	owner := getU(ownerID, "Admin")
	owner.Role = 3
	saveAll()

	// --- КЛАВИАТУРЫ ---
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}
	btnOrder := menu.Text("🚀 Заказать проект")
	btnDeposit := menu.Text("💳 Пополнить баланс")
	btnCabinet := menu.Text("💰 Кабинет")
	btnPromo := menu.Text("🎁 Промо")
	btnSupport := menu.Text("👨‍💻 Поддержка")
	
	menu.Reply(
		menu.Row(btnOrder),
		menu.Row(btnDeposit, btnCabinet),
		menu.Row(btnPromo, btnSupport),
	)

	// Кнопка для перехода в ЛС к тебе
	depositMenu := &telebot.ReplyMarkup{}
	btnWriteAdmin := depositMenu.URL("📨 Написать админу для оплаты", "https://t.me/xDrezzx23")
	depositMenu.Inline(depositMenu.Row(btnWriteAdmin))

	orderPayMenu := &telebot.ReplyMarkup{}
	btnPayBot := orderPayMenu.Data("🤖 Оплатить Бота (120₽)", "buy_bot")
	btnPayPrj := orderPayMenu.Data("🏢 Оплатить Проект (350₽)", "buy_prj")
	orderPayMenu.Inline(orderPayMenu.Row(btnPayBot), orderPayMenu.Row(btnPayPrj))

	var userStates sync.Map

	// --- ОБРАБОТЧИКИ ---

	b.Handle("/start", func(c telebot.Context) error {
		getU(c.Sender().ID, c.Sender().Username)
		txt := "👋 Привет в **XDrezzx Studio**!\n\nИспользуй кнопки ниже для навигации."
		if _, err := os.Stat("banner.jpg"); err == nil {
			return c.Send(&telebot.Photo{File: telebot.FromDisk("banner.jpg"), Caption: txt}, menu, telebot.ModeMarkdown)
		}
		return c.Send(txt, menu, telebot.ModeMarkdown)
	})

	// Кнопка Пополнить (Твои курсы)
	b.Handle(&btnDeposit, func(c telebot.Context) error {
		msg := "💳 **Пополнение баланса**\n\n" +
			"Для пополнения напишите администратору. Мы принимаем:\n\n" +
			"🌟 **Telegram Stars:** 50 ⭐ = 100₽\n" +
			"🇺🇦 **Гривны (UAH):** 50₴ = 120₽\n" +
			"🇷🇺 **Рубли (RUB):** 100₽ = 100₽\n" +
			"💎 **TON:** 1 TON = 90₽\n\n" +
			"После оплаты скиньте скриншот чека админу."
		return c.Send(msg, depositMenu, telebot.ModeMarkdown)
	})

	b.Handle(&btnCabinet, func(c telebot.Context) error {
		u := getU(c.Sender().ID, c.Sender().Username)
		return c.Send(fmt.Sprintf("👤 **Ваш кабинет**\n\n🆔 ID: `%d`\n💰 Баланс: `%.2f ₽`", u.ID, u.Balance), telebot.ModeMarkdown)
	})

	b.Handle(&btnSupport, func(c telebot.Context) error {
		return c.Send("👨‍💻 **Техподдержка:**\n1. @xDrezzx23\n2. @sshadow_k1ngg", telebot.ModeMarkdown)
	})

	// --- АДМИН КОМАНДЫ (Для тебя и Старших Админов) ---

	// Выдача баланса: /addbal [ID] [Сумма]
	b.Handle("/addbal", func(c telebot.Context) error {
		u := getU(c.Sender().ID, c.Sender().Username)
		if u.Role < 2 { return nil } // Только 2+ уровень

		f := strings.Fields(c.Message().Payload)
		if len(f) < 2 { return c.Send("⚠️ Ошибка. Нужно: `/addbal 1234567 100`") }

		targetID, _ := strconv.ParseInt(f[0], 10, 64)
		amount, _ := strconv.ParseFloat(f[1], 64)

		mu.Lock()
		if target, ok := Storage[targetID]; ok {
			target.Balance += amount
		} else {
			mu.Unlock()
			return c.Send("❌ Юзер не найден в базе.")
		}
		mu.Unlock()
		saveAll()

		b.Send(telebot.ChatID(targetID), fmt.Sprintf("💰 Ваш баланс пополнен на **%.2f ₽**!", amount), telebot.ModeMarkdown)
		return c.Send("✅ Баланс успешно начислен.")
	})

	// Назначить старшего админа (только ты): /setadmin [ID]
	b.Handle("/setadmin", func(c telebot.Context) error {
		if c.Sender().ID != ownerID { return nil }
		targetID, _ := strconv.ParseInt(c.Message().Payload, 10, 64)
		
		mu.Lock()
		if target, ok := Storage[targetID]; ok {
			target.Role = 2
		}
		mu.Unlock()
		saveAll()
		return c.Send("✅ Пользователь теперь Старший Администратор.")
	})

	// Рассылка
	b.Handle("/send", func(c telebot.Context) error {
		if getU(c.Sender().ID, c.Sender().Username).Role < 2 { return nil }
		msg := c.Message().Payload
		if msg == "" { return c.Send("⚠️ Напиши текст рассылки.") }

		mu.RLock()
		for id := range Storage {
			b.Send(telebot.ChatID(id), "📢 **Уведомление:**\n\n"+msg, telebot.ModeMarkdown)
		}
		mu.RUnlock()
		return c.Send("✅ Рассылка завершена.")
	})

	// --- ЗАКАЗЫ И ОПЛАТА ---

	b.Handle(&btnOrder, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "order")
		return c.Send("📝 Пришлите ТЗ вашего проекта (одним сообщением):")
	})

	b.Handle(telebot.OnText, func(c telebot.Context) error {
		u := getU(c.Sender().ID, c.Sender().Username)
		state, _ := userStates.Load(u.ID)

		if state == "order" {
			userStates.Delete(u.ID)
			b.Send(telebot.ChatID(ownerID), fmt.Sprintf("📩 **НОВОЕ ТЗ**\nОт: @%s (%d)\n\n%s", u.Username, u.ID, c.Text()))
			return c.Send("✅ ТЗ принято! Теперь выберите товар для оплаты с баланса:", orderPayMenu)
		}
		return nil
	})

	// Списание денег при покупке
	payFunc := func(c telebot.Context, cost float64, product string) error {
		u := getU(c.Sender().ID, c.Sender().Username)
		if u.Balance < cost {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ У вас недостаточно денег на балансе. Пополните через админа!", ShowAlert: true})
		}
		mu.Lock()
		u.Balance -= cost
		mu.Unlock()
		saveAll()
		
		b.Send(telebot.ChatID(ownerID), fmt.Sprintf("💸 **ПОКУПКА!**\nЮзер: @%s\nТовар: %s\nСписано: %.2f₽", u.Username, product, cost))
		return c.Edit("✅ Оплачено! Мы начали работу над вашим заказом.")
	}

	b.Handle(&btnPayBot, func(c telebot.Context) error { return payFunc(c, pBotRUB, "Бот") })
	b.Handle(&btnPayPrj, func(c telebot.Context) error { return payFunc(c, pPrjRUB, "Проект") })

	log.Println("Studio Bot запущен!")
	b.Start()
}
