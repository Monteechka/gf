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
	Balance     float64 `json:"balance"` // Баланс в РУБЛЯХ
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

// Цены в РУБЛЯХ (для прайса)
const (
	pBotRUB = 120.0
	pPrjRUB = 350.0
	minDep  = 100.0 // Минималка 100 руб
)

// Курсы пополнения (к рублю)
const (
	rateStars = 2.0  // 50 звезд * 2 = 100 руб
	rateUAH   = 2.4  // 50 грн * 2.4 = 120 руб
	rateTON   = 90.0 // 1 TON * 90 = 90 руб
)

func loadAll() {
	mu.Lock()
	defer mu.Unlock()
	f1, _ := os.ReadFile(dbFile)
	json.Unmarshal(f1, &Storage)
	f2, _ := os.ReadFile(pmFile)
	json.Unmarshal(f2, &Promos)
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
		Storage[id] = &UserData{ID: id, Role: 0, Balance: 0}
	}
	Storage[id].Username = c.Sender().Username
	return Storage[id]
}

func main() {
	loadAll()

	token := "8749363555:AAFM3L2Yj61gFuvNZz37SsnDx3u3B5dwdIM"
	ownerID := int64(8699513395)
	wallet := "UQBkHHHtTrkYraYeABJAepEr00eMYXKaVUUter4zfNk6eUxm"
	reviewChanID, _ := strconv.ParseInt("-1003953639397", 10, 64)

	// Назначаем владельца при запуске
	mu.Lock()
	if u, ok := Storage[ownerID]; ok {
		u.Role = 3
	} else {
		Storage[ownerID] = &UserData{ID: ownerID, Role: 3}
	}
	mu.Unlock()
	saveAll()

	b, err := telebot.NewBot(telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal(err)
		return
	}

	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}
	btnOrder := menu.Text("🚀 Заказать проект")
	btnPrices := menu.Text("💰 Кабинет")
	btnPromo := menu.Text("🎁 Промокод")
	btnSupport := menu.Text("👨‍💻 Поддержка")
	btnReviews := menu.Text("💬 Отзывы")
	menu.Reply(menu.Row(btnOrder), menu.Row(btnPrices, btnPromo), menu.Row(btnReviews, btnSupport))

	payOpt := &telebot.ReplyMarkup{}
	btnPayStars := payOpt.Data("⭐ Stars", "pay_stars")
	btnPayTON := payOpt.Data("💎 TON", "pay_ton")
	btnPayRUB := payOpt.Data("🇷🇺 RUB", "pay_rub")
	btnPayUAH := payOpt.Data("🇺🇦 UAH", "pay_uah")
	payOpt.Inline(payOpt.Row(btnPayStars, btnPayTON), payOpt.Row(btnPayRUB, btnPayUAH))

	revOpt := &telebot.ReplyMarkup{}
	btnWriteRev := revOpt.Data("✍️ Написать отзыв", "write_rev")
	revOpt.Inline(revOpt.Row(btnWriteRev))

	var userStates sync.Map

	getDisc := func(id int64) int {
		mu.RLock()
		u := Storage[id]
		mu.RUnlock()
		d := 0
		if u == nil {
			return 0
		}
		if u.PromoActive {
			d += 5
		}
		d += (u.RefCount / 3) * 5
		if d > 30 {
			d = 30
		}
		return d
	}

	// --- АДМИН КОМАНДЫ ---

	b.Handle("/stats", func(c telebot.Context) error {
		if getU(c).Role < 1 {
			return nil
		}
		mu.RLock()
		count := len(Storage)
		money := 0.0
		for _, u := range Storage {
			money += u.Balance
		}
		mu.RUnlock()
		return c.Send(fmt.Sprintf("📊 **Статистика:**\nЮзеров: %d\nВсего в системе: %.2f ₽", count, money), telebot.ModeMarkdown)
	})

	b.Handle("/find", func(c telebot.Context) error {
		if getU(c).Role < 1 {
			return nil
		}
		target := strings.Replace(c.Message().Payload, "@", "", -1)
		mu.RLock()
		defer mu.RUnlock()
		for id, u := range Storage {
			if strings.EqualFold(u.Username, target) {
				return c.Send(fmt.Sprintf("🔍 Нашел! ID: `%d` (Баланс: %.2f)", id, u.Balance), telebot.ModeMarkdown)
			}
		}
		return c.Send("❌ Юзер не найден.")
	})

	b.Handle("/promo_info", func(c telebot.Context) error {
		if getU(c).Role < 2 {
			return nil
		}
		mu.RLock()
		p, ok := Promos[c.Message().Payload]
		mu.RUnlock()
		if !ok {
			return c.Send("❌ Промо не найден.")
		}
		return c.Send(fmt.Sprintf("🎟 Промо: `%s`\nСумма: %.2f\nОсталось: %d", p.Code, p.Amount, p.Uses))
	})

	b.Handle("/ban", func(c telebot.Context) error {
		if getU(c).Role < 2 {
			return nil
		}
		id, _ := strconv.ParseInt(c.Message().Payload, 10, 64)
		mu.Lock()
		if u, ok := Storage[id]; ok {
			u.IsBanned = true
		}
		mu.Unlock()
		saveAll()
		return c.Send("🚫 Забанен.")
	})

	b.Handle("/unban", func(c telebot.Context) error {
		if getU(c).Role < 2 {
			return nil
		}
		id, _ := strconv.ParseInt(c.Message().Payload, 10, 64)
		mu.Lock()
		if u, ok := Storage[id]; ok {
			u.IsBanned = false
		}
		mu.Unlock()
		saveAll()
		return c.Send("✅ Разбанен.")
	})

	b.Handle("/reset", func(c telebot.Context) error {
		if getU(c).Role < 3 {
			return nil
		}
		id, _ := strconv.ParseInt(c.Message().Payload, 10, 64)
		mu.Lock()
		if u, ok := Storage[id]; ok {
			u.Balance = 0
			u.RefCount = 0
			u.PromoActive = false
		}
		mu.Unlock()
		saveAll()
		return c.Send("♻️ Профиль сброшен.")
	})

	b.Handle("/clear_promos", func(c telebot.Context) error {
		if getU(c).Role < 3 {
			return nil
		}
		mu.Lock()
		Promos = make(map[string]*Promo)
		mu.Unlock()
		saveAll()
		return c.Send("🧹 Промокоды очищены.")
	})

	b.Handle("/setrole", func(c telebot.Context) error {
		if getU(c).Role < 3 {
			return nil
		}
		f := strings.Fields(c.Message().Payload)
		if len(f) < 2 {
			return c.Send("Юзай: /setrole [ID] [0-3]")
		}
		id, _ := strconv.ParseInt(f[0], 10, 64)
		role, _ := strconv.Atoi(f[1])
		mu.Lock()
		if u, ok := Storage[id]; ok {
			u.Role = role
		}
		mu.Unlock()
		saveAll()
		return c.Send("👑 Роль обновлена.")
	})

	b.Handle("/broadcast", func(c telebot.Context) error {
		if getU(c).Role < 3 {
			return nil
		}
		mu.RLock()
		for id := range Storage {
			b.Send(telebot.ChatID(id), "📢 **ОБЪЯВЛЕНИЕ:**\n\n"+c.Message().Payload, telebot.ModeMarkdown)
		}
		mu.RUnlock()
		return c.Send("🚀 Рассылка завершена.")
	})

	b.Handle("/giveall", func(c telebot.Context) error {
		if getU(c).Role < 3 {
			return nil
		}
		val, _ := strconv.ParseFloat(c.Message().Payload, 64)
		mu.Lock()
		for _, u := range Storage {
			u.Balance += val
		}
		mu.Unlock()
		saveAll()
		return c.Send(fmt.Sprintf("🎁 Всем начислено по %.2f ₽!", val))
	})

	b.Handle("/logs", func(c telebot.Context) error {
		if getU(c).Role < 3 {
			return nil
		}
		return c.Send(&telebot.Document{File: telebot.FromDisk(dbFile)})
	})

	// --- ОСНОВНЫЕ ОБРАБОТЧИКИ ---

	b.Handle("/start", func(c telebot.Context) error {
		u := getU(c)
		if u.IsBanned {
			return c.Send("🚫 Вы забанены.")
		}
		return c.Send("👋 Привет в **XDrezzx Studio**!\nВесь баланс и цены теперь в **рублях**.", menu, telebot.ModeMarkdown)
	})

	b.Handle(&btnPrices, func(c telebot.Context) error {
		u := getU(c)
		d := getDisc(u.ID)
		msg := fmt.Sprintf("👤 **Ваш кабинет:**\n— Баланс: `%.2f ₽`\n— Скидка: `%d%%`\n\n"+
			"💰 **Цены (в рублях):**\n— Создание бота: %.0f ₽\n— Крупный проект: %.0f ₽\n\n"+
			"Выберите способ пополнения (мин. %.0f ₽):",
			u.Balance, d,
			pBotRUB*(1-float64(d)/100), pPrjRUB*(1-float64(d)/100), minDep)
		return c.Send(msg, payOpt, telebot.ModeMarkdown)
	})

	b.Handle(&btnPayStars, func(c telebot.Context) error {
		needed := minDep / rateStars
		return c.Send(fmt.Sprintf("⭐ **Пополнение Stars:**\nМинимальная сумма: **%.0f Stars** (даст 100₽).\nПереведите Stars на @xDrezzx23 и скиньте чек в поддержку.", needed), telebot.ModeMarkdown)
	})

	b.Handle(&btnPayTON, func(c telebot.Context) error {
		needed := minDep / rateTON
		return c.Send(fmt.Sprintf("💎 **Пополнение TON:**\nМинимальная сумма: **%.2f TON**.\nРеквизиты: `%s`\nПосле оплаты скиньте чек в поддержку.", needed, wallet), telebot.ModeMarkdown)
	})

	b.Handle(&btnPayRUB, func(c telebot.Context) error {
		return c.Send(fmt.Sprintf("🇷🇺 **Пополнение RUB:**\nМинимальная сумма: **%.0f ₽**.\nНапишите @xDrezzx23 для получения карты.", minDep), telebot.ModeMarkdown)
	})

	b.Handle(&btnPayUAH, func(c telebot.Context) error {
		needed := minDep / rateUAH
		return c.Send(fmt.Sprintf("🇺🇦 **Пополнение UAH:**\nМинимальная сумма: **%.0f грн** (даст 120₽).\nНапишите @xDrezzx23 для получения карты.", needed), telebot.ModeMarkdown)
	})

	b.Handle(&btnReviews, func(c telebot.Context) error {
		return c.Send("💬 **Отзывы:** @OtzXdrezzx\nХотите оставить отзыв?", revOpt)
	})

	b.Handle(&btnWriteRev, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "write_rev")
		return c.Send("✍️ Напишите ваш отзыв:")
	})

	b.Handle(&btnSupport, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "support")
		return c.Send("👨‍💻 **Поддержка:** Опишите проблему (чек, вопрос, баг). Админ ответит сюда.")
	})

	b.Handle(&btnPromo, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "promo")
		return c.Send("🎟 Введите промокод:")
	})

	b.Handle(telebot.OnText, func(c telebot.Context) error {
		u := getU(c)
		if u.IsBanned {
			return nil
		}
		state, _ := userStates.Load(u.ID)

		switch state {
		case "promo":
			mu.Lock()
			p, ok := Promos[c.Text()]
			if ok && p.Uses > 0 {
				if p.IsPerc {
					u.PromoActive = true
				} else {
					u.Balance += p.Amount
				}
				p.Uses--
				if p.Uses <= 0 {
					delete(Promos, c.Text())
				}
				saveAll()
				c.Send("✅ Промокод успешно применен!")
			} else {
				c.Send("❌ Код недействителен.")
			}
			mu.Unlock()
			userStates.Delete(u.ID)

		case "write_rev":
			msg := fmt.Sprintf("💬 **ОТЗЫВ**\n👤 @%s\n📝 %s", c.Sender().Username, c.Text())
			b.Send(telebot.ChatID(reviewChanID), msg)
			userStates.Delete(u.ID)
			c.Send("✅ Отзыв отправлен!")

		case "support":
			msg := fmt.Sprintf("❓ **ПОДДЕРЖКА**\n👤 @%s (ID: `%d`)\n📝 %s", c.Sender().Username, u.ID, c.Text())
			b.Send(telebot.ChatID(ownerID), msg)
			userStates.Delete(u.ID)
			c.Send("✅ Сообщение отправлено админу.")

		case "order":
			msg := fmt.Sprintf("📩 **ЗАКАЗ**\n👤 @%s (ID: `%d`)\n📝 %s", c.Sender().Username, u.ID, c.Text())
			b.Send(telebot.ChatID(ownerID), msg)
			userStates.Delete(u.ID)
			c.Send("✅ Заказ в обработке!")
		}

		if u.Role >= 1 && c.Message().ReplyTo != nil {
			txt := c.Message().ReplyTo.Text
			if strings.Contains(txt, "ID:") {
				parts := strings.Split(txt, "ID: ")
				targetID, _ := strconv.ParseInt(strings.Fields(parts[1])[0], 10, 64)
				b.Send(telebot.ChatID(targetID), "✉️ **Ответ админа:**\n\n"+c.Text())
				return c.Send("✅ Отправлено пользователю.")
			}
		}
		return nil
	})

	b.Handle(&btnOrder, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "order")
		return c.Send("📝 Пришлите ТЗ проекта:")
	})

	b.Handle("/addbal", func(c telebot.Context) error {
		if getU(c).Role < 3 {
			return nil
		}
		f := strings.Fields(c.Message().Payload)
		if len(f) < 2 {
			return c.Send("⚠️ /addbal [ID] [Сумма в рублях]")
		}
		id, _ := strconv.ParseInt(f[0], 10, 64)
		val, _ := strconv.ParseFloat(f[1], 64)
		mu.Lock()
		if u, ok := Storage[id]; ok {
			u.Balance += val
		}
		mu.Unlock()
		saveAll()
		b.Send(telebot.ChatID(id), fmt.Sprintf("💰 Ваш баланс пополнен на %.2f ₽!", val))
		return c.Send("✅ Баланс успешно начислен.")
	})

	log.Println("Бот запущен. Баланс в рублях.")
	b.Start()
}
