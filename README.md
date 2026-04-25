## Теперь:

Я пересмотрел работу с конфигом - важно чтобы был файл app.env(шаблон снизу),\
в докере не указываются енв переменные(чтобы чувствительные данные не оставлять),\
но дефолтный шаблон app.env позвоялет указывать их, значения из файла будут перекрыты енв переменными

# LinkTracker

**LinkTracker** – Telegram-бот, который отслеживает изменения на веб-страницах и оперативно информирует пользователя о них.

---
### Конфиг
`app.env` шаблон:
```text
bot{
    app_telegram_token = "PUT_YOUR_TOKEN"
    app_telegram_token = ${?APP_TELEGRAM_TOKEN}

    scrapper_addr_grpc = "scrapper:50051"
    scrapper_addr_grpc = ${?SCRAPPER_ADDR_GRPC}

    bot_addr_grpc = "0.0.0.0:50052"
    bot_addr_grpc = ${?BOT_ADDR_GRPC}

    bot_addr_http = "0.0.0.0:8082"
    bot_addr_http = ${?BOT_ADDR_HTTP}

    bot_disable_telegram = false
    bot_disable_telegram = ${?BOT_DISABLE_TELEGRAM}
}

scrapper{
    scrapper_addr_grpc = "0.0.0.0:50051"
    scrapper_addr_grpc = ${?SCRAPPER_ADDR_GRPC}

    scrapper_addr_http = "0.0.0.0:8080"
    scrapper_addr_http = ${?SCRAPPER_ADDR_HTTP}

    bot_addr_grpc = "bot:50052"
    bot_addr_grpc = ${?BOT_ADDR_GRPC}

    github_token = ""
    github_token = ${?GITHUB_TOKEN}

    stack_overflow_key = ""
    stack_overflow_key = ${?STACK_OVERFLOW_KEY}

    minutes_interval_check = 1
    minutes_interval_check = ${?MINUTES_INTERVAL_CHECK}

    postgres_dsn="postgres://postgres:postgres@postgres:5432/link_tracker?sslmode=disable"
    postgres_dsn = ${?POSTGRES_DSN}

    migrations_path ="/app/migrations"
    migrations_path = ${?MIGRATIONS_PATH}

    db_access_type="orm"
    db_access_type = ${?DB_ACCESS_TYPE}

    batch_size = 100
    batch_size = ${?BATCH_SIZE}

    worker_count = 4
    worker_count = ${?WORKER_COUNT}
}
```
---
## Команда запуска
```bash
docker compose up --build
```
---
## Для удобства:
Команда запуска линетера
```bash
golangci-lint run --fix
```
Команда запуска кодогенерации
```bash
go generate ./...
```

```bash
task generate
```