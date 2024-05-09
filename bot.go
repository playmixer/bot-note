package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/playmixer/bot-note/models"
	tg "github.com/playmixer/telegram-bot-api/v3"
)

const (
	LIST_PAGE_SIZE = 5
)

const (
	CB_ROUTE_LIST_ALL        = "_list_all"
	CB_ROUTE_LIST_PREV       = "_list_prev"
	CB_ROUTE_LIST_NEXT       = "_list_next"
	CB_ROUTE_SHOW            = "_show"
	CB_ROUTE_TAG_SHOW        = "_show_by_tag"
	CB_ROUTE_NEW             = "_new"
	CB_ROUTE_EDIT            = "_edit_"
	CB_ROUTE_EDITING         = "_editing"
	CB_ROUTE_DEL             = "_delete"
	CB_ROUTE_SEARCH_TAG      = "_search_by_tag_all"
	CB_ROUTE_SEARCH_TAG_PREV = "_search_by_tag_prev"
	CB_ROUTE_SEARCH_TAG_NEXT = "_search_by_tag_next"
)

func start(update tg.UpdateResult, bot *tg.TelegramBot) {
	keyboard := tg.InlineMarkup()

	btnList := *keyboard.Button("Список заметок")
	btnList.SetCallbackData(CB_ROUTE_LIST_ALL)

	btnAdd := *keyboard.Button("Новая заметка")
	btnAdd.SetCallbackData(CB_ROUTE_NEW)

	keyboard.Add([]tg.InlineKeyboardButton{btnList, btnAdd})

	msg := bot.SendMessage(update.Message.Chat.Id, "Бот для заметок, введите команду:\n/new - добавить заметку\n/list - увидеть свои заметки", keyboard.Option())
	if !msg.Ok {
		log.ERROR(msg.Description)
		return
	}
	user := store.Get(update.Message.From.Id)
	user.Status = USER_STATUS_NONE
	defer func() {
		store.Set(update.Message.From.Id, user)
	}()

	err := CacheUserStore(update.Message.From.Id)
	if err != nil {
		log.ERROR("caching user store error:", err.Error())
		return
	}

	log.DEBUG(fmt.Sprint(user))
}

func list(update tg.UpdateResult, bot *tg.TelegramBot) {
	if !IsEnableTelegramUser(update, bot) {
		log.INFO(fmt.Sprintf("user %d is note create", update.Message.From.Id))
		return
	}
	user := store.Get(update.Message.From.Id)
	defer func() {
		store.Set(update.Message.From.Id, user)
	}()

	options := []tg.MessageOption{
		tg.StyleMarkdown(tg.MessageStyleMarkdownV2),
	}

	text := validateString("Загружаю...")
	msg := bot.SendMessage(update.Message.From.Id, text, options...)
	if !msg.Ok {
		log.ERROR("error send message", text)
		log.ERROR(msg.Description)
		return
	}

	keyboard, err := KeyboardList(&user)
	if err != nil {
		log.ERROR(fmt.Sprintf("%v keyboard error: %e", user.Id, err))
	}

	text = "*Список заметок:*"
	msg = bot.EditMessage(
		update.Message.Chat.Id,
		msg.Result.MessageId,
		validateString(text),
		keyboard.Option(),
		tg.StyleMarkdown(tg.MessageStyleMarkdownV2),
	)
	if !msg.Ok {
		log.ERROR("error send message", text)
		log.ERROR(msg.Description)
		return
	}
	user.Status = USER_STATUS_NONE
	log.DEBUG(fmt.Sprint(user))
}

func new(update tg.UpdateResult, bot *tg.TelegramBot) {
	if !IsEnableTelegramUser(update, bot) {
		log.INFO(fmt.Sprintf("user %d is note create", update.Message.From.Id))
		return
	}
	log.INFO(fmt.Sprintf("user %v use command add", update.Message.From.Id))
	msg := bot.SendMessage(update.Message.Chat.Id, "Введите название заметки:")
	if !msg.Ok {
		log.ERROR(msg.Description)
		return
	}

	user := store.Get(update.Message.From.Id)
	defer func() {
		store.Set(update.Message.From.Id, user)
	}()
	user.Status = USER_STATUS_NEW

}

