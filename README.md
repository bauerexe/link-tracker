# LinkTracker

**LinkTracker** – Telegram-бот, который отслеживает изменения на веб-страницах и оперативно информирует пользователя о них.

---
### Конфиг
`app.env` шаблон:
```text
bot{
    app_telegram_token = "PUT_YOUR_TOKEN"
    app_telegram_token = ${?APP_TELEGRAM_TOKEN}

    scrapper_addr_grpc = "0.0.0.0:50051"
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

    bot_addr_grpc = "0.0.0.0:50052"
    bot_addr_grpc = ${?BOT_ADDR_GRPC}

    github_token = "PUT_YOUR_TOKEN"
    github_token = ${?GITHUB_TOKEN}

    stack_overflow_key = "PUT_YOUR_TOKEN"
    stack_overflow_key = ${?STACK_OVERFLOW_KEY}

    minutes_interval_check = 1
    minutes_interval_check = ${?MINUTES_INTERVAL_CHECK}

    postgres_dsn="postgres://postgres:postgres@postgres:5432/link_tracker?sslmode=disable"
    postgres_dsn = ${?POSTGRES_DSN}

    migrations_path ="/app/migrations"
    migrations_path = ${?MIGRATIONS_PATH}

    db_access_type="sql"
    db_access_type = ${?DB_ACCESS_TYPE}
}
```
---
## Команда запуска
в двух терминалах:
```bash
make tg_bot_run
```
```bash
make scrapper_run
```
ИЛИ
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