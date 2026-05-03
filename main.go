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

// --- СТРУКТУРЫ ДАННЫХ ---
type UserData struct {
	ReferrerID  int64 `json:"referrer_id"`
	PromoActive bool  `json:"promo_active"`
	RefCount    int   `json:"ref_count"`
	IsBanned    bool  `json:"is_banned"`
	Role        int   `json:"role"` // 0: User, 1: Mod, 2: Admin, 3: Owner
}

var (
	Storage = make(map[int64]*UserData)
	mutex   sync.Mutex
)

const (
	dbFile    = "users.json"
	pBotStars = 50.0
	pPrjStars = 150.0
	pBotTON   = 1.5
	pPrjTON   = 6.0
	pBotUAH   = 50.0
	pPrjUAH   = 150.0
	pBotRUB   = 120.0
	pPrjRUB   = 350.0
)

// --- РАБОТА С БАЗОЙ ---
func loadData() {
	file, err := os.ReadFile(dbFile)
	if err != nil {
		return
	}
	json.Unmarshal(file, &Storage)
}

func saveData() {
	mutex.Lock()
	defer mutex.Unlock()
	data, _ := json.MarshalIndent(Storage, "", "  ")
	os.WriteFile(dbFile, data, 0644)
}

func getU(id int64) *UserData {
	if _, ok := Storage[id]; !ok {
		Storage[id] = &UserData{Role: 0}
		saveData()
	}
	return Storage[id]
}

