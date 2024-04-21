package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
	"github.com/playmixer/corvid/logger"
)

var (
	DB  *sql.DB
	log *logger.Logger
)

func init() {
	log = logger.New("database")
}

/*
CREATE TABLE public.users (

	id int4 GENERATED ALWAYS AS IDENTITY( INCREMENT BY 1 MINVALUE 1 MAXVALUE 2147483647 START 1 CACHE 1 NO CYCLE) NOT NULL,
	tg_chat_id int4 NULL,
	tg_username varchar NULL,
	CONSTRAINT user_pk PRIMARY KEY (id)

);
CREATE UNIQUE INDEX user_tg_chat_id_idx ON public.users USING btree (tg_chat_id);
*/
type User struct {
	Id         int64  `json:"id"`
	TgChatId   int64  `json:"tg_chat_id"`
	TgUsername string `json:"tg_username"`
}

/*
CREATE TABLE public.tags (

	id int4 GENERATED ALWAYS AS IDENTITY NOT NULL,
	user_id int4 NOT NULL,
	title varchar NOT NULL,
	CONSTRAINT tag_pk PRIMARY KEY (id),
	CONSTRAINT tag_user_fk FOREIGN KEY (user_id) REFERENCES public.users(id)

);
*/
type Tag struct {
	Id     int64  `json:"id"`
	UserId int64  `json:"user_id"`
	Title  string `json:"title"`
}

/*
CREATE TABLE public.notes (

	id int4 GENERATED ALWAYS AS IDENTITY( INCREMENT BY 1 MINVALUE 1 MAXVALUE 2147483647 START 1 CACHE 1 NO CYCLE) NOT NULL,
	user_id int4 NOT NULL,
	title varchar NOT NULL,
	url varchar NULL,
	description varchar NULL,
	CONSTRAINT note_pk PRIMARY KEY (id),
	CONSTRAINT note_user_fk FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE

);
*/
type Note struct {
	Id          int64  `json:"id"`
	UserId      int64  `json:"user_id"`
	Title       string `json:"title"`
	Url         string `json:"url"`
	Description string `json:"description"`
}

/*
CREATE TABLE public.tags_to_note (
	id int4 GENERATED ALWAYS AS IDENTITY NOT NULL,
	note_id int4 NOT NULL,
	tag_id int4 NOT NULL,
	CONSTRAINT tags_to_note_pk PRIMARY KEY (id),
	CONSTRAINT tags_to_note_notes_fk FOREIGN KEY (note_id) REFERENCES public.notes(id) ON DELETE CASCADE,
	CONSTRAINT tags_to_note_tags_fk FOREIGN KEY (tag_id) REFERENCES public.tags(id) ON DELETE CASCADE
);
*/

func Connect(host, port, user, password, dbname string) (*sql.DB, error) {
	psqlconn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)

	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func GetUser(id int64) (User, error) {
	row := DB.QueryRow("select * from \"users\" where id = $1 limit 1", id)
	var user = User{}
	err := row.Scan(&user.Id, &user.TgChatId, &user.TgUsername)
	if err != nil {
		return user, err
	}

	return user, nil
}

func GetUserByTelegramId(id int64) (User, error) {
	var user = User{}
	row := DB.QueryRow("select id, tg_chat_id, tg_username from \"users\" where tg_chat_id = $1 limit 1", id)

	err := row.Scan(&user.Id, &user.TgChatId, &user.TgUsername)
	if err != nil && errors.Is(sql.ErrNoRows, err) {
		return user, nil
	}
	if err != nil {
		return user, fmt.Errorf("error scan select by telegram id: %s", err)
	}

	return user, nil
}

func GetUserByUserId(id int64) (User, error) {
	var user = User{}
	row := DB.QueryRow("select id, tg_chat_id, tg_username from \"users\" where id = $1 limit 1", id)

	err := row.Scan(&user.Id, &user.TgChatId, &user.TgUsername)
	if err != nil && errors.Is(sql.ErrNoRows, err) {
		return user, nil
	}
	if err != nil {
		log.ERROR(fmt.Sprintf("error scan select by user id: %s", err))
		return user, err
	}

	return user, nil
}

func NewUser(tgChatId int64, tgUsername string) (id int64, err error) {
	err = DB.QueryRow("insert into \"users\" (tg_chat_id, tg_username) values ($1, $2) returning id", tgChatId, tgUsername).Scan(&id)
	return
}