func echo(update tg.UpdateResult, bot *tg.TelegramBot) {
	var err error
	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)

	if !IsEnableTelegramUser(update, bot) {
		log.INFO(fmt.Sprintf("user %d is note create", update.Message.From.Id))
		return
	}

	if update.CallbackQuery.Id != "" {
		log.INFO(fmt.Sprintf("%v callback text %s", update.CallbackQuery.From.Id, update.CallbackQuery.Data))
		return
	}

	user := store.Get(update.Message.From.Id)
	defer func() {
		store.Set(update.Message.From.Id, user)
	}()

	if user.Status == USER_STATUS_NONE {
		return
	}

	keyboard, err := KeyboardNewNote(&user)
	if err != nil {
		log.ERROR(fmt.Sprintf("%v keyboard error: %e", user.Id, err))
		return
	}
	keyboardEdit, err := KeyboardEditNote(&user)
	if err != nil {
		log.ERROR(fmt.Sprintf("%v keyboard error: %e", user.Id, err))
		return
	}

	switch user.Status {
	case USER_STATUS_NEW:
		user.Add(update.Message.Text)
		user.Status = USER_STATUS_NEW_URL
		log.DEBUG(fmt.Sprintf("%v set name %s", update.Message.From.Id, update.Message.Text))

		bot.SendMessage(update.Message.Chat.Id, "Введите ссылку:", keyboard.Option())

	case USER_STATUS_NEW_URL:
		_, err := url.ParseRequestURI(update.Message.Text)
		if err != nil {
			bot.SendMessage(update.Message.Chat.Id, "Не корректная ссылка, введите ссылку:", keyboard.Option())
			return
		}
		user.AddUrl(update.Message.Text)
		user.Status = USER_STATUS_NEW_DESCRIPTION
		log.INFO(fmt.Sprintf("%v set url %s", update.Message.From.Id, update.Message.Text))

		bot.SendMessage(update.Message.Chat.Id, "Добавьте описание:", keyboard.Option())

	case USER_STATUS_NEW_DESCRIPTION:
		user.AddDescription(update.Message.Text)
		user.Status = USER_STATUS_NEW_TAGS
		log.INFO(fmt.Sprintf("%v set description %s", update.Message.From.Id, update.Message.Text))

		bot.SendMessage(update.Message.Chat.Id, "Добавьте теги (через пробел):", keyboard.Option())

	case USER_STATUS_NEW_TAGS:
		tags := strings.Split(update.Message.Text, " ")
		user.AddTags(tags)
		user.Status = USER_STATUS_NONE
		log.INFO(fmt.Sprintf("%v set tags %s", update.Message.From.Id, update.Message.Text))

		err = models.NewNote(ctx, user.Id, user.Note.Name, user.Note.URL, user.Note.Description, user.Note.Tags)
		if err != nil {
			log.ERROR(err.Error())
			bot.SendMessage(update.Message.Chat.Id, "Ошибка на сервере")
			return
		}
		err = user.SaveNote()
		if err != nil {
			log.ERROR(err.Error())
			bot.SendMessage(update.Message.Chat.Id, "Ошибка добавления заметки")
			return
		}
		log.INFO(fmt.Sprintf("%v saved note", update.Message.Chat.Id))

		bot.SendMessage(update.Message.Chat.Id, "Заметка сохранена")
		return

	case USER_STATUS_EDIT:
		user.Add(update.Message.Text)
		user.Status = USER_STATUS_EDIT_URL
		log.DEBUG(fmt.Sprintf("%v set name %s", update.Message.From.Id, update.Message.Text))

		bot.SendMessage(update.Message.Chat.Id, "Введите ссылку:", keyboardEdit.Option())
		return
	case USER_STATUS_EDIT_URL:
		_, err := url.ParseRequestURI(update.Message.Text)
		if err != nil {
			bot.SendMessage(update.Message.Chat.Id, "Не корректная ссылка, введите ссылку:", keyboardEdit.Option())
			return
		}
		user.AddUrl(update.Message.Text)
		user.Status = USER_STATUS_EDIT_DESCRIPTION
		log.DEBUG(fmt.Sprintf("%v set url %s", update.Message.From.Id, update.Message.Text))

		bot.SendMessage(update.Message.Chat.Id, "Введите описание:", keyboardEdit.Option())
		return

	case USER_STATUS_EDIT_DESCRIPTION:
		user.AddDescription(update.Message.Text)
		user.Status = USER_STATUS_EDIT_TAGS
		log.DEBUG(fmt.Sprintf("%v set description %s", update.Message.From.Id, update.Message.Text))

		bot.SendMessage(update.Message.Chat.Id, "Введите теги (через пробел):", keyboardEdit.Option())
		return
	case USER_STATUS_EDIT_TAGS:
		tags := strings.Split(update.Message.Text, " ")
		user.AddTags(tags)
		user.Status = USER_STATUS_NONE
		log.DEBUG(fmt.Sprintf("%v set tags %s", update.Message.From.Id, update.Message.Text))

		err := models.UpdNote(ctx, models.Note{
			Id:          user.Note.Id,
			Title:       user.Note.Name,
			UserId:      user.Id,
			Description: user.Note.Description,
			Url:         user.Note.URL,
		}, tags)
		if err != nil {
			log.ERROR(fmt.Sprintf("%v database erros: %e", update.Message.From.Id, err))
			return
		}
		bot.SendMessage(update.Message.Chat.Id, "Заметка обновлена")
		return
	}

}

