# Plan-001: meshctl — менеджер Mesh VPN сетей (WireGuard multihop)

> Рабочие названия: `meshctl` (основное), альтернативы: `wgmesh`, `routemesh`.
> Язык: Go. Один бинарник, YAML-конфиги, TUI + CLI, peer-to-peer без центрального сервера.

---

## 🎯 Философия проекта

**Принципы:**
- Один бинарник, zero dependencies (для конечного пользователя)
- Конфиги в YAML файлах (версионируются в Git)
- TUI для визуализации и управления
- CLI для автоматизации и скриптов
- Нет центрального сервера (peer-to-peer конфигурация)
- Работает на Linux, Mikrotik, OpenWRT

---

## 🏗️ Архитектура (упрощённая)

```
┌─────────────────────────────────────────────┐
│              meshctl binary                 │
├─────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │   CLI    │  │   TUI    │  │  Config  │  │
│  │ Commands │  │ (Bubble  │  │  Loader  │  │
│  │          │  │   Tea)   │  │  (YAML)  │  │
│  └──────────┘  └──────────┘  └──────────┘  │
│         │             │             │        │
│         └─────────────┼─────────────┘        │
│                       │                      │
│              ┌────────▼────────┐             │
│              │  Mesh Manager   │             │
│              │  (Core Logic)   │             │
│              └─────────────────┘             │
│                       │                      │
│         ┌─────────────┼─────────────┐        │
│         │             │             │        │
│    ┌────▼────┐   ┌────▼────┐   ┌───▼────┐   │
│    │  Linux  │   │Mikrotik │   │  WG    │   │
│    │ Driver  │   │ (SSH)   │   │ Config │   │
│    └─────────┘   └─────────┘   └────────┘   │
└─────────────────────────────────────────────┘
```

**Ключевые компоненты:**
- **CLI** — команды для скриптов и автоматизации
- **TUI** — интерактивный интерфейс для визуализации и drag-and-drop
- **Config Loader** — читает YAML конфиги (ноды, маршруты, ключи)
- **Mesh Manager** — логика применения конфигов к нодам
- **Drivers** — абстракции для Linux, Mikrotik, OpenWRT

---

## 🛠️ Стек технологий

| Компонент | Технология | Почему |
|---|---|---|
| Язык | Go | Один бинарник, кросс-компиляция, отличная работа с сетью |
| TUI | Bubble Tea | Популярная, красивая, много примеров |
| CLI | Cobra | Стандарт для Go CLI приложений |
| Конфиги | YAML (`gopkg.in/yaml.v3`) | Человекочитаемый, версионируется в Git |
| SSH для Mikrotik | `golang.org/x/crypto/ssh` | Стандартная библиотека |
| WireGuard | `wg` CLI + `wg-quick` | Не изобретаем велосипед |
| Графы (для TUI) | tui-components + кастомный рендерер | Простая ASCII-визуализация |

---

## 📁 Структура проекта

```
meshctl/
├── cmd/
│   ├── meshctl/           # Main binary
│   │   └── main.go
│   └── meshctl-tui/       # TUI binary (опционально)
│       └── main.go
├── internal/
│   ├── cli/               # CLI команды
│   │   ├── init.go        # meshctl init
│   │   ├── node.go        # meshctl node add/list/remove
│   │   ├── route.go       # meshctl route add/edit
│   │   └── apply.go       # meshctl apply
│   ├── tui/               # TUI интерфейс
│   │   ├── app.go         # Главное приложение
│   │   ├── topology.go    # Визуализация топологии
│   │   └── editor.go      # Редактор маршрутов
│   ├── config/            # Работа с YAML конфигами
│   │   ├── loader.go
│   │   └── types.go       # Node, Route, Mesh структуры
│   ├── mesh/              # Core логика
│   │   ├── manager.go     # Применение конфигов
│   │   └── validator.go   # Валидация маршрутов
│   └── drivers/           # Драйверы для разных платформ
│       ├── linux.go
│       ├── mikrotik.go
│       └── openwrt.go
├── configs/               # Примеры конфигов
│   ├── mesh.yaml
│   └── nodes/
│       ├── kz-server.yaml
│       └── de-server.yaml
├── scripts/               # Helper скрипты
│   └── install.sh
└── README.md
```

---