func GetTagsByNoteId(noteId int64) ([]Tag, error) {
	var tags []Tag
	rows, err := DB.Query(`select tags.id, tags.user_id, tags.title from tags 
	join tags_to_note ttn on tags.id = ttn.tag_id 
	join notes on notes.id = ttn.note_id 
	where notes.id = $1`, noteId)
	if err != nil && !errors.Is(sql.ErrNoRows, err) {
		return tags, nil
	}
	if err != nil {
		return tags, err
	}
	for rows.Next() {
		tag := Tag{}
		err = rows.Scan(&tag.Id, &tag.UserId, &tag.Title)
		if err != nil {
			return tags, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func GetTagsByUserId(userId int64) ([]Tag, error) {
	var tags []Tag
	rows, err := DB.Query(`select t.id, t.user_id, t.title from tags t
	where t.user_id = $1`, userId)
	if err != nil && !errors.Is(sql.ErrNoRows, err) {
		return tags, nil
	}
	if err != nil {
		return tags, err
	}
	for rows.Next() {
		tag := Tag{}
		err = rows.Scan(&tag.Id, &tag.UserId, &tag.Title)
		if err != nil {
			return tags, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func NewNote(ctx context.Context, userId int64, title, url, description string, _tags []string) error {
	var err error
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tags := []Tag{}
	for _, _tag := range _tags {
		if strings.Trim(_tag, " ") == "" {
			continue
		}
		tag := Tag{
			UserId: userId,
			Title:  _tag,
		}
		err = tx.QueryRowContext(ctx, "select id from tags where user_id = $1 and title = $2", userId, _tag).Scan(&tag.Id)
		if err != nil && !errors.Is(sql.ErrNoRows, err) {
			return err
		}
		if errors.Is(sql.ErrNoRows, err) {
			row := tx.QueryRowContext(ctx, "insert into tags (user_id, title) values ($1, $2) returning id", userId, _tag)
			err = row.Scan(&tag.Id)
			if err != nil {
				return err
			}
		}
		tags = append(tags, tag)
	}

	note := Note{}
	err = tx.QueryRowContext(ctx, "insert into \"notes\" (user_id, title, url, description) values ($1, $2, $3, $4) returning id", userId, title, url, description).Scan(&note.Id)
	if err != nil {
		return err
	}

	for _, tag := range tags {
		_, err = tx.ExecContext(ctx, "insert into tags_to_note (note_id, tag_id) values ($1, $2)", note.Id, tag.Id)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func RemoveNoteTag(ctx context.Context, tagId int64, noteId int64) error {
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "delete from tags_to_note where tag_id = $1 and note_id = $2", tagId, noteId)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func UpdNote(ctx context.Context, note Note, newTags []string) error {
	var err error
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		log.ERROR(err.Error())
		return err
	}
	defer tx.Rollback()
	oldTags, err := GetTagsByNoteId(note.Id)
	if err != nil {
		log.ERROR(err.Error())
		return err
	}

	diffTags := map[string]bool{}
	for _, _tag := range newTags {
		diffTags[_tag] = true
	}
	for _, _tag := range oldTags {
		if _, ok := diffTags[_tag.Title]; ok {
			delete(diffTags, _tag.Title)
		} else {
			_, err = tx.ExecContext(ctx, "delete from tags_to_note where tag_id = $1 and note_id = $2", _tag.Id, note.Id)
			if err != nil {
				log.ERROR(err.Error())
				return err
			}
		}
	}

	tags := []Tag{}
	for _tag := range diffTags {
		if strings.Trim(_tag, " ") == "" {
			continue
		}
		tag := Tag{
			UserId: note.UserId,
			Title:  _tag,
		}
		row := tx.QueryRowContext(ctx, "select id from tags where user_id = $1 and title = $2", note.UserId, _tag)
		err = row.Scan(&tag.Id)
		if err != nil && !errors.Is(sql.ErrNoRows, err) {
			log.ERROR(err.Error())
			return err
		}
		if tag.Id == 0 {
			row = tx.QueryRowContext(ctx, "insert into tags (user_id, title) values ($1, $2) returning id", note.UserId, _tag)
			err = row.Scan(&tag.Id)
			if err != nil {
				log.ERROR(err.Error())
				return err
			}
		}
		tags = append(tags, tag)
	}

	_, err = tx.ExecContext(ctx, "update notes set title = $1, url = $2, description = $3 where id = $4 and user_id = $5", note.Title, note.Url, note.Description, note.Id, note.UserId)
	if err != nil {
		log.ERROR(err.Error())
		return err
	}

	for _, tag := range tags {
		_, err = tx.ExecContext(ctx, "insert into tags_to_note (note_id, tag_id) values ($1, $2)", note.Id, tag.Id)
		if err != nil {
			log.ERROR(fmt.Sprintf("note %v, tag %v, error: %e", note.Id, tag.Id, err))
			return err
		}
	}

	return tx.Commit()
}

func GetNotes(userId int64) ([]Note, error) {
	var err error
	notes := []Note{}
	rows, err := DB.Query("select id, title, url, description from notes where user_id = $1", userId)
	if err != nil && !errors.Is(sql.ErrNoRows, err) {
		return nil, err
	}
	if errors.Is(sql.ErrNoRows, err) {
		return notes, nil
	}
	defer rows.Close()

	for rows.Next() {
		note := Note{}
		err = rows.Scan(&note.Id, &note.Title, &note.Url, &note.Description)
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return notes, nil
}
func GetNotesByTag(userId int64, tag string) ([]Note, error) {
	var err error
	notes := []Note{}
	rows, err := DB.Query(`select notes.id, notes.title, url, description from notes 
	join tags_to_note ttn on ttn.note_id = notes.id 
	join tags t on t.id = ttn.tag_id and t.user_id = notes.user_id 
	where notes.user_id = $1
	and t.title = $2`, userId, tag)
	if err != nil && !errors.Is(sql.ErrNoRows, err) {
		return nil, err
	}
	if errors.Is(sql.ErrNoRows, err) {
		return notes, nil
	}
	defer rows.Close()

	for rows.Next() {
		note := Note{}
		err = rows.Scan(&note.Id, &note.Title, &note.Url, &note.Description)
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return notes, nil
}

func GetNote(noteId int64) (Note, error) {
	var err error
	note := Note{}
	row := DB.QueryRow("select id, title, url, description, user_id from notes where id = $1", noteId)
	err = row.Scan(&note.Id, &note.Title, &note.Url, &note.Description, &note.UserId)
	if err != nil && !errors.Is(sql.ErrNoRows, err) {
		return note, err
	}
	if errors.Is(sql.ErrNoRows, err) {
		return note, nil
	}

	return note, nil
}

func DeleteNote(noteId int64) error {
	log.DEBUG(fmt.Sprintf("delete note with id=%v", noteId))
	_, err := DB.Exec("delete from notes where id = $1", noteId)
	if err != nil {
		log.ERROR(err.Error())
	}

	return err
}