func cbShow(update tg.UpdateResult, bot *tg.TelegramBot) {
	if !IsEnableTelegramUser(update, bot) {
		log.INFO(fmt.Sprintf("user %d is note create", update.Message.From.Id))
		return
	}
	user := store.Get(update.CallbackQuery.From.Id)
	defer func() {
		store.Set(update.CallbackQuery.From.Id, user)
	}()

	if user.Id == 0 {
		log.ERROR("user not found")
		return
	}
	var cb string
	var noteId int64
	_, err := fmt.Sscan(update.CallbackQuery.Data, &cb, &noteId)
	if err != nil {
		log.ERROR(fmt.Sprintf("%v error: %e", user.Id, err))
	}

	note, err := models.GetNote(noteId)
	if err != nil {
		log.ERROR(fmt.Sprintf("%v database error: %e", user.Id, err))
	}

	tags, err := models.GetTagsByNoteId(note.Id)
	if err != nil {
		log.ERROR(fmt.Sprintf("%v database error: %e", user.Id, err))
	}

	tagsString := make([]string, len(tags))
	for i, _t := range tags {
		tagsString[i] = _t.Title
	}

	text := fmt.Sprintf("*Название:* %s \n*Ссылка:* %s \n*Описание:* %s \n*Теги:* %s",
		note.Title, note.Url, note.Description, strings.Join(tagsString, " "))

	var keyboard tg.InlineKeyboardMarkup
	switch cb {
	case CB_ROUTE_TAG_SHOW:
		keyboard, err = KeyboardListByTag(&user, user.SearchTag)
		if err != nil {
			log.ERROR(fmt.Sprintf("%v keyboard error %e", user.Id, err))
			return
		}
	default:
		keyboard, err = KeyboardList(&user)
		if err != nil {
			log.ERROR(fmt.Sprintf("%v keyboard error %e", user.Id, err))
			return
		}
	}

	msg := bot.EditMessage(update.CallbackQuery.From.Id, update.CallbackQuery.Message.MessageId,
		validateString(text),
		tg.StyleMarkdown(tg.MessageStyleMarkdownV2),
		keyboard.Option(),
	)
	if !msg.Ok {
		log.ERROR(msg.Description)
		return
	}
}