## 📋 Конфигурация (YAML)

### mesh.yaml — главный файл

```yaml
name: "My Mesh Network"
version: 1

nodes:
  - name: "kz-server"
    type: "linux"
    host: "109.248.198.55"
    ssh_user: "root"
    ssh_key: "~/.ssh/id_rsa"
    wireguard:
      interface: "wg0"
      listen_port: 51820
      private_key: "..."  # Генерируется автоматически

  - name: "de-server"
    type: "linux"
    host: "194.87.71.7"
    ssh_user: "root"
    ssh_key: "~/.ssh/id_rsa"
    wireguard:
      interface: "wg0"
      listen_port: 51820

  - name: "home-router"
    type: "mikrotik"
    host: "192.168.1.1"
    ssh_user: "admin"
    ssh_password: "..."  # Или ssh_key
    wireguard:
      interface: "wg-home"
      listen_port: 51821

routes:
  - name: "via-kz"
    path: ["client", "kz-server"]
    exit_node: "kz-server"

  - name: "via-kz-de"
    path: ["client", "kz-server", "de-server"]
    exit_node: "de-server"

  - name: "via-de-kz"
    path: ["client", "de-server", "kz-server"]
    exit_node: "kz-server"
```

### nodes/kz-server.yaml — детали ноды (опционально)

```yaml
name: "kz-server"
type: "linux"
host: "109.248.198.55"
ssh_user: "root"
ssh_key: "~/.ssh/id_rsa"

wireguard:
  interface: "wg0"
  listen_port: 51820
  private_key: "GENERATED"  # Заполняется автоматически
  public_key: "GENERATED"

peers: []  # Заполняется автоматически при apply
```

---

## 🚀 Roadmap по фазам

### 🟢 Фаза 1 — MVP (1-2 недели)

**Цель:** работает маршрут Client → Node A → Internet

**CLI команды:**
- `meshctl init` — создаёт mesh.yaml с дефолтной конфигурацией
- `meshctl node add <name> --type linux --host <ip> --user <user>` — добавляет ноду
- `meshctl node list` — показывает все ноды
- `meshctl route add <name> --path <node1,node2,...> --exit <node>` — создаёт маршрут
- `meshctl apply` — применяет все конфиги (генерирует WG ключи, настраивает ноды)
- `meshctl client-config <route>` — генерирует WG конфиг для клиента (QR-код)

**Linux driver:**
- Подключение по SSH
- Установка WireGuard (если не установлен)
- Генерация и применение WG конфигов
- Настройка iptables (NAT, forwarding)

**Конфиги:**
- YAML структура для mesh, nodes, routes
- Автоматическая генерация WG ключей
- Валидация конфигов

**Результат:** можно добавить 2 Linux сервера, создать маршрут, применить, получить клиентский конфиг.

### 🟡 Фаза 2 — Multihop и TUI (2-3 недели)

**Цель:** можно строить цепочки и визуализировать их

**Multihop логика:**
- Поддержка маршрутов с несколькими хопами
- Автоматическая настройка forwarding на промежуточных нодах
- Правильная маршрутизация (AllowedIPs, routes)

**TUI интерфейс:**
- `meshctl tui` — запускает интерактивный режим
- ASCII-визуализация топологии (ноды как кружки, связи как линии)
- Навигация стрелками, выбор нод/маршрутов
- Редактирование маршрутов (добавить/удалить хоп)
- Применение изменений (кнопка "Apply")

**Валидация:**
- Проверка циклов в маршрутах
- Проверка достижимости всех нод
- Проверка конфликтов портов/подсетей

**Результат:** можно визуально видеть топологию, drag-and-drop маршруты, применять изменения.

### 🟠 Фаза 3 — Mikrotik и OpenWRT (2-3 недели)

**Цель:** поддержка гетерогенных нод

**Mikrotik driver:**
- Подключение по SSH (RouterOS CLI)
- Создание WG интерфейса
- Добавление peers
- Настройка маршрутов (`/ip route`)
- Обработка ошибок и откат

**OpenWRT driver (опционально):**
- Подключение по SSH
- Установка WireGuard пакета
- Настройка `/etc/config/wireguard`
- Перезапуск сети

**Универсальный интерфейс:**
- Абстракция `Driver` с методами `ApplyConfig()`, `RemoveConfig()`, `GetStatus()`
- Каждый драйвер реализует этот интерфейс

