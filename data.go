package main

import (
	"fmt"
	"strings"

	"github.com/playmixer/bot-note/models"
	tg "github.com/playmixer/telegram-bot-api/v2"
)

func validateString(message string) string {
	charsets := []rune{'-', '.', '='}
	for _, c := range charsets {
		message = strings.ReplaceAll(message, string(c), fmt.Sprintf("\\%c", c))
	}

	return message
}

func IsEnableTelegramUser(update tg.UpdateResult, bot *tg.TelegramBot) bool {
	var userId int64 = update.Message.From.Id
	if update.CallbackQuery.From.Id != 0 {
		userId = update.CallbackQuery.From.Id
	}
	user := store.Get(userId)

	if user.Id == 0 {
		err := CacheUserStore(userId)
		if err != nil {
			log.ERROR("cached user store error:", err.Error())
			return false
		}
		return true
	}
	return true
}

func CacheUserStore(userId int64) error {
	var user User

	//ищем пользователя, если его нет то создаём
	userModel, err := models.GetUserByTelegramId(userId)
	if err != nil {
		log.ERROR(err.Error())
		log.DEBUG("user", fmt.Sprint(userModel))
		// bot.EditMessage(update.Message.From.Id, msg.Result.MessageId, "Ошибка на сервер")
		return err
	}
	if userModel.Id == 0 {
		user.Id, err = models.NewUser(userId, "")
		if err != nil {
			log.ERROR(err.Error())
			return err
		}
	} else {
		user.Id = userModel.Id
	}

	log.DEBUG(fmt.Sprintf("cache user store %v, %s", userId, fmt.Sprint(user)))

	store.Set(userId, user)

	return nil
}

func KeyboardList(user *User) (tg.InlineKeyboardMarkup, error) {
	keyboard := tg.InlineMarkup()
	notes, err := models.GetNotes(user.Id)
	if err != nil {
		return keyboard, err
	}

	_start := max(int(user.NotePage*LIST_PAGE_SIZE), 0)
	_end := min(int(user.NotePage*LIST_PAGE_SIZE+LIST_PAGE_SIZE), len(notes))
	_start = min(_start, _end)
	_end = max(_start, _end)

	for _, note := range notes[_start:_end] {
		btnShow := keyboard.Button(note.Title).SetCallbackData(fmt.Sprintf("%s %v", CB_ROUTE_SHOW, note.Id))
		keyboard.Add([]tg.InlineKeyboardButton{*btnShow})

		btns := []tg.InlineKeyboardButton{}

		btnEdit := keyboard.Button("📝")
		btnEdit.SetCallbackData(fmt.Sprintf("%s %v", CB_ROUTE_EDIT, note.Id))
		btns = append(btns, *btnEdit)

		if note.Url != "" {
			btnOpen := keyboard.Button("📖")
			btnOpen.SetUrl(note.Url)
			btns = append(btns, *btnOpen)
		}

		btnDel := keyboard.Button("❌")
		btnDel.SetCallbackData(fmt.Sprintf("%s %v", CB_ROUTE_DEL, note.Id))
		btns = append(btns, *btnDel)

		keyboard.Add(btns)

	}
	btnsControl := []tg.InlineKeyboardButton{}
	btnPrev := keyboard.Button("<<").SetCallbackData(CB_ROUTE_LIST_PREV)
	if user.NotePage > 0 {
		btnsControl = append(btnsControl, *btnPrev)
	}
	btnNext := keyboard.Button(">>").SetCallbackData(CB_ROUTE_LIST_NEXT)
	if int(user.NotePage*LIST_PAGE_SIZE)+LIST_PAGE_SIZE < len(notes) {
		btnsControl = append(btnsControl, *btnNext)
	}
	keyboard.Add(btnsControl)
	return keyboard, nil
}

func KeyboardNewNote(user *User) (tg.InlineKeyboardMarkup, error) {
	keyboard := tg.InlineMarkup()

	// btnNewTitle := keyboard.Button("Название").SetCallbackData(fmt.Sprintf("%s %s", CB_ROUTE_NEW, "title"))
	btnNewUrl := keyboard.Button("Ссылка").SetCallbackData(fmt.Sprintf("%s %s", CB_ROUTE_NEW, "url"))
	btnNewDescription := keyboard.Button("Описание").SetCallbackData(fmt.Sprintf("%s %s", CB_ROUTE_NEW, "description"))
	btnNewTags := keyboard.Button("Теги").SetCallbackData(fmt.Sprintf("%s %s", CB_ROUTE_NEW, "tags"))
	keyboard.Add([]tg.InlineKeyboardButton{*btnNewUrl, *btnNewDescription, *btnNewTags})

	btnNewSave := keyboard.Button("Сохранить").SetCallbackData(fmt.Sprintf("%s %s", CB_ROUTE_NEW, "save"))
	keyboard.Add([]tg.InlineKeyboardButton{*btnNewSave})

	return keyboard, nil
}

