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
	Role        int      `json:"role"` // 0 - юзер, 3 - админ
}

type Promo struct {
	Code   string  `json:"code"`
	Amount float64 `json:"amount"`
	Uses   int     `json:"uses"`
}

var (
	Storage = make(map[int64]*UserData)
	Promos  = make(map[string]*Promo)
	mu      sync.RWMutex
	dbFile  = "users.json"
	pmFile  = "promos.json"
	logFile = "logs.txt"
)

// Константы цен
const (
	pBotRUB = 120.0
	pPrjRUB = 350.0
)

// --- ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ---

func writeLog(msg string) {
	f, _ := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	timestamp := time.Now().Format("02.01 15:04:05")
	f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
}

func loadAll() {
	mu.Lock()
	defer mu.Unlock()
	// Создаем файлы если их нет
	for _, f := range []string{dbFile, pmFile} {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			os.WriteFile(f, []byte("{}"), 0644)
		}
	}
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

	// Установка админа
	admin := getU(ownerID, "Admin")
	admin.Role = 3
	saveAll()

	// Кнопки
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}
	btnOrder := menu.Text("🚀 Заказать проект")
	btnCabinet := menu.Text("💰 Кабинет")
	btnPromo := menu.Text("🎁 Промокод")
	btnSupport := menu.Text("👨‍💻 Поддержка")
	menu.Reply(menu.Row(btnOrder), menu.Row(btnCabinet, btnPromo), menu.Row(btnSupport))

	orderPayMenu := &telebot.ReplyMarkup{}
	btnPayBot := orderPayMenu.Data("🤖 Бот (120₽)", "buy_bot")
	btnPayPrj := orderPayMenu.Data("🏢 Проект (350₽)", "buy_prj")
	orderPayMenu.Inline(orderPayMenu.Row(btnPayBot), orderPayMenu.Row(btnPayPrj))

	var userStates sync.Map

	// --- ОБРАБОТЧИКИ КОМАНД ---

	b.Handle("/start", func(c telebot.Context) error {
		getU(c.Sender().ID, c.Sender().Username)
		caption := "👋 Привет в **XDrezzx Studio**!\n\nЗдесь ты можешь заказать разработку ботов или целых проектов."
		
		// Пробуем отправить фото, если его нет — просто текст
		if _, err := os.Stat("banner.jpg"); err == nil {
			banner := &telebot.Photo{File: telebot.FromDisk("banner.jpg"), Caption: caption}
			return c.Send(banner, menu, telebot.ModeMarkdown)
		}
		return c.Send(caption, menu, telebot.ModeMarkdown)
	})

	// Рассылка (только для админа)
	b.Handle("/send", func(c telebot.Context) error {
		u := getU(c.Sender().ID, c.Sender().Username)
		if u.Role < 3 { return nil }
		
		msg := c.Message().Payload
		if msg == "" { return c.Send("⚠️ Введите текст: `/send Всем привет!`") }

		count := 0
		mu.RLock()
		for id := range Storage {
			b.Send(telebot.ChatID(id), "📢 **ОБЪЯВЛЕНИЕ:**\n\n"+msg, telebot.ModeMarkdown)
			count++
		}
		mu.RUnlock()
		return c.Send(fmt.Sprintf("✅ Рассылка завершена! Получили %d юзеров.", count))
	})

	b.Handle("/gen", func(c telebot.Context) error {
		if getU(c.Sender().ID, c.Sender().Username).Role < 3 { return nil }
		f := strings.Fields(c.Message().Payload)
		if len(f) < 3 { return c.Send("⚠️ `/gen [код] [сумма] [кол-во]`") }
		amt, _ := strconv.ParseFloat(f[1], 64)
		uses, _ := strconv.Atoi(f[2])
		mu.Lock()
		Promos[f[0]] = &Promo{Code: f[0], Amount: amt, Uses: uses}
		mu.Unlock()
		saveAll()
		return c.Send(fmt.Sprintf("✅ Промокод `%s` на %.2f ₽ создан.", f[0], amt))
	})

	// --- ОБРАБОТЧИКИ КНОПОК ---

	b.Handle(&btnCabinet, func(c telebot.Context) error {
		u := getU(c.Sender().ID, c.Sender().Username)
		return c.Send(fmt.Sprintf("👤 **Ваш профиль**\n\n🆔 ID: `%d`\n💰 Баланс: `%.2f ₽`", u.ID, u.Balance), telebot.ModeMarkdown)
	})

	b.Handle(&btnSupport, func(c telebot.Context) error {
		return c.Send("👨‍💻 **Техническая поддержка**\n\nЕсли возникли вопросы: \n• @xDrezzx23\n• @sshadow_k1ngg", telebot.ModeMarkdown)
	})

	b.Handle(&btnOrder, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "order")
		return c.Send("📝 Опишите кратко, что вам нужно:")
	})

	b.Handle(&btnPromo, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "promo")
		return c.Send("🎁 Введите ваш промокод:")
	})

	// --- ЛОГИКА ОПЛАТЫ ---

	handlePayment := func(c telebot.Context, price float64, name string) error {
		u := getU(c.Sender().ID, c.Sender().Username)
		if u.Balance < price {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Недостаточно средств!", ShowAlert: true})
		}
		mu.Lock()
		u.Balance -= price
		mu.Unlock()
		saveAll()

		writeLog(fmt.Sprintf("Юзер %d купил %s за %.2f", u.ID, name, price))
		b.Send(telebot.ChatID(ownerID), fmt.Sprintf("💸 **ОПЛАЧЕНО!**\nОт: @%s\nТип: %s\nСписано: %.2f ₽", u.Username, name, price))
		return c.Edit("✅ Оплата прошла! Админ получил уведомление и свяжется с вами для уточнения деталей.")
	}

	b.Handle(&btnPayBot, func(c telebot.Context) error { return handlePayment(c, pBotRUB, "Бот") })
	b.Handle(&btnPayPrj, func(c telebot.Context) error { return handlePayment(c, pPrjRUB, "Проект") })

	// --- ОБРАБОТКА ТЕКСТА ---

	b.Handle(telebot.OnText, func(c telebot.Context) error {
		u := getU(c.Sender().ID, c.Sender().Username)
		state, _ := userStates.Load(u.ID)

		if state == "order" {
			userStates.Delete(u.ID)
			b.Send(telebot.ChatID(ownerID), fmt.Sprintf("📩 **НОВОЕ ТЗ**\nЮзер: @%s\nID: %d\n\n%s", u.Username, u.ID, c.Text()))
			return c.Send("🎯 ТЗ сохранено. Теперь выберите вариант для оплаты:", orderPayMenu)
		}

		if state == "promo" {
			userStates.Delete(u.ID)
			code := strings.TrimSpace(c.Text())
			mu.Lock()
			defer mu.Unlock()

			p, ok := Promos[code]
			if !ok || p.Uses <= 0 { return c.Send("❌ Код не найден или истек.") }

			for _, used := range u.UsedPromos {
				if used == code { return c.Send("❌ Вы уже использовали этот код!") }
			}

			u.Balance += p.Amount
			u.UsedPromos = append(u.UsedPromos, code)
			p.Uses--
			if p.Uses <= 0 { delete(Promos, code) }
			
			writeLog(fmt.Sprintf("Юзер %d активировал промо %s на %.2f", u.ID, code, p.Amount))
			saveAll()
			return c.Send(fmt.Sprintf("✅ Баланс пополнен на %.2f ₽!", p.Amount))
		}
		return nil
	})

	log.Println("Studio Bot is running...")
	b.Start()
}
