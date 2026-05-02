package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

func main() {
	// --- НАСТРОЙКИ ---
	var adminID int64 = 8699513395
	// Твой адрес из Tonkeeper/Wallet
	walletAddress := "UQBkHHHtTrkYraYeABJAepEr00eMYXKaVUUter4zfNk6eUxm"

	token := "8749363555:AAG8U7XWaLVXzkgXrymr5UcLK8xpeb4Zr1o"

	pref := telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	userStates := make(map[int64]string)

	// --- КЛАВИАТУРЫ (УНИКАЛЬНЫЙ ТЕКСТ = НЕТ БАГОВ) ---

	// Главное меню
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}
	btnOrder := menu.Text("🚀 Заказать проект")
	btnPrices := menu.Text("💰 Прайсы / Оплата")
	btnReviews := menu.Text("💬 Отзывы")
	btnSupport := menu.Text("🛠 Тех. поддержка")
	menu.Reply(menu.Row(btnOrder), menu.Row(btnPrices, btnReviews), menu.Row(btnSupport))

	// Меню выбора проекта
	orderMenu := &telebot.ReplyMarkup{ResizeKeyboard: true}
	btnTGBot := orderMenu.Text("🤖 ТГ Бот")
	btnOther := orderMenu.Text("✨ Другое")
	btnExitToHome := orderMenu.Text("🏠 Вернуться в меню")
	orderMenu.Reply(orderMenu.Row(btnTGBot, btnOther), orderMenu.Row(btnExitToHome))

	// Меню способов оплаты
	payMenu := &telebot.ReplyMarkup{ResizeKeyboard: true}
	btnStars := payMenu.Text("⭐ Telegram Stars")
	btnTON := payMenu.Text("💎 TON (Крипта)")
	btnCards := payMenu.Text("💳 Карты (UAH/RUB)")
	btnBackToStart := payMenu.Text("🏠 На главную")
	payMenu.Reply(payMenu.Row(btnStars, btnTON), payMenu.Row(btnCards), payMenu.Row(btnBackToStart))

	// Меню выбора карт
	cardMenu := &telebot.ReplyMarkup{ResizeKeyboard: true}
	btnUAH := cardMenu.Text("🇺🇦 Карта Украины")
	btnRUB := cardMenu.Text("🇷🇺 Оплата из РФ")
	btnReturnToPay := cardMenu.Text("🔙 К способам оплаты")
	cardMenu.Reply(cardMenu.Row(btnUAH, btnRUB), cardMenu.Row(btnReturnToPay))

	// Инлайн-кнопка для отзывов
	reviewsMenu := &telebot.ReplyMarkup{}
	btnGoToChannel := reviewsMenu.URL("🔗 Перейти в канал", "https://t.me/OtzXdrezzx")
	reviewsMenu.Inline(reviewsMenu.Row(btnGoToChannel))

	// --- ОБРАБОТЧИКИ ---

	// Старт
	b.Handle("/start", func(c telebot.Context) error {
		delete(userStates, c.Sender().ID)
		return c.Send("👋 **Привет! Это XDrezzx Order Bot.**\n\nРазработка ботов, софта и 3D моделей.\n\nВыбирай нужный раздел ниже:", menu, telebot.ModeMarkdown)
	})

	// Навигация (каждая кнопка ведет в свое место)
	b.Handle(&btnExitToHome, func(c telebot.Context) error {
		delete(userStates, c.Sender().ID)
		return c.Send("Главное меню:", menu)
	})

	b.Handle(&btnBackToStart, func(c telebot.Context) error {
		return c.Send("Главное меню:", menu)
	})

	b.Handle(&btnReturnToPay, func(c telebot.Context) error {
		return c.Send("Выберите способ оплаты:", payMenu)
	})

	// Раздел оплаты
	b.Handle(&btnPrices, func(c telebot.Context) error {
		return c.Send("📊 **Прайс-лист и Оплата**\n\nВыбери валюту для подробностей:", payMenu, telebot.ModeMarkdown)
	})

	b.Handle(&btnStars, func(c telebot.Context) error {
		text := "🌟 **Оплата Telegram Stars:**\n\n🔹 Бот: от 50 ⭐\n🔹 Проект: от 150 ⭐\n\n_Оплата принимается подарками или через счета._"
		return c.Send(text, telebot.ModeMarkdown)
	})

	b.Handle(&btnTON, func(c telebot.Context) error {
		text := fmt.Sprintf("💎 **Оплата в TON:**\n\n🔹 Бот: от 1.5 TON\n🔹 Проект: от 6 TON\n\n**Ваш адрес для оплаты:**\n`%s`\n\n_(Нажми на адрес, чтобы скопировать)_", walletAddress)
		return c.Send(text, telebot.ModeMarkdown)
	})

	b.Handle(&btnCards, func(c telebot.Context) error {
		return c.Send("Выберите регион вашей карты:", cardMenu)
	})

	b.Handle(&btnUAH, func(c telebot.Context) error {
		text := "🇺🇦 **Оплата (UAH):**\n\n🔹 Бот: от 50 грн\n🔹 Проект: от 150 грн\n\nПринимаем на Monobank / Privat24.\nДля получения реквизитов пиши: @xDrezzx23"
		return c.Send(text, telebot.ModeMarkdown)
	})

	b.Handle(&btnRUB, func(c telebot.Context) error {
		text := "🇷🇺 **Оплата из РФ:**\n\n🔹 Бот: от 150 ₽\n🔹 Проект: от 400 ₽\n\n**Как оплатить:**\nПрямые переводы не работают. Используйте **TON** через @send или @wallet (покупка картой РФ/СБП).\n\nНапиши мне @xDrezzx23, я помогу всё сделать быстро!"
		return c.Send(text, telebot.ModeMarkdown)
	})

	// Остальное
	b.Handle(&btnReviews, func(c telebot.Context) error {
		return c.Send("Наши отзывы здесь:", reviewsMenu)
	})

	b.Handle(&btnSupport, func(c telebot.Context) error {
		return c.Send("🛠 **Тех. поддержка:** @xDrezzx23")
	})

	b.Handle(&btnOrder, func(c telebot.Context) error {
		return c.Send("Что будем создавать?", orderMenu)
	})

	b.Handle(&btnTGBot, func(c telebot.Context) error {
		userStates[c.Sender().ID] = "waiting_order"
		return c.Send("🤖 Опиши свой запрос для ТГ Бота:")
	})

	b.Handle(&btnOther, func(c telebot.Context) error {
		userStates[c.Sender().ID] = "waiting_order"
		return c.Send("✨ Опиши свой проект максимально подробно:")
	})

	// Обработка сообщений (Заказы и ответы админа)
	b.Handle(telebot.OnText, func(c telebot.Context) error {
		// Ответ админа пользователю
		if c.Sender().ID == adminID && c.Message().ReplyTo != nil {
			lines := strings.Split(c.Message().ReplyTo.Text, "\n")
			var userID int64
			lastLine := lines[len(lines)-1]
			fmt.Sscanf(lastLine, "ID: %d", &userID)
			if userID != 0 {
				b.Send(telebot.ChatID(userID), "✉️ **Ответ от разработчика:**\n\n"+c.Text(), telebot.ModeMarkdown)
				return c.Send("✅ Отправлено!")
			}
		}

		// Прием заказа
		if state, ok := userStates[c.Sender().ID]; ok && state == "waiting_order" {
			msg := fmt.Sprintf("📥 **НОВЫЙ ЗАКАЗ!**\nОт: @%s\nЗапрос: %s\n\nID: %d",
				c.Sender().Username, c.Text(), c.Sender().ID)
			b.Send(telebot.ChatID(adminID), msg)
			delete(userStates, c.Sender().ID)
			return c.Send("✅ Запрос отправлен! Ожидай ответа прямо здесь.")
		}
		return nil
	})

	log.Println("xDrezzx Bot запущен с кошельком TON!")
	b.Start()
}