func cbChangePage(update tg.UpdateResult, bot *tg.TelegramBot) {
	if !IsEnableTelegramUser(update, bot) {
		log.INFO(fmt.Sprintf("user %d is note create", update.Message.From.Id))
		return
	}
	user := store.Get(update.CallbackQuery.From.Id)
	defer func() {
		store.Set(update.CallbackQuery.From.Id, user)
	}()

	if strings.Contains(update.CallbackQuery.Data, CB_ROUTE_LIST_PREV) && user.NotePage > 0 {
		user.NotePage -= 1
	}
	if strings.Contains(update.CallbackQuery.Data, CB_ROUTE_LIST_NEXT) {
		user.NotePage += 1
	}

	keyboard, err := KeyboardList(&user)
	if err != nil {
		log.ERROR(fmt.Sprintf("%v keyboard error %e", user.Id, err))
		return
	}
	msg := bot.EditMessage(update.CallbackQuery.From.Id, update.CallbackQuery.Message.MessageId,
		validateString(update.CallbackQuery.Message.Text),
		tg.StyleMarkdown(tg.MessageStyleMarkdownV2),
		keyboard.Option(),
	)
	if !msg.Ok {
		log.ERROR(msg.Description)
		return
	}
}

func cbList(update tg.UpdateResult, bot *tg.TelegramBot) {
	if !IsEnableTelegramUser(update, bot) {
		log.INFO(fmt.Sprintf("user %d is note create", update.Message.From.Id))
		return
	}
	user := store.Get(update.CallbackQuery.From.Id)
	defer func() {
		store.Set(update.CallbackQuery.From.Id, user)
	}()
	log.DEBUG(fmt.Sprint(user))
	keyboard, err := KeyboardList(&user)
	if err != nil {
		log.ERROR(fmt.Sprintf("%v keyboard error: %e", user.Id, err))
		return
	}

	text := "*Список заметок:*"
	msg := bot.SendMessage(
		update.CallbackQuery.From.Id,
		validateString(text),
		keyboard.Option(),
		tg.StyleMarkdown(tg.MessageStyleMarkdownV2),
	)
	if !msg.Ok {
		log.ERROR("error send message", text)
		log.ERROR(msg.Description)
		return
	}

}

func cbNew(update tg.UpdateResult, bot *tg.TelegramBot) {
	log.DEBUG("route new")
	var err error
	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)

	if !IsEnableTelegramUser(update, bot) {
		log.INFO(fmt.Sprintf("user %d is note create", update.CallbackQuery.From.Id))
		return
	}
	user := store.Get(update.CallbackQuery.From.Id)
	defer func() {
		store.Set(update.CallbackQuery.From.Id, user)
	}()

	if update.CallbackQuery.Id == "" {
		log.INFO(fmt.Sprintf("%v callback text %s", update.CallbackQuery.From.Id, update.CallbackQuery.Data))
		return
	}

	_qb := ""
	state := ""

	_, err = fmt.Sscan(update.CallbackQuery.Data, &_qb, &state)
	if err != nil {
		log.WARN(fmt.Sprintf("%v cant sscan callback data: `%s`, error: %e", update.CallbackQuery.From.Id, update.CallbackQuery.Data, err))
	}

	keyboard, err := KeyboardNewNote(&user)
	if err != nil {
		log.ERROR(fmt.Sprintf("%v keyboard error: %e", user.Id, err))
		return
	}

	switch state {
	case "url":
		user.Status = USER_STATUS_NEW_URL
		msg := bot.SendMessage(update.CallbackQuery.From.Id, "Введите ссылку:", keyboard.Option())
		if !msg.Ok {
			log.ERROR(msg.Description)
		}
		return
	case "description":
		user.Status = USER_STATUS_NEW_DESCRIPTION
		msg := bot.SendMessage(update.CallbackQuery.From.Id, "Добавьте описание:", keyboard.Option())
		if !msg.Ok {
			log.ERROR(msg.Description)
		}
		return
	case "tags":
		user.Status = USER_STATUS_NEW_TAGS
		msg := bot.SendMessage(update.CallbackQuery.From.Id, "Добавьте теги (через пробел):", keyboard.Option())
		if !msg.Ok {
			log.ERROR(msg.Description)
		}
		return
	case "save":
		err = models.NewNote(ctx, user.Id, user.Note.Name, user.Note.URL, user.Note.Description, user.Note.Tags)
		if err != nil {
			log.ERROR(err.Error())
			msg := bot.SendMessage(update.CallbackQuery.From.Id, "Ошибка на сервере")
			if !msg.Ok {
				log.ERROR(msg.Description)
			}
			return
		}
		err = user.SaveNote()
		if err != nil {
			log.ERROR(err.Error())
			msg := bot.SendMessage(update.CallbackQuery.From.Id, "Ошибка добавления заметки")
			if !msg.Ok {
				log.ERROR(msg.Description)
			}
			return
		}
		log.INFO(fmt.Sprintf("%v saved note", update.CallbackQuery.From.Id))

		msg := bot.SendMessage(update.CallbackQuery.From.Id, "Заметка сохранена")
		if !msg.Ok {
			log.ERROR(msg.Description)
			return
		}
		user.Status = USER_STATUS_NONE
		return

	}

	user.Status = USER_STATUS_NEW

	msg := bot.SendMessage(update.CallbackQuery.From.Id, "Введите название:")
	if !msg.Ok {
		log.ERROR(msg.Description)
		return
	}

}

