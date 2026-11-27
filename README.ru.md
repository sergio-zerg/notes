[English](README.md) | **Русский**

<p align="center"><img src="docs/cover.png" alt="notes" width="100%"></p>

# 📝 Notes

[![CI](https://github.com/sergio-zerg/notes/actions/workflows/ci.yml/badge.svg)](https://github.com/sergio-zerg/notes/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-yellow.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.26.0-00ADD8?logo=go&logoColor=white)
![Flutter](https://img.shields.io/badge/Flutter-Dart-02569B?logo=flutter&logoColor=white)

Небольшое full-stack приложение для **заметок** — бэкенд на Go и клиент на Flutter.
Учебный проект, чтобы отработать чистую архитектуру, два транспорта
**REST + gRPC** и поставку как в продакшене (Docker, миграции, авторизация,
лимит запросов, наблюдаемость, CI).

![Клиент на Flutter — список заметок](docs/screenshot-list.png)

## Устройство репозитория

Монорепозиторий из двух независимых частей:

| Путь | Что это |
|------|---------|
| [`backend/`](backend/README.md) | API на Go (чистая архитектура + DDD). CRUD заметок **и по REST, и по gRPC**, хранение в PostgreSQL, вход по API-ключу, лимит запросов на клиента, трассировка и метрики OpenTelemetry. |
| [`frontend/`](frontend/README.md) | Клиент на Flutter (веб / iOS / Android): вход по API-ключу, поиск, постраничный вывод, создание и правка заметок. |

## Стек

- **Бэкенд:** Go · gorilla/mux (HTTP) · gRPC · pgx + миграции goose · PostgreSQL · OpenTelemetry · лимит запросов по алгоритму token bucket
- **Клиент:** Flutter / Dart
- **Инфраструктура:** Docker и Docker Compose · CI на GitHub Actions

## Быстрый старт

Весь стек (PostgreSQL + API + веб-клиент на Flutter) поднимается одной командой:

```bash
./up.sh                 # собрать при необходимости и запустить всё
./up.sh status          # что запущено и на каких портах
./up.sh down            # остановить, база сохраняется
```

После запуска:

| Сервис | Адрес |
|--------|-------|
| Веб-клиент | http://localhost:8081 |
| REST API | http://localhost:8080 — gRPC на `:50051` |
| Метрики | http://localhost:8080/metrics |

Войдите с API-ключом по умолчанию из compose — **`dev-secret-key-change-me`** (для реального запуска задайте свой `API_KEYS`).

Запуск на машине без Docker — в [`backend/README.md`](backend/README.md).

## Авторизация и настройки

API **не запускается без `API_KEYS`**, если не задан `ALLOW_INSECURE_AUTH=true`
(только для локальных опытов), поэтому развёрнутый сервис не может молча
подняться без авторизации. Скопируйте шаблон и отредактируйте:

```bash
cp backend/.env.example backend/.env   # .env в .gitignore — настоящие секреты не попадают в git
```

Все настройки (база данных, CORS, лимиты запросов, трассировка, остановка)
описаны прямо в [`backend/.env.example`](backend/.env.example).

## Что отрабатывает проект

- **Два транспорта** — одно ядро приложения обслуживает и REST, и gRPC.
- **Чистая архитектура / DDD** — зависимости направлены внутрь; домен ничего не знает об HTTP и SQL.
- **Аккуратная поставка** — встроенные миграции, плавная остановка, вход по API-ключу, лимит запросов на клиента, ограничение размера тела, CORS.
- **Наблюдаемость** — трассировка и метрики OpenTelemetry, структурированные логи.
- **Тесты и CI** — модульные и интеграционные тесты на всех слоях, пайплайн GitHub Actions.

## Скриншоты

| Новая заметка | Пустой список | Светлая тема |
|---|---|---|
| ![Новая заметка](docs/screenshot-editor.png) | ![Пустой список](docs/screenshot-empty.png) | ![Светлая тема](docs/screenshot-list-light.png) |

## Статус

Учебный проект — упор на архитектуру и аккуратную поставку (слои, тесты, CI,
наблюдаемость), а не на количество функций. Что готово и что дальше —
[роадмап](ROADMAP.ru.md).

## Лицензия

[MIT](LICENSE) © sergio-zerg
