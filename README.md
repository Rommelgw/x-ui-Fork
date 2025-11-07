[English](/README.md) | [فارسی](/README.fa_IR.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
    <img alt="VPN Master Panel" src="./media/01-overview-light.png" width="720">
  </picture>
</p>

# VPN Master Panel

**Централизованная система управления Xray-нодами с мультиподписками.**

Этот форк x-ui переработан в распределённую платформу, где мастер-панель управляет множеством VPN-нод через защищённые агенты. Проект ориентирован на автоматизацию развёртывания Xray-core, консолидацию конфигураций и выдачу мультиформатных подписок пользователям.

## Возможности

- **Центральная панель (Go + Gin)** — API для агентов, администраторов и подписок, автоматические миграции БД, мониторинг состояния.
- **Node Agent (Go)** — без вмешательства устанавливает/обновляет Xray-core, применяет конфиги от мастера, отправляет статистику и телеметрию.
- **Мультиподписки** — генерация конфигов в форматах JSON, Clash, V2Ray, Shadowrocket с учётом групп, весов и ограничений трафика.
- **Безопасность** — HMAC-подписи всех запросов агент↔мастер, TLS на внешних интерфейсах, хранение секретов в системных env-файлах.
- **Мониторинг** — heartbeat-чеки, статусы нод (online/offline/degraded), агрегированные метрики для панели администрирования.

Подробное описание архитектуры доступно в `docs/architecture.md`.

## Быстрый старт

### 1. Требования

- Linux x86_64 / arm64 с systemd
- Go 1.23+
- Node.js 18+ (для фронтенда)
- SQLite/MySQL/PostgreSQL для мастера (по умолчанию используется SQLite)

### 2. Установка мастер-панели

```bash
curl -fsSL https://raw.githubusercontent.com/your-org/vpn-master-panel/main/scripts/install-master.sh | bash
```

Опции скрипта:

- `-p <port>` — HTTP-порт (по умолчанию `8085`)
- `-s <secret>` — HMAC-секрет (по умолчанию генерируется автоматически)
- `-d <dir>` — директория установки (по умолчанию `/opt/vpn-master`)
- `-D <dir>` — директория данных (по умолчанию `/var/lib/vpn-master`)

После установки сервис доступен как `vpn-master.service`, переменные окружения хранятся в `/etc/vpn-master.env`.

### 3. Установка агента на ноде

```bash
curl -fsSL https://raw.githubusercontent.com/your-org/vpn-master-panel/main/scripts/install-agent.sh \
  -o install-agent.sh
chmod +x install-agent.sh
sudo ./install-agent.sh \
  -u https://master.example.com \
  -n "EU Node 01" \
  -r <registration-secret>
```

После регистрации мастер выдаст постоянный секрет, агент будет синхронизировать конфигурации и статистику автоматически.

### 4. Сборка фронтенда (опционально)

```bash
cd web/admin
npm install
npm run build       # статика будет собрана в dist/
```

Готовую сборку можно обслуживать любым web-сервером либо интегрировать в панель.

## Конфигурация окружения

| Переменная              | Назначение                                     | По умолчанию              |
|-------------------------|------------------------------------------------|---------------------------|
| `MASTER_HTTP_PORT`      | Порт HTTP API                                  | `8085`                    |
| `MASTER_DB_DRIVER`      | Драйвер БД (`sqlite`, `mysql`, `postgres`)     | `sqlite`                  |
| `MASTER_DB_DSN`         | DSN для подключения к БД                       | `data/master.db`          |
| `MASTER_DB_AUTO_MIGRATE`| Автоматические миграции (bool)                 | `true`                    |
| `MASTER_HMAC_SECRET`    | Секрет для подписи запросов агентов            | **обязателен**            |
| `MASTER_TLS_CERT_FILE`  | Путь к TLS сертификату (опционально)           | —                         |
| `MASTER_TLS_KEY_FILE`   | Путь к приватному ключу (опционально)          | —                         |

Агент использует `/etc/node-agent/config.json`, создаваемый установщиком (`master_url`, `node_id`, `registration_secret`, `secret_key`, `listen_addr` и др.).

## REST API (избранное)

- `POST /api/nodes/register` — регистрация ноды (HMAC `registration_secret`)
- `GET /api/nodes/:id/config` — выдача конфигурации Xray
- `POST /api/nodes/:id/stats` — приём телеметрии
- `GET /api/subscriptions/:client_uuid` — мультиподписка для клиента
- `GET /api/admin/dashboard` — агрегированные метрики
- `GET /api/admin/nodes` — список нод с группами и статусами

Запросы от мастер-панели к агентам подписываются заголовком `X-Master-Signature`, агенты отвечают заголовком `X-Node-Signature`.

## Разработка

```bash
go run ./cmd/master    # запуск мастер-панели в dev-режиме
go run ./cmd/agent     # запуск агента (использует config/json)
```

- `go build ./...` — проверка сборки
- `npm run dev --prefix web/admin` — dev-сервер фронтенда

База SQLite по умолчанию создаётся в `data/master.db`. Для MySQL/PostgreSQL задайте `MASTER_DB_DRIVER` и `MASTER_DB_DSN`.

## Безопасность

- Все запросы агент↔мастер имеют HMAC-подпись.
- Рекомендуется включить TLS на мастер-панели и закрыть административный интерфейс VPN.
- Секреты хранятся в root-owned env-файлах, права ограничены 600.

## Лицензия

Проект распространяется по лицензии GPLv3. Xray-core и другие зависимости имеют собственные лицензии — см. `LICENSE` и файлы в папке `media/`.