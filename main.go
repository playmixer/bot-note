package main

/*
Бот для заметок

добавление заметки
выбор заметки -> вывод структурированного описания заметки
изменение заметки (по slug?)
удаление заметки (по slug?)

добавить тег для заметки
поиск по тегу
редактировать теги у заметки

Добавить событие по расписанию с сообщением в ТГ с зметкой



Commands:
start - начать работать с ботом
list - показать все заметки
new - добавить заметку
tags - ваши теги
*/

import (
	"crypto/tls"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/playmixer/bot-note/models"
	"github.com/playmixer/corvid/logger"
	tg "github.com/playmixer/telegram-bot-api"
)

var (
	bot   tg.TelegramBot
	log   *logger.Logger
	store UserStore
)

func init() {
	store = UserStore{
		data: make(map[int64]User),
		mu:   sync.Mutex{},
	}
}

func main() {
	var err error
	log = logger.New("app")
	log.SetLevel(logger.DEBUG)

	log.INFO("Starting...")
	err = godotenv.Load()
	if err != nil {
		log.ERROR("Error loading .env file")
	}

	if os.Getenv("TLS") == "0" {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	models.DB, err = models.Connect(os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"))
	if err != nil {
		panic(err)
	}

	bot, err = tg.NewBot(os.Getenv("TELEGRAM_BOT_API_KEY"))
	if err != nil {
		log.ERROR(err.Error())
		return
	}

	bot.AddHandle(tg.Command("start", start))
	bot.AddHandle(tg.Command("new", new))
	bot.AddHandle(tg.Command("list", list))
	bot.AddHandle(tg.Command("tags", tags))
	bot.AddHandle(func(update tg.UpdateResult, bot *tg.TelegramBot) {
		if strings.Contains(update.CallbackQuery.Data, CB_ROUTE_SEARCH_TAG) {
			cbSearchByTag(update, bot)
		}
	})
	bot.AddHandle(func(update tg.UpdateResult, bot *tg.TelegramBot) {
		if strings.Contains(update.CallbackQuery.Data, CB_ROUTE_SEARCH_TAG_PREV) ||
			strings.Contains(update.CallbackQuery.Data, CB_ROUTE_SEARCH_TAG_NEXT) {
			cbChangePageByTag(update, bot)
		}
	})
	bot.AddHandle(func(update tg.UpdateResult, bot *tg.TelegramBot) {
		if strings.Contains(update.CallbackQuery.Data, CB_ROUTE_SHOW) ||
			strings.Contains(update.CallbackQuery.Data, CB_ROUTE_TAG_SHOW) {
			cbShow(update, bot)
		}
	})
	bot.AddHandle(func(update tg.UpdateResult, bot *tg.TelegramBot) {
		if strings.Contains(update.CallbackQuery.Data, CB_ROUTE_LIST_ALL) {
			cbList(update, bot)
		}
	})
	bot.AddHandle(func(update tg.UpdateResult, bot *tg.TelegramBot) {
		if strings.Contains(update.CallbackQuery.Data, CB_ROUTE_NEW) {
			cbNew(update, bot)
		}
	})
	bot.AddHandle(func(update tg.UpdateResult, bot *tg.TelegramBot) {
		if strings.Contains(update.CallbackQuery.Data, CB_ROUTE_EDIT) {
			cbEdit(update, bot)
		}
	})
	bot.AddHandle(func(update tg.UpdateResult, bot *tg.TelegramBot) {
		if strings.Contains(update.CallbackQuery.Data, CB_ROUTE_EDITING) {
			cbEditing(update, bot)
		}
	})
	bot.AddHandle(func(update tg.UpdateResult, bot *tg.TelegramBot) {
		if strings.Contains(update.CallbackQuery.Data, CB_ROUTE_DEL) {
			cbDelete(update, bot)
		}
	})
	bot.AddHandle(func(update tg.UpdateResult, bot *tg.TelegramBot) {
		if strings.Contains(update.CallbackQuery.Data, CB_ROUTE_LIST_PREV) ||
			strings.Contains(update.CallbackQuery.Data, CB_ROUTE_LIST_NEXT) {
			cbChangePage(update, bot)
		}
	})
	bot.AddHandle(tg.Text(echo))

	bot.Timeout = time.Second
	log.INFO("Start")
	bot.Polling()
	log.INFO("Exit")
}