func main() {
	loadData()

	// --- НАСТРОЙКИ ---
	token := "8749363555:AAG8U7XWaLvXzkgXrymr5UcLK8xpeb4Zr1o"
	ownerID := int64(8699513395)
	wallet := "UQBkHHHtTrkYraYeABJAepEr00eMYXKaVUUter4zfNk6eUxm"
	reviewChan := int64(-1003953639397)

	getU(ownerID).Role = 3 // Назначаем тебя владельцем
	saveData()

	b, err := telebot.NewBot(telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal(err)
		return
	}

	// --- КНОПКИ И КЛАВИАТУРЫ ---
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}
	btnOrder := menu.Text("🚀 Заказать проект")
	btnPrices := menu.Text("💰 Прайсы / Оплата")
	btnPromo := menu.Text("🎁 Скидки и Рефералы")
	btnReviews := menu.Text("💬 Отзывы")
	menu.Reply(menu.Row(btnOrder), menu.Row(btnPrices, btnPromo), menu.Row(btnReviews))

	payMenu := &telebot.ReplyMarkup{ResizeKeyboard: true}
	btnStars := payMenu.Text("⭐ Telegram Stars")
	btnTON := payMenu.Text("💎 TON (Крипта)")
	btnCards := payMenu.Text("💳 Карты (UAH/RUB)")
	btnHome := payMenu.Text("🏠 На главную")
	payMenu.Reply(payMenu.Row(btnStars, btnTON), payMenu.Row(btnCards), payMenu.Row(btnHome))

	cardMenu := &telebot.ReplyMarkup{ResizeKeyboard: true}
	btnUAH := cardMenu.Text("🇺🇦 Карта Украины")
	btnRUB := cardMenu.Text("🇷🇺 Оплата из РФ")
	cardMenu.Reply(cardMenu.Row(btnUAH, btnRUB), cardMenu.Row(btnHome))

	inlineRev := &telebot.ReplyMarkup{}
	btnWriteRev := inlineRev.Data("✍️ Оставить отзыв", "write_rev")
	inlineRev.Inline(inlineRev.Row(btnWriteRev))

	userStates := make(map[int64]string)

	// --- ЛОГИКА ЦЕН И СКИДОК ---
	getDisc := func(id int64) int {
		u := getU(id)
		d := 0
		if u.PromoActive {
			d += 5
		}
		d += (u.RefCount / 3) * 5
		return d
	}
	calc := func(p float64, d int) float64 { return p * (1 - float64(d)/100) }

	// Функция для отправки прайса (чтобы не дублировать код)
	sendPayInfo := func(c telebot.Context, mode string) error {
		id := c.Sender().ID
		u := getU(id)
		d := getDisc(id)
		var txt string
		switch mode {
		case "Stars":
			txt = fmt.Sprintf("⭐ **Telegram Stars (-%d%%):**\n— Создание бота: %.0f ⭐\n— Крупный проект: %.0f ⭐", d, calc(pBotStars, d), calc(pPrjStars, d))
		case "TON":
			txt = fmt.Sprintf("💎 **TON Crypto (-%d%%):**\n— Создание бота: %.2f TON\n— Крупный проект: %.2f TON\n\n**Реквизиты:**\n`%s`", d, calc(pBotTON, d), calc(pPrjTON, d), wallet)
		case "UAH":
			txt = fmt.Sprintf("🇺🇦 **Карта Украины (-%d%%):**\n— Создание бота: %.0f грн\n— Крупный проект: %.0f грн", d, calc(pBotUAH, d), calc(pPrjUAH, d))
		case "RUB":
			txt = fmt.Sprintf("🇷🇺 **Карта РФ (-%d%%):**\n— Создание бота: %.0f руб\n— Крупный проект: %.0f руб", d, calc(pBotRUB, d), calc(pPrjRUB, d))
		}
		u.PromoActive = false // Скидка используется один раз
		saveData()
		return c.Send(txt+"\n\n⚠️ **Важно:** Реквизиты для карт уточняйте у админа. Оплата производится строго ПОСЛЕ выполнения работы и её проверки вами.", telebot.ModeMarkdown)
	}

	// --- ОСНОВНЫЕ ОБРАБОТЧИКИ ---

	b.Handle("/start", func(c telebot.Context) error {
		delete(userStates, c.Sender().ID)
		// Реферальная система
		if c.Message().Payload != "" {
			rID, _ := strconv.ParseInt(c.Message().Payload, 10, 64)
			u := getU(c.Sender().ID)
			if rID != c.Sender().ID && u.ReferrerID == 0 {
				u.ReferrerID = rID
				getU(rID).RefCount++
				saveData()
				b.Send(telebot.ChatID(rID), "🔔 **У вас новый реферал!** Скидка увеличивается.")
			}
		}
		return c.Send("👋 **Добро пожаловать в студию XDrezzx!**\n\nМы создаем ботов, сайты и игровые проекты любой сложности. Используй меню ниже, чтобы ознакомиться с услугами.", menu)
	})

	b.Handle(&btnOrder, func(c telebot.Context) error {
		userStates[c.Sender().ID] = "waiting_order"
		return c.Send("📝 **Режим оформления заказа.**\n\nПожалуйста, опишите ваше техническое задание (ТЗ) максимально подробно:\n1. Что должен делать проект?\n2. Желаемые сроки?\n3. Ваш бюджет?\n\n*Просто отправьте текст следующим сообщением.*")
	})

	b.Handle(&btnPrices, func(c telebot.Context) error {
		delete(userStates, c.Sender().ID) // Отменяем ввод заказа, если перешли в прайсы
		return c.Send("💰 **Наши актуальные цены.**\nВыбери способ оплаты, чтобы увидеть стоимость с учетом твоих личных скидок:", payMenu)
	})

	b.Handle(&btnPromo, func(c telebot.Context) error {
		delete(userStates, c.Sender().ID)
		u := getU(c.Sender().ID)
		link := fmt.Sprintf("https://t.me/%s?start=%d", b.Me.Username, c.Sender().ID)
		disc := getDisc(c.Sender().ID)

		msg := fmt.Sprintf("🎁 **Скидки и реферальная программа**\n\n— Твоя текущая скидка: **%d%%**\n— Приглашено друзей: **%d**\n\n🔗 **Твоя ссылка для приглашений:**\n`%s`\n\n*Приглашай друзей и получай +5%% скидки за каждых 3-х человек!*", disc, u.RefCount, link)

		promoMarkup := &telebot.ReplyMarkup{ResizeKeyboard: true}
		promoMarkup.Reply(promoMarkup.Row(promoMarkup.Text("🎟 Ввести промокод")), promoMarkup.Row(btnHome))
		return c.Send(msg, promoMarkup, telebot.ModeMarkdown)
	})

	b.Handle(&btnReviews, func(c telebot.Context) error {
		delete(userStates, c.Sender().ID)
		return c.Send("💬 **Отзывы о нашей работе.**\nВсе отзывы от реальных клиентов публикуются здесь: @OtzXdrezzx\n\nНам важно ваше мнение!", inlineRev)
	})

	b.Handle(&btnHome, func(c telebot.Context) error {
		delete(userStates, c.Sender().ID)
		return c.Send("🏠 **Вы вернулись в главное меню.**", menu)
	})

	// Обработка кнопок оплаты
	b.Handle(&btnStars, func(c telebot.Context) error { return sendPayInfo(c, "Stars") })
	b.Handle(&btnTON, func(c telebot.Context) error { return sendPayInfo(c, "TON") })
	b.Handle(&btnCards, func(c telebot.Context) error {
		return c.Send("💳 **Выбери валюту для оплаты картой:**", cardMenu)
	})
	b.Handle(&btnUAH, func(c telebot.Context) error { return sendPayInfo(c, "UAH") })
	b.Handle(&btnRUB, func(c telebot.Context) error { return sendPayInfo(c, "RUB") })

	// --- АДМИНИСТРАТИВНЫЕ КОМАНДЫ ---

	b.Handle("/done", func(c telebot.Context) error {
		if getU(c.Sender().ID).Role < 1 {
			return nil
		}
		tID, _ := strconv.ParseInt(c.Message().Payload, 10, 64)
		if tID == 0 {
			return c.Send("⚠️ Формат: `/done [ID]`")
		}
		b.Send(telebot.ChatID(tID), "🎉 **Твой проект полностью готов!**\nАдминистратор подтвердил выполнение. Пожалуйста, выбери способ оплаты для получения финальных файлов:", payMenu)
		return c.Send("✅ Уведомление о готовности отправлено клиенту.")
	})

	b.Handle("/ban", func(c telebot.Context) error {
		if getU(c.Sender().ID).Role < 2 {
			return nil
		}
		tID, _ := strconv.ParseInt(c.Message().Payload, 10, 64)
		getU(tID).IsBanned = true
		saveData()
		return c.Send("🚫 Пользователь заблокирован.")
	})

	b.Handle("/setrole", func(c telebot.Context) error {
		if getU(c.Sender().ID).Role < 3 {
			return nil
		}
		args := strings.Split(c.Message().Payload, " ")
		if len(args) < 2 {
			return c.Send("⚠️ Формат: `/setrole [ID] [Ранг 1-3]`")
		}
		tID, _ := strconv.ParseInt(args[0], 10, 64)
		rank, _ := strconv.Atoi(args[1])
		getU(tID).Role = rank
		saveData()
		return c.Send(fmt.Sprintf("👑 Пользователю %d назначен ранг %d", tID, rank))
	})

	// --- УМНЫЙ ОБРАБОТЧИК ТЕКСТА ---

	b.Handle(telebot.OnText, func(c telebot.Context) error {
		id := c.Sender().ID
		u := getU(id)
		if u.IsBanned {
			return nil
		}

		// Проверка: это кнопка «Ввести промокод»?
		if c.Text() == "🎟 Ввести промокод" {
			userStates[id] = "waiting_promo"
			return c.Send("⌨️ **Введите ваш секретный промокод:**")
		}

		// Если это ответ админа на пересланное сообщение (Reply)
		if u.Role >= 1 && c.Message().ReplyTo != nil {
			var targetID int64
			lines := strings.Split(c.Message().ReplyTo.Text, "\n")
			lastLine := lines[len(lines)-1]
			fmt.Sscanf(lastLine, "ID: %d", &targetID)
			if targetID != 0 {
				b.Send(telebot.ChatID(targetID), "✉️ **Сообщение от администрации:**\n\n"+c.Text())
				return c.Send("✅ Ответ успешно доставлен.")
			}
		}

		// Логика состояний юзера
		state := userStates[id]
		if state == "" {
			return nil
		} // Если ничего не нажал — игнорим

		switch state {
		case "waiting_order":
			adminMsg := fmt.Sprintf("📥 **НОВЫЙ ЗАКАЗ**\n\n👤 **Клиент:** @%s\n📝 **ТЗ:** %s\n\nID: %d", c.Sender().Username, c.Text(), id)
			b.Send(telebot.ChatID(ownerID), adminMsg)
			// Дублируем всем админам ранга 2+
			for aid, adata := range Storage {
				if aid != ownerID && adata.Role >= 2 {
					b.Send(telebot.ChatID(aid), adminMsg)
				}
			}
			delete(userStates, id)
			return c.Send("✅ **Ваш заказ принят в работу!**\n\nАдминистраторы рассмотрят его и ответят вам в ближайшее время прямо в этом боте.", menu)

		case "waiting_promo":
			if strings.ToUpper(c.Text()) == "STARTX" {
				u.PromoActive = true
				saveData()
				delete(userStates, id)
				return c.Send("✅ **Промокод активирован!** Твоя скидка 5% применится при следующем просмотре прайса.", menu)
			}
			return c.Send("❌ **Неверный код.** Попробуй еще раз или нажми «На главную».")

		case "waiting_review":
			revMsg := fmt.Sprintf("💬 **НОВЫЙ ОТЗЫВ**\n\n👤 **От:** @%s\n📝 **Текст:** %s", c.Sender().Username, c.Text())
			b.Send(telebot.ChatID(reviewChan), revMsg)
			delete(userStates, id)
			return c.Send("🙏 **Спасибо за ваш отзыв!** Мы ценим вашу поддержку.", menu)
		}

		return nil
	})

	// Обработка инлайнового отзыва
	b.Handle(&btnWriteRev, func(c telebot.Context) error {
		userStates[c.Sender().ID] = "waiting_review"
		return c.Send("✍️ **Пожалуйста, напишите ваш отзыв одним сообщением.**\nОн будет опубликован в нашем канале отзывов.")
	})

	log.Println("--- БОТ УСПЕШНО ЗАПУЩЕН ---")
	b.Start()
}
