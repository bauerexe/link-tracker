# LinkTracker

**LinkTracker** – Telegram-бот, который отслеживает изменения на веб-страницах и оперативно информирует пользователя о них.

---
### Конфиг
`app.env` file simple example:
```text
bot{
    app_telegram_token = ""
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

    github_token = ""
    github_token = ${?GITHUB_TOKEN}

    stack_overflow_key = ""
    stack_overflow_key = ${?STACK_OVERFLOW_KEY}

    minutes_interval_check = 1
    minutes_interval_check = ${?MINUTES_INTERVAL_CHECK}
}}
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
### необходимо будет 
в `docker-compose.yml`
указать -
`APP_TELEGRAM_TOKEN: "${APP_TELEGRAM_TOKEN:-}"`\
### при желании
`GITHUB_TOKEN: "${GITHUB_TOKEN:-}"`\
`STACK_OVERFLOW_KEY: "${STACK_OVERFLOW_KEY:-}"`\
Без них будут действовать более строгие лимиты API
запросы будут выполняться без аутентифиции

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