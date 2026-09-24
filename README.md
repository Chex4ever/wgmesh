# meshctl

CLI-менеджер одноранговых (peer-to-peer) WireGuard mesh-сетей с multihop exit-маршрутами.
Один бинарник на Go, YAML-конфиги (версионируются в Git), без центрального сервера.
Архитектура и roadmap — в [Plan-001.md](./Plan-001.md).

## Возможности (Фаза 1 — MVP)

- `meshctl init` — создать `mesh.yaml` с дефолтной конфигурацией;
- `meshctl node add|list|remove` — управление нодами (linux / mikrotik / openwrt);
- `meshctl route add|list|remove` — exit-маршруты: `--path client,nodeA,nodeB --exit nodeB`;
- `meshctl apply [--dry-run|--check]` — генерация WG-ключей и mesh_ip, валидация,
  построение wg-quick конфигов и применение к Linux-нодам по SSH
  (wg-quick + iptables NAT/forwarding);
- `meshctl client-config <route> [--qr]` — клиентский `.conf` + QR-код (PNG).

Mikrotik/OpenWRT драйверы, TUI и мониторинг — в Фазах 2–4 (см. план).

## Сборка и установка

```bash
git clone https://github.com/meshctl/meshctl
cd meshctl
go build -trimpath -ldflags "-s -w -X github.com/meshctl/meshctl/internal/cli.Version=$(git describe --tags --always)" \
  -o meshctl ./cmd/meshctl

# или установщик (кладёт бинарник в /usr/local/bin):
./scripts/install.sh

# или go install
go install github.com/meshctl/meshctl/cmd/meshctl@latest
```

Кросс-компиляция:

```bash
GOOS=linux GOARCH=amd64 go build -o dist/meshctl-linux-amd64 ./cmd/meshctl
GOOS=linux GOARCH=arm64 go build -o dist/meshctl-linux-arm64 ./cmd/meshctl
GOOS=darwin GOARCH=arm64 go build -o dist/meshctl-darwin-arm64 ./cmd/meshctl
```

## Быстрый старт

```bash
meshctl init --name "My Mesh"
meshctl node add kz-server --type linux --host 109.248.198.55 --user root
meshctl node add de-server --type linux --host 194.87.71.7 --user root --wg-port 51821
meshctl route add via-kz    --path client,kz-server --exit kz-server
meshctl route add via-kz-de --path client,kz-server,de-server --exit de-server
meshctl apply --dry-run          # посмотреть план без изменений
meshctl apply                    # настроить ноды по SSH
meshctl client-config via-kz-de --qr
```

Порт `--wg-port` (и/или `--wg-interface`) должен быть уникальным для каждой ноды
на одном хосте — валидатор ловит конфликты `interface:port`.

## Конфигурация

Главный файл — `mesh.yaml` (путь переопределяется флагом `--config`).
Примеры: [`configs/mesh.yaml`](./configs/mesh.yaml) и
[`configs/nodes/kz-server.yaml`](./configs/nodes/kz-server.yaml).

```yaml
name: "My Mesh Network"
version: 1
cidr: 10.66.0.0/24        # mesh-подсеть (нодам раздаются .10+, клиентам — .100+)
nodes:
  - name: kz-server
    type: linux            # linux | mikrotik | openwrt
    host: 109.248.198.55
    ssh_user: root
    ssh_key: ~/.ssh/id_rsa
    wireguard: { interface: wg0, listen_port: 51820 }
routes:
  - name: via-kz-de
    path: [client, kz-server, de-server]
    exit_node: de-server
```

Приватные/публичные ключи и `mesh_ip` заполняются автоматически при `apply`
(генерация на curve25519, без бинарника `wg`). Ключи клиентов кэшируются в
секции `clients:` того же файла командой `client-config`.

## Структура репозитория

```
cmd/meshctl/        точка входа
internal/cli/       команды Cobra (init, node, route, apply, client-config)
internal/config/    YAML-типы, загрузка/сохранение
internal/wg/        генерация ключей, рендеринг wg-quick конфигов
internal/mesh/      менеджер применения (план, ключи, адреса) + валидатор
internal/drivers/   Driver-абстракция; linux (SSH+wg-quick+iptables), заглушки mikrotik/openwrt
configs/            примеры конфигураций
scripts/            install.sh
```

## Разработка

```bash
go build ./... && go vet ./... && go test ./...
```

## Лицензия

MIT — см. [LICENSE](./LICENSE).
