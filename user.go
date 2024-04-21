package main

import "sync"

type UserStatus uint

const (
	USER_STATUS_NONE            UserStatus = iota // Без статуса
	USER_STATUS_NEW                               // Добавить заметку (ввести название)
	USER_STATUS_NEW_URL                           // добавить урл для заметки
	USER_STATUS_NEW_DESCRIPTION                   // добавить описание для заметки
	USER_STATUS_NEW_TAGS                          // добавить теги для заметки

	USER_STATUS_DEL UserStatus = iota + 100 //удалить заметку

	USER_STATUS_EDIT             UserStatus = iota + 200 //редактировать заметку
	USER_STATUS_EDIT_URL                                 //редактировать ссылку
	USER_STATUS_EDIT_DESCRIPTION                         //редактировать описание
	USER_STATUS_EDIT_TAGS                                //редактировать теги
)

type Note struct {
	Id          int64
	Name        string
	URL         string
	Description string
	Tags        []string
}

type User struct {
	Id            int64
	Status        UserStatus
	Note          Note
	LastMessageId int64
	NotePage      uint
	SearchTag     string
}

func (u *User) Add(name string) {
	u.Note.Name = name
}

func (u *User) AddUrl(url string) {
	u.Note.URL = url
}

func (u *User) AddDescription(desc string) {
	u.Note.Description = desc
}
func (u *User) AddTags(tags []string) {
	u.Note.Tags = tags
}

func (u *User) SaveNote() error {

	u.Note = Note{}
	return nil
}

type UserStore struct {
	data map[int64]User
	mu   sync.Mutex
}

func (s *UserStore) Get(key int64) User {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; ok {
		return s.data[key]
	}
	return User{}
}

func (s *UserStore) Set(key int64, value User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}
