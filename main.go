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
	ReferrerID  int64   `json:"referrer_id"`
	PromoActive bool    `json:"promo_active"`
	RefCount    int     `json:"ref_count"`
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
	minDep  = 100.0
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

func getU(c telebot.Context) *UserData {
	id := c.Sender().ID
	mu.Lock()
	defer mu.Unlock()
	if _, ok := Storage[id]; !ok {
		Storage[id] = &UserData{ID: id, Role: 0}
	}
	Storage[id].Username = c.Sender().Username
	return Storage[id]
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

	// Назначаем владельца
	mu.Lock()
	if _, ok := Storage[ownerID]; !ok { Storage[ownerID] = &UserData{ID: ownerID} }
	Storage[ownerID].Role = 3
	mu.Unlock()
	saveAll()

	// Клавиатуры
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}
	btnOrder := menu.Text("🚀 Заказать проект")
	btnPrices := menu.Text("💰 Кабинет")
	btnPromo := menu.Text("🎁 Промокод")
	btnSupport := menu.Text("👨‍💻 Поддержка")
	menu.Reply(menu.Row(btnOrder), menu.Row(btnPrices, btnPromo), menu.Row(btnSupport))

	// Кнопки оплаты заказа
	orderPayMenu := &telebot.ReplyMarkup{}
	btnPayBot := orderPayMenu.Data("🤖 Оплатить Бота (120₽)", "buy_bot")
	btnPayPrj := orderPayMenu.Data("🏢 Оплатить Проект (350₽)", "buy_prj")
	orderPayMenu.Inline(orderPayMenu.Row(btnPayBot), orderPayMenu.Row(btnPayPrj))

	var userStates sync.Map

	// --- ВСЕ КОМАНДЫ (СТРОГО В НАЧАЛЕ) ---

	b.Handle("/gen", func(c telebot.Context) error {
		if getU(c).Role < 3 { return nil }
		f := strings.Fields(c.Message().Payload)
		if len(f) < 4 { return c.Send("⚠️ `/gen [код] [сумма] [тип p/v] [кол-во]`", telebot.ModeMarkdown) }
		amt, _ := strconv.ParseFloat(f[1], 64)
		uses, _ := strconv.Atoi(f[3])
		mu.Lock()
		Promos[f[0]] = &Promo{Code: f[0], Amount: amt, IsPerc: f[2] == "p", Uses: uses}
		mu.Unlock()
		saveAll()
		return c.Send(fmt.Sprintf("✅ Промокод `%s` создан!", f[0]), telebot.ModeMarkdown)
	})

	b.Handle("/addbal", func(c telebot.Context) error {
		if getU(c).Role < 3 { return nil }
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

	b.Handle("/stats", func(c telebot.Context) error {
		if getU(c).Role < 1 { return nil }
		mu.RLock()
		defer mu.RUnlock()
		return c.Send(fmt.Sprintf("📊 Юзеров: %d\nПромо: %d", len(Storage), len(Promos)))
	})

	// --- ЛОГИКА ОПЛАТЫ С БАЛАНСА ---

	b.Handle(&btnPayBot, func(c telebot.Context) error {
		u := getU(c)
		price := pBotRUB
		if u.Balance < price { return c.Edit("❌ Недостаточно средств на балансе!") }
		
		mu.Lock()
		u.Balance -= price
		mu.Unlock()
		saveAll()
		
		b.Send(telebot.ChatID(ownerID), fmt.Sprintf("💸 **ОПЛАЧЕНО!**\nЮзер: @%s\nТип: Бот (120₽)", u.Username))
		return c.Edit("✅ Оплата прошла успешно! Админ свяжется с вами.")
	})

	b.Handle(&btnPayPrj, func(c telebot.Context) error {
		u := getU(c)
		price := pPrjRUB
		if u.Balance < price { return c.Edit("❌ Недостаточно средств на балансе!") }
		
		mu.Lock()
		u.Balance -= price
		mu.Unlock()
		saveAll()
		
		b.Send(telebot.ChatID(ownerID), fmt.Sprintf("💸 **ОПЛАЧЕНО!**\nЮзер: @%s\nТип: Проект (350₽)", u.Username))
		return c.Edit("✅ Оплата прошла успешно! Админ свяжется с вами.")
	})

	// --- ОБРАБОТЧИКИ КНОПОК ---

	b.Handle("/start", func(c telebot.Context) error {
		return c.Send("👋 Привет в **XDrezzx Studio**!", menu, telebot.ModeMarkdown)
	})

	b.Handle(&btnOrder, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "order")
		return c.Send("📝 Напишите ТЗ вашего проекта:")
	})

	b.Handle(&btnPrices, func(c telebot.Context) error {
		u := getU(c)
		return c.Send(fmt.Sprintf("👤 **Кабинет**\nID: `%d`\nБаланс: `%.2f ₽`", u.ID, u.Balance), telebot.ModeMarkdown)
	})

	b.Handle(&btnPromo, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "promo")
		return c.Send("🎟 Введите промокод:")
	})

	// --- ГЛАВНЫЙ ОБРАБОТЧИК ТЕКСТА ---
	b.Handle(telebot.OnText, func(c telebot.Context) error {
		u := getU(c)
		state, _ := userStates.Load(u.ID)

		switch state {
		case "order":
			userStates.Delete(u.ID)
			b.Send(telebot.ChatID(ownerID), fmt.Sprintf("📩 **НОВОЕ ТЗ**\nОт: @%s\nТекст: %s", u.Username, c.Text()))
			return c.Send("✅ ТЗ получено! Теперь выберите тип проекта для оплаты с баланса:", orderPayMenu)

		case "promo":
			userStates.Delete(u.ID)
			mu.Lock()
			defer mu.Unlock()
			if p, ok := Promos[c.Text()]; ok && p.Uses > 0 {
				if p.IsPerc { u.PromoActive = true } else { u.Balance += p.Amount }
				p.Uses--
				if p.Uses <= 0 { delete(Promos, c.Text()) }
				saveAll()
				return c.Send("✅ Промокод применен!")
			}
			return c.Send("❌ Код неверный.")
		}
		return nil
	})

	log.Println("Бот запущен. Команды и оплата активны.")
	b.Start()
}