**Результат:** можно добавить Mikrotik в сеть, построить маршрут через него.

### 🔴 Фаза 4 — Продвинутые фичи (по желанию)

**Git sync:**
- Автоматический коммит изменений в Git
- Pull/Push для синхронизации между участниками
- История изменений (кто и когда поменял маршрут)

**Мониторинг:**
- `meshctl status` — показывает состояние всех нод (online/offline, latency)
- Ping-mesh для проверки связности
- Алерты при падении ноды

**Обфускация (AmneziaWG):**
- Опция `--obfuscate` при создании маршрута
- Использование AmneziaWG вместо WireGuard
- Генерация конфигов с параметрами обфускации

**Экспорт конфигов:**
- `meshctl export <route> --format amnezia` — экспорт в формат AmneziaVPN
- `meshctl export <route> --format wireguard` — стандартный WG конфиг

---

## 💻 Примеры использования

### CLI

```bash
# Инициализация
meshctl init

# Добавление нод
meshctl node add kz-server --type linux --host 109.248.198.55 --user root
meshctl node add de-server --type linux --host 194.87.71.7 --user root
meshctl node add home-router --type mikrotik --host 192.168.1.1 --user admin

# Создание маршрутов
meshctl route add via-kz --path client,kz-server --exit kz-server
meshctl route add via-kz-de --path client,kz-server,de-server --exit de-server

# Применение
meshctl apply

# Получение клиентского конфига
meshctl client-config via-kz-de --qr
```

### TUI

```bash
meshctl tui
```

Откроется интерактивный интерфейс:

```
┌─────────────────────────────────────────────────────────┐
│  My Mesh Network                                        │
├─────────────────────────────────────────────────────────┤
│                                                         │
│    [Client] ──→ [KZ Server] ──→ [DE Server] ──→ 🌐     │
│                                                         │
│    [Client] ──→ [DE Server] ──→ [KZ Server] ──→ 🌐     │
│                                                         │
│    [Client] ──→ [Home Router] ──→ [KZ Server] ──→ 🌐   │
│                                                         │
├─────────────────────────────────────────────────────────┤
│  Nodes: 3 online, 0 offline                             │
│  Routes: 3 active                                       │
│                                                         │
│  [↑↓] Navigate  [Enter] Edit  [A] Apply  [Q] Quit      │
└─────────────────────────────────────────────────────────┘
```

---

## 🎯 Как это решает исходные задачи

| Задача | Решение |
|---|---|
| Визуально видеть серверы | TUI с ASCII-визуализацией топологии |
| Добавлять/исключать/перемещать ноды | CLI команды + TUI редактор |
| Мульти-хоп маршруты | YAML конфиги с `path: [A, B, C]` |
| Поддержка Mikrotik | Mikrotik driver (SSH + RouterOS CLI) |
| Коллаборация | Git sync (конфиги в Git репозитории) |
| Простота | Один бинарник, YAML конфиги, минимум зависимостей |

---

## 📦 Установка и распространение

```bash
# Установка из исходников
git clone https://github.com/yourusername/meshctl
cd meshctl
go build -o meshctl ./cmd/meshctl

# Или через go install
go install github.com/yourusername/meshctl/cmd/meshctl@latest

# Кросс-компиляция для разных платформ
GOOS=linux GOARCH=amd64 go build -o meshctl-linux-amd64
GOOS=linux GOARCH=arm64 go build -o meshctl-linux-arm64
GOOS=darwin GOARCH=amd64 go build -o meshctl-darwin-amd64
```

---

## ✅ Статус выполнения

См. историю коммитов Git и чек-листы ниже (обновляется по ходу работ).

- [ ] Фаза 1: скелет проекта (go.mod, структура каталогов)
- [ ] Фаза 1: config types + loader + save
- [ ] Фаза 1: WG keygen + валидация (validator)
- [ ] Фаза 1: CLI: init / node add|list|remove / route add|list|remove
- [ ] Фаза 1: client-config (генерация .conf + QR)
- [ ] Фаза 1: apply (dry-run + план) + mesh manager
- [ ] Фаза 1: Linux driver (SSH, wg-quick, iptables)
- [ ] Тесты, README, примеры конфигов, install.sh
