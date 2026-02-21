# LinkTracker

**LinkTracker** – Telegram-бот, который отслеживает изменения на веб-страницах и оперативно информирует пользователя о них.

---
### Конфиг
`.env` file example:
```text
bot {
app_telegram_token = "EXAMPLE_TOKEN"
app_telegram_token = ${?APP_TELEGRAM_TOKEN}
}
```
---
## Команда запуска
```bash
make build && ./bin/bot
```
&uarr;\
Эквивалентны\
&darr;

```bash
make tg_bot_run
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