func KeyboardEditNote(user *User) (tg.InlineKeyboardMarkup, error) {
	keyboard := tg.InlineMarkup()

	btnNewTitle := keyboard.Button("Название").SetCallbackData(fmt.Sprintf("%s %s", CB_ROUTE_EDITING, "title"))
	btnNewUrl := keyboard.Button("Ссылка").SetCallbackData(fmt.Sprintf("%s %s", CB_ROUTE_EDITING, "url"))
	btnNewDescription := keyboard.Button("Описание").SetCallbackData(fmt.Sprintf("%s %s", CB_ROUTE_EDITING, "description"))
	btnNewTags := keyboard.Button("Теги").SetCallbackData(fmt.Sprintf("%s %s", CB_ROUTE_EDITING, "tags"))
	keyboard.Add([]tg.InlineKeyboardButton{*btnNewTitle, *btnNewUrl, *btnNewDescription, *btnNewTags})

	btnNewSave := keyboard.Button("Обновить").SetCallbackData(fmt.Sprintf("%s %s", CB_ROUTE_EDITING, "update"))
	keyboard.Add([]tg.InlineKeyboardButton{*btnNewSave})

	return keyboard, nil
}

func KeyboardTags(tags []string) (tg.InlineKeyboardMarkup, error) {
	keyboard := tg.InlineMarkup()

	keyLine := []tg.InlineKeyboardButton{}

	for i, tag := range tags {
		if i%3 == 0 {
			keyboard.Add(keyLine)
			keyLine = []tg.InlineKeyboardButton{}
		}
		btn := keyboard.Button(tag).SetCallbackData(fmt.Sprintf("%s %s", CB_ROUTE_SEARCH_TAG, tag))
		keyLine = append(keyLine, *btn)
	}
	if len(keyLine) > 0 {
		keyboard.Add(keyLine)
	}

	return keyboard, nil
}

func KeyboardListByTag(user *User, tag string) (tg.InlineKeyboardMarkup, error) {
	keyboard := tg.InlineMarkup()
	notes, err := models.GetNotesByTag(user.Id, tag)
	if err != nil {
		return keyboard, err
	}

	_start := max(int(user.NotePage*LIST_PAGE_SIZE), 0)
	_end := min(int(user.NotePage*LIST_PAGE_SIZE+LIST_PAGE_SIZE), len(notes))
	_start = min(_start, _end)
	_end = max(_start, _end)

	for _, note := range notes[_start:_end] {
		btnShow := keyboard.Button(note.Title).SetCallbackData(fmt.Sprintf("%s %v", CB_ROUTE_TAG_SHOW, note.Id))
		keyboard.Add([]tg.InlineKeyboardButton{*btnShow})

		btns := []tg.InlineKeyboardButton{}

		btnEdit := keyboard.Button("📝")
		btnEdit.SetCallbackData(fmt.Sprintf("%s %v", CB_ROUTE_EDIT, note.Id))
		btns = append(btns, *btnEdit)

		if note.Url != "" {
			btnOpen := keyboard.Button("📖")
			btnOpen.SetUrl(note.Url)
			btns = append(btns, *btnOpen)
		}

		btnDel := keyboard.Button("❌")
		btnDel.SetCallbackData(fmt.Sprintf("%s %v", CB_ROUTE_DEL, note.Id))
		btns = append(btns, *btnDel)

		keyboard.Add(btns)

	}
	btnsControl := []tg.InlineKeyboardButton{}
	btnPrev := keyboard.Button("<<").SetCallbackData("_list_prev")
	if user.NotePage > 0 {
		btnsControl = append(btnsControl, *btnPrev)
	}
	btnNext := keyboard.Button(">>").SetCallbackData("_list_next")
	if int(user.NotePage*LIST_PAGE_SIZE)+LIST_PAGE_SIZE < len(notes) {
		btnsControl = append(btnsControl, *btnNext)
	}
	keyboard.Add(btnsControl)
	return keyboard, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