func cbEdit(update tg.UpdateResult, bot *tg.TelegramBot) {
	if !IsEnableTelegramUser(update, bot) {
		log.INFO(fmt.Sprintf("user %d is note create", update.CallbackQuery.From.Id))
		return
	}
	user := store.Get(update.CallbackQuery.From.Id)
	defer func() {
		store.Set(update.CallbackQuery.From.Id, user)
	}()

	qb := ""
	noteId := 0

	fmt.Sscan(update.CallbackQuery.Data, &qb, &noteId)
	log.DEBUG(fmt.Sprintf("%s %v", qb, noteId))

	user.Status = USER_STATUS_EDIT

	note, err := models.GetNote(int64(noteId))
	if err != nil {
		log.ERROR(fmt.Sprintf("%v not found note by callback data %s, error: %e", update.CallbackQuery.From.Id, update.CallbackQuery.Data, err))
		bot.SendMessage(update.CallbackQuery.From.Id, "Ошибка поиска заметки")
		return
	}
	user.Note.Id = note.Id
	user.Note.Name = note.Title
	user.Note.URL = note.Url
	user.Note.Description = note.Description
	tags, err := models.GetTagsByNoteId(note.Id)
	if err != nil {
		log.ERROR(fmt.Sprintf("%v database error: %e", update.CallbackQuery.From.Id, err))
		return
	}
	user.Note.Tags = make([]string, len(tags))
	for i, tag := range tags {
		user.Note.Tags[i] = tag.Title
	}
	if note.Description == "" {
		note.Description = "-"
	}
	if note.Url == "" {
		note.Url = "-"
	}
	if note.Title == "" {
		note.Title = "-"
	}
	text := fmt.Sprintf("*Название:* _%s_ \n*Ссылка:* _%s_ \n*Описание:* _%s_ \n*Теги:* _ %s _",
		note.Title, note.Url, note.Description, strings.Join(user.Note.Tags, " "))
	keyboard, err := KeyboardEditNote(&user)
	if err != nil {
		log.ERROR(fmt.Sprintf("%v keyboard error: %e", user.Id, err))
		return
	}

	msg := bot.SendMessage(update.CallbackQuery.From.Id, validateString(text),
		keyboard.Option(),
		tg.StyleMarkdown(tg.MessageStyleMarkdownV2))
	if !msg.Ok {
		log.ERROR(msg.Description)
	}
}

