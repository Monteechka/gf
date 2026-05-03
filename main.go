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

const (
	rateStars = 2.0
	rateUAH   = 2.4
	rateTON   = 90.0
)

// --- СИСТЕМНЫЕ ФУНКЦИИ ---

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

	// Авто-назначение Владельца
	mu.Lock()
	if _, ok := Storage[ownerID]; !ok {
		Storage[ownerID] = &UserData{ID: ownerID}
	}
	Storage[ownerID].Role = 3
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

	// Клавиатуры
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
		defer mu.RUnlock()
		u := Storage[id]
		if u == nil { return 0 }
		d := 0
		if u.PromoActive { d += 5 }
		d += (u.RefCount / 3) * 5
		if d > 30 { d = 30 }
		return d
	}

	// --- АДМИН КОМАНДЫ ---

	// Создание промокода: /gen [код] [сумма] [тип p/v] [кол-во]
	b.Handle("/gen", func(c telebot.Context) error {
		if getU(c).Role < 3 { return nil }
		f := strings.Fields(c.Message().Payload)
		if len(f) < 4 { return c.Send("⚠️ /gen [код] [сумма] [тип p/v] [кол-во]") }
		
		isPerc := f[2] == "p"
		amt, _ := strconv.ParseFloat(f[1], 64)
		uses, _ := strconv.Atoi(f[3])

		mu.Lock()
		Promos[f[0]] = &Promo{Code: f[0], Amount: amt, IsPerc: isPerc, Uses: uses}
		mu.Unlock()
		saveAll()
		return c.Send(fmt.Sprintf("✅ Промокод `%s` на %s создан!", f[0], f[1]), telebot.ModeMarkdown)
	})

	b.Handle("/del_promo", func(c telebot.Context) error {
		if getU(c).Role < 3 { return nil }
		mu.Lock()
		delete(Promos, c.Message().Payload)
		mu.Unlock()
		saveAll()
		return c.Send("🗑 Промокод удален.")
	})

	b.Handle("/unreg", func(c telebot.Context) error {
		if getU(c).Role < 3 { return nil }
		id, _ := strconv.ParseInt(c.Message().Payload, 10, 64)
		mu.Lock()
		delete(Storage, id)
		mu.Unlock()
		saveAll()
		return c.Send("🗑 Юзер полностью удален из базы.")
	})

	b.Handle("/stats", func(c telebot.Context) error {
		if getU(c).Role < 1 { return nil }
		mu.RLock()
		defer mu.RUnlock()
		total := 0.0
		for _, u := range Storage { total += u.Balance }
		return c.Send(fmt.Sprintf("📊 **Статистика:**\nЮзеров: %d\nВсего денег: %.2f ₽\nПромокодов: %d", len(Storage), total, len(Promos)), telebot.ModeMarkdown)
	})

	b.Handle("/addbal", func(c telebot.Context) error {
		if getU(c).Role < 3 { return nil }
		f := strings.Fields(c.Message().Payload)
		if len(f) < 2 { return c.Send("⚠️ /addbal [ID] [сумма]") }
		id, _ := strconv.ParseInt(f[0], 10, 64)
		val, _ := strconv.ParseFloat(f[1], 64)
		
		mu.Lock()
		if u, ok := Storage[id]; ok {
			u.Balance += val
		} else {
			mu.Unlock()
			return c.Send("❌ Юзер не заходил в бот.")
		}
		mu.Unlock()
		saveAll()
		b.Send(telebot.ChatID(id), fmt.Sprintf("💰 Ваш баланс пополнен на %.2f ₽!", val))
		return c.Send("✅ Успешно!")
	})

	// --- БАЗОВЫЕ КОМАНДЫ ---

	b.Handle("/start", func(c telebot.Context) error {
		u := getU(c)
		if u.IsBanned { return c.Send("🚫 Доступ заблокирован.") }
		return c.Send("👋 Добро пожаловать в **XDrezzx Studio**!", menu, telebot.ModeMarkdown)
	})

	b.Handle("/myid", func(c telebot.Context) error {
		return c.Send(fmt.Sprintf("🆔 Ваш ID: `%d`", c.Sender().ID), telebot.ModeMarkdown)
	})

	b.Handle(&btnPrices, func(c telebot.Context) error {
		u := getU(c)
		d := getDisc(u.ID)
		msg := fmt.Sprintf("👤 **Кабинет:**\nID: `%d`\nБаланс: `%.2f ₽`\nСкидка: `%d%%`\n\n"+
			"Выберите способ пополнения (мин. %.0f ₽):", u.ID, u.Balance, d, minDep)
		return c.Send(msg, payOpt, telebot.ModeMarkdown)
	})

	// Обработка ввода (Промо, Поддержка, ТЗ)
	b.Handle(telebot.OnText, func(c telebot.Context) error {
		id := c.Sender().ID
		u := getU(c)
		if u.IsBanned { return nil }
		state, _ := userStates.Load(id)

		switch state {
		case "promo":
			mu.Lock()
			p, ok := Promos[c.Text()]
			if ok && p.Uses > 0 {
				if p.IsPerc { u.PromoActive = true } else { u.Balance += p.Amount }
				p.Uses--
				if p.Uses <= 0 { delete(Promos, c.Text()) }
				c.Send("✅ Успешно применено!")
			} else {
				c.Send("❌ Код не найден или истек.")
			}
			mu.Unlock()
			saveAll()
			userStates.Delete(id)

		case "support":
			b.Send(telebot.ChatID(ownerID), fmt.Sprintf("❓ **SUPPORT**\nОт: @%s (ID: `%d`)\nТекст: %s", c.Sender().Username, id, c.Text()))
			userStates.Delete(id)
			c.Send("✅ Отправлено админу.")

		case "order":
			b.Send(telebot.ChatID(ownerID), fmt.Sprintf("📩 **ЗАКАЗ**\nОт: @%s (ID: `%d`)\nТЗ: %s", c.Sender().Username, id, c.Text()))
			userStates.Delete(id)
			c.Send("✅ Ваш заказ принят в работу!")
		}

		// Reply админа пользователю
		if u.Role >= 1 && c.Message().ReplyTo != nil {
			txt := c.Message().ReplyTo.Text
			if strings.Contains(txt, "ID: ") {
				parts := strings.Split(txt, "ID: ")
				targetID, _ := strconv.ParseInt(strings.Fields(parts[1])[0], 10, 64)
				b.Send(telebot.ChatID(targetID), "✉️ **Ответ администрации:**\n\n"+c.Text())
				return c.Send("✅ Ответ доставлен.")
			}
		}
		return nil
	})

	// Состояния кнопок
	b.Handle(&btnPromo, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "promo")
		return c.Send("🎟 Введите промокод:")
	})
	b.Handle(&btnSupport, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "support")
		return c.Send("👨‍💻 Опишите вашу проблему:")
	})
	b.Handle(&btnOrder, func(c telebot.Context) error {
		userStates.Store(c.Sender().ID, "order")
		return c.Send("📝 Пришлите описание вашего проекта (ТЗ):")
	})

	// Обработка инлайн-кнопок оплаты
	b.Handle(&btnPayStars, func(c telebot.Context) error {
		return c.Send(fmt.Sprintf("⭐ Минимум: **%.0f Stars**.\nПеревод на @xDrezzx23", minDep/rateStars))
	})
	b.Handle(&btnPayTON, func(c telebot.Context) error {
		return c.Send(fmt.Sprintf("💎 Минимум: **%.2f TON**.\nКошелек: `%s`", minDep/rateTON, wallet), telebot.ModeMarkdown)
	})
	b.Handle(&btnPayRUB, func(c telebot.Context) error {
		return c.Send("🇷🇺 Для оплаты картой напишите @xDrezzx23")
	})
	b.Handle(&btnPayUAH, func(c telebot.Context) error {
		return c.Send(fmt.Sprintf("🇺🇦 Минимум: **%.0f грн**.\nНапишите @xDrezzx23", minDep/rateUAH))
	})

	log.Println("--- БОТ ЗАПУЩЕН И ГОТОВ К РАБОТЕ ---")
	b.Start()
}
