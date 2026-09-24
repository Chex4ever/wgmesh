# wgmesh

CLI/TUI-менеджер одноранговых (peer-to-peer) WireGuard & AmneziaWG mesh-сетей с multihop exit-маршрутами, селективным туннелированием по доменам (Split Tunneling) и мульти-клиентской маршрутизацией.

Один бинарник на Go, YAML-конфиги (версионируются в Git), управление роутерами и VPS по SSH, без центрального сервера (Zero-Server / Zero-Agent).

---

## 🔥 Ключевые особенности

- **Zero-Server & Zero-Agent**: Работает локально, настраивает VPS и роутеры по SSH без установки управляющих серверов и фоновых даемонов.
- **Multihop Chaining**: Произвольные цепочки узлов (`Client → KZ Server → DE Exit Server`).
- **Селективная маршрутизация (Domain/IP Split Tunneling)**: Настройка правил для конкретных сайтов (например, `youtube.com` через Германию, `antigravity.com` через Казахстан).
- **Мульти-клиентская маршрутизация**: Учёт источника трафика (`from: [alice-phone, home-mikrotik]`) — каждый клиент или роутер может иметь свои правила и цепочки выхода.
- **Гетерогенные узлы**: Прямая поддержка **Linux** (wg-quick, iptables/nftables), **Mikrotik RouterOS 7** (`/interface wireguard`, `/ip dns static`, address-lists, mangle) и **OpenWRT** (UCI + fw4).
- **Защита от DPI (AmneziaWG)**: Поддержка заголовков H1-H4, размеров S1-S4 и мусора Jc/Jmin/Jmax на межсерверных звеньях.
- **Гибкий экспорт**: Генерация конфигов и QR-кодов в форматах **WireGuard**, **AmneziaVPN**, **Sing-box JSON** (с автоматически запечёнными доменными правилами) и **Universal URIs** (`sing-box://`, `wireguard://`).
- **Интерактивный TUI**: ASCII-визуализация топологии сети на Bubble Tea / Lipgloss.

---

## 📋 Конфигурация (`mesh.yaml` v2)

Пример декларативной конфигурации с выборочной маршрутизацией:

```yaml
name: "My Mesh Network"
version: 2
cidr: 10.66.0.0/16        # mesh-подсеть

# Ноды сети (Linux VPS, роутеры Mikrotik / OpenWRT)
nodes:
  - name: kz-server
    type: linux            # linux | mikrotik | openwrt
    host: 109.248.198.55
    ssh_user: root
    ssh_key: ~/.ssh/id_rsa
    wireguard: { interface: wg0, listen_port: 51820 }

  - name: de-server
    type: linux
    host: 194.87.71.7
    ssh_user: root
    ssh_key: ~/.ssh/id_rsa
    wireguard: { interface: wg0, listen_port: 51820 }

  - name: home-router
    type: mikrotik
    host: 192.168.1.1
    ssh_user: admin
    mikrotik: { wan_interface: "ether1" }

# Клиенты (устройства пользователей)
clients:
  - name: alice-phone
    ip: 10.66.100.1
    ingress: kz-server

  - name: bob-laptop
    ip: 10.66.100.2
    ingress: de-server

# Списки доменов и IP для выборочного туннелирования
lists:
  - name: youtube-list
    domains:
      - "youtube.com"
      - "*.googlevideo.com"
      - "ytimg.com"

  - name: work-apps
    domains:
      - "antigravity.com"
      - "*.internal.company"
    ips:
      - "195.201.0.0/16"

# Маршруты и правила распределения трафика
routes:
  # YouTube уходит через Германию для Алисы и домашнего роутера
  - name: youtube-via-de
    from: [alice-phone, home-router]
    match:
      list: youtube-list
    path: [kz-server, de-server]
    exit_node: de-server
    link_obfuscation:
      "kz-server->de-server": "ampere" # AmneziaWG на международном плече

  # Рабочие приложения уходят через Казахстан для всех клиентов
  - name: work-via-kz
    from: [alice-phone, bob-laptop, home-router]
    match:
      list: work-apps
    path: [kz-server]
    exit_node: kz-server

  # Дефолтный трафик Алисы
  - name: default-alice
    from: [alice-phone]
    match: default
    path: [kz-server]
    exit_node: kz-server
```

---

## ⚡ Быстрый старт

```bash
# Инициализация нового проекта
wgmesh init --name "My Mesh"

# Добавление нод
wgmesh node add kz-server --type linux --host 109.248.198.55 --user root
wgmesh node add de-server --type linux --host 194.87.71.7 --user root
wgmesh node add home-router --type mikrotik --host 192.168.1.1 --user admin

# Создание маршрута
wgmesh route add via-kz-de --path client,kz-server,de-server --exit de-server

# Проверка плана без изменений
wgmesh apply --dry-run

# Настройка нод по SSH
wgmesh apply

# Экспорт конфигов
wgmesh client-config via-kz-de --qr                           # Стандартный WG + QR
wgmesh export --client alice-phone --format sing-box -o alice.json # Sing-box с доменными правилами
wgmesh export --client alice-phone --format uri                    # 1-click URI ссылка
```

---

## 💻 Интерактивный TUI

Запуск консольного интерфейса:

```bash
wgmesh tui
```

В TUI доступны:
- ASCII-визуализация топологии сети и цепочек хопов;
- Навигация и интерактивное редактирование маршрутов (`+` / `-` / `←` / `→`);
- Просмотр статусов нод и ping/latency;
- Просмотр лога `apply` в реальном времени;
- Экспорт конфигов и просмотр QR-кода (клавиша `x`).

---

## 🛠️ Сборка и установка

```bash
git clone https://github.com/Chex4ever/wgmesh
cd wgmesh
go build -trimpath -ldflags "-s -w -X github.com/meshctl/meshctl/internal/cli.Version=$(git describe --tags --always)" \
  -o wgmesh ./cmd/wgmesh

# Кросс-компиляция:
GOOS=linux GOARCH=amd64 go build -o dist/wgmesh-linux-amd64 ./cmd/wgmesh
GOOS=linux GOARCH=arm64 go build -o dist/wgmesh-linux-arm64 ./cmd/wgmesh
GOOS=darwin GOARCH=arm64 go build -o dist/wgmesh-darwin-arm64 ./cmd/wgmesh
```

---

## 📁 Структура репозитория

```
cmd/wgmesh/         Точка входа
internal/cli/       Команды Cobra (init, node, route, apply, client-config, export, git)
internal/config/    YAML-типы (Mesh, Node, Route, Client, DomainList, Obfuscation)
internal/tui/       Интерфейс Bubble Tea (топология, редактор, монитор)
internal/wg/        Генерация ключей, рендеринг WireGuard / AmneziaWG
internal/export/    Реестр экспорта (WireGuard, Amnezia, Sing-box JSON, URI, QR)
internal/mesh/      Ядро применения (план, ключи, валидация, policy routing)
internal/drivers/   Драйверы платформ (Linux, Mikrotik RouterOS 7, OpenWRT)
configs/            Примеры конфигураций
scripts/            install.sh
```

---

## 📜 Лицензия

MIT — см. [LICENSE](./LICENSE).