func cbEditing(update tg.UpdateResult, bot *tg.TelegramBot) {
	if !IsEnableTelegramUser(update, bot) {
		log.INFO(fmt.Sprintf("user %d is note create", update.CallbackQuery.From.Id))
		return
	}
	user := store.Get(update.CallbackQuery.From.Id)
	defer func() {
		store.Set(update.CallbackQuery.From.Id, user)
	}()
	if user.Note.Id == 0 {
		msg := bot.SendMessage(update.CallbackQuery.From.Id, "Заметка не выбрана")
		if !msg.Ok {
			log.ERROR(msg.Description)
		}
		return
	}

	qb := ""
	state := ""

	fmt.Sscan(update.CallbackQuery.Data, &qb, &state)
	log.DEBUG(fmt.Sprintf("%s %v", qb, state))
	keyboard, err := KeyboardEditNote(&user)
	if err != nil {
		log.ERROR(fmt.Sprintf("%v keyboard error: %e", user.Id, err))
		return
	}

	switch state {
	case "title":
		user.Status = USER_STATUS_EDIT
		msg := bot.SendMessage(update.CallbackQuery.From.Id, "Введите название", keyboard.Option())
		if !msg.Ok {
			log.ERROR(msg.Description)
		}
		return
	case "url":
		user.Status = USER_STATUS_EDIT_URL
		msg := bot.SendMessage(update.CallbackQuery.From.Id, "Введите ссылку", keyboard.Option())
		if !msg.Ok {
			log.ERROR(msg.Description)
		}
		return
	case "description":
		user.Status = USER_STATUS_EDIT_DESCRIPTION
		msg := bot.SendMessage(update.CallbackQuery.From.Id, "Введите описание", keyboard.Option())
		if !msg.Ok {
			log.ERROR(msg.Description)
		}
		return
	case "tags":
		user.Status = USER_STATUS_EDIT_TAGS
		msg := bot.SendMessage(update.CallbackQuery.From.Id, "Введите теги (через пробел):", keyboard.Option())
		if !msg.Ok {
			log.ERROR(msg.Description)
		}
		return
	case "update":
		ctx, _ := context.WithTimeout(context.Background(), time.Second*10)
		err := models.UpdNote(
			ctx,
			models.Note{
				Id:          user.Note.Id,
				UserId:      user.Id,
				Title:       user.Note.Name,
				Url:         user.Note.URL,
				Description: user.Note.Description,
			},
			user.Note.Tags,
		)
		if err != nil {
			log.ERROR(err.Error())
			return
		}
		user.Status = USER_STATUS_NONE
		msg := bot.SendMessage(update.CallbackQuery.From.Id, "Заметка обновлена")
		if !msg.Ok {
			log.ERROR(msg.Description)
		}
		return
	}

}

func cbDelete(update tg.UpdateResult, bot *tg.TelegramBot) {
	if !IsEnableTelegramUser(update, bot) {
		log.INFO(fmt.Sprintf("user %d is note create", update.CallbackQuery.From.Id))
		return
	}
	user := store.Get(update.CallbackQuery.From.Id)
	defer func() {
		store.Set(update.CallbackQuery.From.Id, user)
	}()

	cb := ""
	var noteId int64

	fmt.Sscan(update.CallbackQuery.Data, &cb, &noteId)

	if noteId == 0 {
		log.ERROR(fmt.Sprintf("not dound note id in data: %s", update.CallbackQuery.Data))
		return
	}

	note, err := models.GetNote(noteId)
	if err != nil {
		log.ERROR(fmt.Sprintf("database error: %e", err))
		bot.SendMessage(update.CallbackQuery.From.Id, "Ошибка поиска заметки")
		return
	}
	if note.Id == 0 {
		bot.SendMessage(update.CallbackQuery.From.Id, "Заметка не найдена")
		log.DEBUG(fmt.Sprint(note))
		return
	}
	if note.UserId != user.Id {
		bot.SendMessage(update.CallbackQuery.From.Id, "Заметка не найдена")
		log.ERROR(fmt.Sprintf("user with id=%v, try deleting note by user id=%v", user.Id, note.UserId))
		return
	}

	err = models.DeleteNote(noteId)
	if err != nil {
		log.ERROR(fmt.Sprintf("database error: %e", err))
		bot.SendMessage(update.CallbackQuery.From.Id, "Ошибка удаления заметки")
		return
	}

	bot.SendMessage(update.CallbackQuery.From.Id, fmt.Sprintf("Заметка \"%s\" удалена", note.Title))
}

