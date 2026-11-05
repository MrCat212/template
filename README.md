# TripGo — репозиторий для лабораторных работ

Заготовка курса «Разработка микросервисов на Go». Здесь вы делаете все пять
работ: каждая следующая продолжает предыдущую, переписывать сервис с нуля не
нужно.

## Что делать сразу

1. **Fork** этого репозитория к себе. Форк нужен, чтобы преподаватели видели
   список всех работ курса одной страницей.
2. Заведите модуль:

```bash
git clone git@github.com:<ваш-логин>/<ваш-репозиторий>.git
cd <ваш-репозиторий>
go mod init github.com/<ваш-логин>/<ваш-репозиторий>
```

Путь модуля потом не меняется — иначе придётся править все импорты. Проще всего
взять адрес своего репозитория, каким бы он ни был.

Дальше — [`homework/docs/getting-started.md`](https://github.com/course-go-autumn-2026/course/blob/main/homework/docs/getting-started.md)
в репозитории курса: инструменты, окружение, миграции, вид сданной работы.

## Где что лежит

| Что | Где |
|---|---|
| Задания, документация, контракты | [`course-go-autumn-2026/course`](https://github.com/course-go-autumn-2026/course) |
| Слайды и записи лекций | [`lections/`](https://github.com/course-go-autumn-2026/course/tree/main/lections) |
| Как оценивают, дедлайны, порядок сдачи | [`homework/docs/grading.md`](https://github.com/course-go-autumn-2026/course/blob/main/homework/docs/grading.md) |
| Локальное окружение и утилита `tripgoctl` | [`course-go-autumn-2026/course-infra`](https://github.com/course-go-autumn-2026/course-infra) |

Задания появляются по мере курса, каждое — после своей пары лекций.

## Как сдавать

Ветка `homework/NN` от `main`, pull request в `main` своего форка, ССЫЛКУ НИКУДА СКИДЫВАТЬ НЕ НУЖНО, ревьюер сам найдет ее внутри вашего форка. Подробно — в `grading.md` репозитория курса.

## Чужие работы

Форки видны всем, включая ваши. Смотреть чужие решения, пока идёт курс, —
плохая идея: одинаковый код виден сразу, а разбираться на защите придётся
самому.

---

## Лабораторная работа 1. HTTP API и PostgreSQL

В этой работе реализован простой сервис для управления поездками на Go.

Сервис позволяет:
- создавать поездки;
- получать информацию о поездке по ID;
- завершать поездки;
- проверять состояние сервера и подключение к базе данных.

Данные хранятся в PostgreSQL.

### Используемые технологии

- Go
- PostgreSQL
- pgxpool
- Squirrel
- Chi
- Goose
- oapi-codegen

### Запуск проекта

Для работы нужны Go, Docker и tripgoctl.

Сначала запускаем окружение:

```bash
tripgoctl cluster start
tripgoctl environment start
```

После запуска окружения применяем миграции:

```bash
make migrate
```

Запускаем сервер:

```bash
make run
```

По умолчанию сервер доступен на порту 8080.

### Основные команды

```bash
make migrate
make run
make generate
make test
```

- `migrate` — применение миграций базы данных.
- `run` — запуск сервиса.
- `generate` — генерация Go-кода из OpenAPI.
- `test` — запуск тестов.

### Переменные окружения

Настройки берутся из файла `.env`, который создаётся через tripgoctl.

Основные переменные:

- `HTTP_ADDR` — адрес HTTP-сервера.
- `LOG_LEVEL` — уровень логирования.
- `SHUTDOWN_TIMEOUT` — время на остановку сервера.
- `DATABASE_URL` — подключение к PostgreSQL.
- `DATABASE_MAX_CONNS` — максимальное количество соединений.
- `DATABASE_MIN_CONNS` — минимальное количество соединений.
- `DATABASE_MAX_CONN_LIFETIME` — время жизни соединения.
- `DATABASE_CONNECT_TIMEOUT` — таймаут подключения.
- `DATABASE_QUERY_TIMEOUT` — таймаут SQL-запросов.

Также предусмотрены настройки HTTP-таймаутов:

- `HTTP_READ_TIMEOUT`
- `HTTP_READ_HEADER_TIMEOUT`
- `HTTP_WRITE_TIMEOUT`
- `HTTP_IDLE_TIMEOUT`

Пример настроек находится в `.env.example`.

### HTTP API

Реализованы следующие запросы:

| Метод | Путь | Назначение |
|---|---|---|
| POST | /api/v1/trips | Создание поездки |
| GET | /api/v1/trips/{tripId} | Получение поездки |
| POST | /api/v1/trips/{tripId}/finish | Завершение поездки |
| GET | /health | Проверка работы сервера |
| GET | /ready | Проверка подключения к БД |

### Решения

#### Транзакции

Для управления транзакциями реализован TxManager.

Транзакция открывается через Do() и передаётся через context.Context.

Если операция прошла успешно, выполняется COMMIT. Если произошла ошибка или паника, выполняется ROLLBACK.

Если транзакция уже существует, повторный вызов Do() использует её, а не открывает новую.

Для транзакций выбран уровень изоляции Read Committed. Его достаточно для этой работы, потому что основные ограничения дополнительно обеспечиваются PostgreSQL.

#### Создание поездки

При создании поездки данные записываются в таблицы trips и trip_status_history.

Обе операции выполняются в одной транзакции. Если одна из них завершится ошибкой, изменения откатываются.

#### Защита от двух активных поездок

Для таблицы trips создан частичный уникальный индекс по driver_id для поездок со статусом active.

Он не позволяет одному водителю иметь две активные поездки одновременно.

При нарушении ограничения сервис возвращает ошибку 409 driver_busy.

#### Завершение поездки

При завершении статус меняется с active на completed.

Изменение выполняется через UPDATE с проверкой текущего статуса. Это защищает от повторного завершения поездки.

Если поездка уже завершена, возвращается ошибка 409 trip_completed.

#### Проверка состояния

/health проверяет, что HTTP-сервер работает.

/ready дополнительно проверяет доступность PostgreSQL.

Если база недоступна, /ready возвращает 503 Service Unavailable.

---

## Лабораторная работа 2. Архитектура, качество кода и observability

Во второй лабораторной внешний HTTP API остался прежним, но реализация
разделена на слои, добавлены тесты, линтер и наблюдаемость через OpenTelemetry.
Новые HTTP-ручки и миграции для этой работы не добавлялись.

### Архитектура

```text
HTTP-клиент
    |
    v
internal/http/trip/{create,get,finish}
    |
    v
internal/usecase/trip/{create,get,finish}
    |                         \
    v                          v
internal/domain/trip     интерфейсы зависимостей
                               |
                               v
                      internal/repository/trip
                               |
                               v
                       internal/infra/postgres
                               |
                               v
                           PostgreSQL
```

- `cmd/trip-service/main.go` загружает конфигурацию, запускает сервер и
  обеспечивает корректную остановку по сигналам.
- `internal/container` создаёт конкретные зависимости и соединяет их.
- `internal/domain/trip` содержит модель поездки, статусы, переход `Finish`,
  ошибки и тип ошибки валидации; не зависит от HTTP и БД.
- `internal/usecase/trip/{create,get,finish}` реализует сценарии приложения.
  Сценарии не знают о реализации PostgreSQL и HTTP.
- `internal/http/trip/{create,get,finish}` разбирает запросы и возвращает
  ответы по OpenAPI-контракту. `internal/http/domainerror` централизованно
  преобразует доменные ошибки в HTTP Problem по RFC 9457.
- `internal/repository/trip` реализует сохранение и чтение поездок в БД;
  `internal/infra/postgres` управляет пулом соединений и транзакциями.
- `internal/infra/telemetry` настраивает OpenTelemetry, JSON-логи `slog`
  и метрики. `internal/http/middleware` обрабатывает входящие запросы.

Зависимости направлены от внешних слоёв к внутренним. Узкие интерфейсы
объявлены в `contract.go` на стороне **потребителя** (usecase или обработчика),
а не в реализации репозитория. Это позволяет подменять зависимости моками
через `gomock` и добавлять другие транспорты без изменения домена.

### Принятые решения

- Создание поездки и запись первого статуса выполняются в одной транзакции.
  При ошибке обе операции откатываются.
- Уникальный частичный индекс PostgreSQL запрещает две активные поездки
  у одного водителя. Ошибка преобразуется в `409 driver_busy`.
- Завершение поездки — доменный переход `active` → `completed` с условным
  `UPDATE` и записью истории внутри транзакции. Повторная попытка даёт
  `409 trip_completed`.
- Ошибки валидации преобразуются в `400 invalid_request`, отсутствие поездки
  — в `404 trip_not_found`, неожиданные ошибки — в `500 internal_error` без
  раскрытия внутреннего текста исключения.
- Для HTTP используется цепочка middleware
  `Recover → Tracing → Logging → Metrics → router`.
- Каждый HTTP-запрос логируется один раз после ответа, бизнес-события
  создания и завершения поездки записываются отдельно.

### Запуск лабораторной работы 2

Требуются Go, Docker, `tripgoctl` и `golangci-lint` версии 2.

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
make migrate
make run
```

`tripgoctl connect` создаёт локальный `.env` с адресами PostgreSQL и
OTLP-коллектора. Секреты в репозиторий не добавляются. Перечень переменных
находится в `.env.example`. Для OpenTelemetry используются
`OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`,
`OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_RESOURCE_ATTRIBUTES` и
`OTEL_TRACES_SAMPLER`.

### Генерация кода и проверка

```bash
make generate
make lint
make test
make test-integration
```

- `make generate` обновляет OpenAPI-код и тестовые моки из `contract.go`.
  Сгенерированные моки хранятся рядом с тестами.
- `make lint` запускает `golangci-lint` с `.golangci.yml` преподавателя.
- `make test` запускает unit-тесты с `-race` и печатает покрытие.
- `make test-integration` дополнительно запускает тесты PostgreSQL,
  помеченные `//go:build integration`; нужен запущенный PostgreSQL и `.env`.
- В домене и во всех трёх usecase покрытие превышает требуемые 50%.

### Наблюдаемость

Сервис отправляет трейсы и метрики через OTLP-коллектор. Трейсы просматриваются
в Jaeger, метрики — в Prometheus/Grafana; актуальные адреса выдаёт
`tripgoctl connect`.

В каждом логе запроса присутствуют `trace_id` и `span_id`, позволяющие найти
соответствующий трейс в Jaeger. HTTP-спаны содержат вложенные операции
PostgreSQL (`BEGIN`, `INSERT`, `SELECT`, `UPDATE`, `COMMIT`). Метки маршрутов
содержат шаблон (`/api/v1/trips/{tripId}`), а не конкретный UUID.

Обязательные метрики лабораторной работы 2:

| Метрика | Назначение |
|---|---|
| `http.server.request.duration` | Гистограмма времени HTTP-ответов |
| `http.server.active_requests` | Количество текущих HTTP-запросов |
| `db.client.operation.duration` | Гистограмма длительности запросов к PostgreSQL |
| `tripgo.trips.created` | Счётчик созданных поездок |
| `tripgo.trips.completed` | Счётчик завершённых поездок |

Скриншоты трассы HTTP + PostgreSQL из Jaeger и метрик из Grafana
прикладываются **в описание Pull Request** `homework/02` → `main` своего форка.
