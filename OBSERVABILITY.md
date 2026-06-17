# Observability

## Что добавлено

Scrapper отдает Prometheus metrics на основном HTTP*порту:

* локально: `http://localhost:8080/metrics`
* внутри Docker Compose: `http://scrapper:8080/metrics`

Bot отдает Prometheus metrics на отдельном metrics*порту:

* локально: `http://localhost:8011/metrics`
* внутри Docker Compose: `http://bot:8011/metrics`

Prometheus добавляет target labels `application` и `application_type` через `monitoring/prometheus/prometheus.yml`.
Для Scrapper используются значения `scrapper`, для Bot * `bot`. Это нужно, чтобы одинаково фильтровать custom, runtime и process metrics.

## Scrapper metrics

`links_on_track_total`

* type: Gauge
* labels: `tracked_source`
* values: `github`, `stackoverflow`, `other`
* meaning: текущее количество активных отслеживаемых ссылок в БД, сгруппированное по нормализованному источнику
* raw URL не используется как label

`request_duration_ms_total`

* type: Histogram
* labels: `scope`, `scope_type`
* scopes:
  * `database`: операции репозиториев и outbox*таблицы, `scope_type` = `chats`, `links`, `github_sync_state`, `outbox_messages`
  * `external_source`: вызовы GitHub и StackOverflow, `scope_type` = `github`, `stackoverflow`
  * `kafka`: публикация outbox*сообщений, `scope_type` = Kafka topic
* LLM*вызовы не добавлены в Scrapper, потому что LLM находится в отдельном `agent`, а не в Scrapper*сервисе

`api_requests_total`

* type: Counter
* labels: `source`, `method`, `path`, `status`
* `source`: `rest` или `grpc`
* `path`: нормализованный путь, numeric path segments заменяются на `{id}`
* `status`: HTTP*compatible status code; gRPC status codes приводятся к HTTP*compatible codes для 5xx error queries

`api_request_duration_ms_total`

* type: Histogram
* labels: `source`, `method`, `path`, `status`
* meaning: latency входящих REST/gRPC запросов Scrapper

## Bot metrics

`command_requests_total`

* type: Counter
* labels: `command`
* allowed values: `start`, `help`, `track`, `untrack`, `list`, `unknown`
* `chat_id`, `user_id` и текст сообщений не используются как labels

`command_duration_ms_total`

* type: Histogram
* labels: `scope`, `scope_type`
* `scope`: `scrapper_sync_api`
* `scope_type`: gRPC method name: `CreateChat`, `DeleteChat`, `CreateLink`, `GetLinks`, `DeleteLink`

`sent_notification_total`

* type: Counter
* labels: none
* инкрементируется после успешной отправки update notification пользователю через gRPC Bot endpoint или Kafka consumer path

## Monitoring stack

Конфиги:

* Prometheus: `monitoring/prometheus/prometheus.yml`
* Grafana datasource provisioning: `monitoring/grafana/provisioning/datasources/prometheus.yml`
* Grafana dashboard provisioning: `monitoring/grafana/provisioning/dashboards/dashboards.yml`
* Dashboard JSON: `monitoring/grafana/dashboards/link*tracker*observability.json`
* PromQL examples: `example_pql.txt`

Запуск:

```bash
docker compose up **build
```

Prometheus:

* UI: `http://localhost:9090`
* targets: `http://localhost:9090/targets`
* jobs: `scrapper`, `bot`

Grafana:

* UI: `http://localhost:3000`
* login/password: `admin` / `admin`
* dashboard folder: `Link Tracker`
* dashboard title: `Link Tracker Observability`

## Dashboard panels

RED panels:

* Request Rate: `sum(rate(api_requests_total{application=~"$application"}[$__rate_interval])) by (application)`
* Error Rate: `sum(rate(api_requests_total{application=~"$application", status=~"5.."}[$__rate_interval])) by (application)`
* API Latency: `histogram_quantile` over `api_request_duration_ms_total_bucket`
* Memory Usage: `process_resident_memory_bytes` and `go_memstats_alloc_bytes`

Business panels:

* User Messages Per Second: `sum(rate(command_requests_total[$__rate_interval])) by (command)`
* Active Tracked Links: `sum(links_on_track_total) by (tracked_source)`
* Scrape Operation Latency: `histogram_quantile` over `request_duration_ms_total_bucket{scope="external_source"}`
* Bot Command Duration: `histogram_quantile` over `command_duration_ms_total_bucket`
* Telegram Bot Requests: `sum(rate(command_requests_total[$__rate_interval]))`
* Sent Notifications: `sum(increase(sent_notification_total[$__range]))` and `sum(sent_notification_total)`

Полный список PromQL*запросов лежит в `example_pql.txt`.

## Как проверить

Проверить Scrapper metrics:

```bash
curl http://localhost:8080/metrics
```

Проверить Bot metrics:

```bash
curl http://localhost:8011/metrics
```

Проверить Prometheus targets:

```bash
curl http://localhost:9090/api/v1/targets
```

Проверить отдельные метрики в Prometheus UI:

* `api_requests_total`
* `links_on_track_total`
* `request_duration_ms_total_bucket`
* `command_requests_total`
* `command_duration_ms_total_bucket`
* `sent_notification_total`
* `process_resident_memory_bytes`