func tags(update tg.UpdateResult, bot *tg.TelegramBot) {
	if !IsEnableTelegramUser(update, bot) {
		log.INFO(fmt.Sprintf("user %d is note create", update.Message.From.Id))
		return
	}
	user := store.Get(update.Message.From.Id)
	defer func() {
		store.Set(update.Message.From.Id, user)
	}()

	_tags, err := models.GetTagsByUserId(user.Id)
	if err != nil {
		log.ERROR(fmt.Sprintf("error getting tags for user %d", update.Message.From.Id), err.Error())
	}
	tags := make([]string, len(_tags))

	for i, t := range _tags {
		tags[i] = t.Title
	}
	// сортируй tags по алфавиту

	keyboard, _ := KeyboardTags(tags)
	msg := bot.SendMessage(update.Message.From.Id, "Ваши теги", keyboard.Option())
	if !msg.Ok {
		log.ERROR(msg.Description)
		return
	}

}

func cbSearchByTag(update tg.UpdateResult, bot *tg.TelegramBot) {
	if !IsEnableTelegramUser(update, bot) {
		log.INFO(fmt.Sprintf("user %d is note create", update.CallbackQuery.From.Id))
		return
	}
	user := store.Get(update.CallbackQuery.From.Id)
	defer func() {
		store.Set(update.CallbackQuery.From.Id, user)
	}()

	cb := ""
	tag := ""
	fmt.Sscan(update.CallbackQuery.Data, &cb, &tag)

	if tag == "" {
		log.ERROR(fmt.Sprintf("%s, tag is empty", update.CallbackQuery.Data))
		bot.SendMessage(update.CallbackQuery.From.Id, "Тег не выбран")
		return
	}

	user.SearchTag = tag

	keyboard, _ := KeyboardListByTag(&user, tag)
	msg := bot.SendMessage(update.CallbackQuery.From.Id, fmt.Sprintf("Заметки по тегу \"%s\"", tag), keyboard.Option())
	if !msg.Ok {
		log.ERROR(msg.Description)
		return
	}
}

func cbChangePageByTag(update tg.UpdateResult, bot *tg.TelegramBot) {
	if !IsEnableTelegramUser(update, bot) {
		log.INFO(fmt.Sprintf("user %d is note create", update.CallbackQuery.From.Id))
		return
	}
	user := store.Get(update.CallbackQuery.From.Id)
	defer func() {
		store.Set(update.CallbackQuery.From.Id, user)
	}()

	if strings.Contains(update.CallbackQuery.Data, CB_ROUTE_SEARCH_TAG_PREV) && user.NotePage > 0 {
		user.NotePage -= 1
	}
	if strings.Contains(update.CallbackQuery.Data, CB_ROUTE_SEARCH_TAG_NEXT) {
		user.NotePage += 1
	}

	keyboard, err := KeyboardListByTag(&user, user.SearchTag)
	if err != nil {
		log.ERROR(fmt.Sprintf("%v keyboard error %e", user.Id, err))
		return
	}
	msg := bot.EditMessage(update.CallbackQuery.From.Id, update.CallbackQuery.Message.MessageId,
		validateString(update.CallbackQuery.Message.Text),
		tg.StyleMarkdown(tg.MessageStyleMarkdownV2),
		keyboard.Option(),
	)
	if !msg.Ok {
		log.ERROR(msg.Description)
		return
	}
}
