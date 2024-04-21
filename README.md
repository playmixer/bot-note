# Телеграм бот для заметок
- создание заметок
- редактирование заметок
- удаление заметок
- поиска заметок по тегу

feature
- напоминание

# Install
### Clone
```
git clone https://github.com/playmixer/bot-note.git
```
### cd bot-note && vi .env
```
TELEGRAM_BOT_API_KEY={telegram_api_token}

DB_HOST=localhost
DB_PORT=5432
DB_USER={database user}
DB_PASSWORD={database password}
DB_NAME={database name}
```

### Run
```
go run .
```
