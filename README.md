# LinkTracker

**LinkTracker** – Telegram-бот, который отслеживает изменения на веб-страницах и оперативно информирует пользователя о них.

---
### Конфиг
`app.env` шаблон:
```
Находиться в app.env.example
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

---

## Нагрузочное тестирование

Подготовлены артефакты для сравнения производительности `GET /links` и `POST /links`:

- k6 сценарий с профилем нагрузки 100:1 (`GET`:`POST/DELETE`), ramp-up 1 мин, stage >= 5 мин: `load/k6/list_load.js`.
- SQL скрипт для наполнения данных (1000 чатов × 100 ссылок = ~100k): `load/sql/seed_100k.sql`.
