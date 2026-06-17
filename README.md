# LinkTracker

## Monitoring

Prometheus and Grafana are included in `docker-compose.yml`.

- Scrapper metrics: `http://localhost:8080/metrics`
- Bot metrics: `http://localhost:8011/metrics`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000` (`admin` / `admin`)
- Dashboard JSON: `monitoring/grafana/dashboards/link-tracker-observability.json`
- PromQL examples: `example_pql.txt`
- Full notes: `OBSERVABILITY.md`

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
