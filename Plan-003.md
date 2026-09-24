# Plan-003: Фаза 3 — Mikrotik и OpenWRT (гетерогенные ноды)

> Родительский документ: [Plan-001.md](./Plan-001.md), раздел «🟠 Фаза 3».
> Предусловие: ✅ Фаза 2 завершена (Plan-002 AC1–AC6). В коде Фазы 1 уже подготовлены:
> интерфейс `drivers.Driver` (`ApplySpec/RemoveConfig/GetStatus`) и фабрика `drivers.New()`;
> заглушки `MikrotikDriver` / `OpenWRtDriver` в `internal/drivers/stubs.go`, которые эта фаза реализует.

**Цель фазы:** нода типа `mikrotik` и `openwrt` полноценно участвует в mesh:
настройка WG-интерфейса, peers, маршрутизация/NAT, статус, откат при ошибке.
Можно построить маршрут `client → mikrotik-home → kz(exit)` и `client → openwrt → de`.

**Длительность:** 2–3 недели.

---

## 0. Что уже есть и что меняем

| Элемент | Текущее состояние | Изменение в Фазе 3 |
|---|---|---|
| `drivers.Driver` | ✅ интерфейс из Фазы 1 | расширяем: контекст + ошибки типизации (§1) |
| `drivers/stubs.go` | возвращают «будет в Фазе 3» | удаляем, переносим в `mikrotik.go`, `openwrt.go` |
| `config.Node.SSH*` | user/key/pass/port | += `ssh_enable` (RouterOS), поля платформы (§2.1) |
| apply pipeline (`cli/apply.go`) | строит wg-quick конфиг для linux | driver-специфичная сериализация через `NodeApplySpec` (§1.3) |
| TUI | видит только linux-статусы | значки платформ уже есть (Фазы 2), добавляем detail-панель платформы (§5) |

---

## 1. Универсальный интерфейс Driver (P0)

### 1.1 Переработка контракта

```go
type Driver interface {
    Name() string
    // Apply — идемпотентно приводит ноду к desired-состоянию спека.
    // Обязана поддерживать partial-failure: либо полное применение, либо откат (§4).
    Apply(ctx context.Context, spec *NodeApplySpec) error
    // Remove — удалить WG-интерфейс и связанные правила (uninstall-безопасно).
    Remove(ctx context.Context, node *config.Node) error
    // Status — машиночитаемое состояние (для CLI/TUI/мониторинга Фазы 4).
    Status(ctx context.Context, node *config.Node) (*NodeStatus, error)
    // Preflight — проверка готовности платформы до боевого apply (пакеты, права, версии).
    Preflight(ctx context.Context, node *config.Node) error
}

type NodeStatus struct {
    Online     bool
    Interface  string
    Peers      []PeerStatus // PublicKey, Endpoint, AllowedIPs, LastHandshake, Rx/Tx
    Extra      map[string]string // платформенные поля (uptime, fw version...)
}

type PeerStatus struct {
    PublicKey     string
    Endpoint      string
    LastHandshake time.Time
    RxBytes, TxBytes uint64
}
```

Миграция: `ApplySpec→Apply`, `GetStatus→Status` — переименование затрагивает
`linux.go`, `cli/apply.go`, TUI. Linux-драйвер первым переводится на новый контракт
(регрессии покрываются существующими тестами + новые golden-тесты статуса).

### 1.2 Desired-state вместо текстового конфига

Сейчас `NodeApplySpec.Config` — это `*wg.NodeConfig` (текст wg-quick). Для RouterOS/UCI
текстовый формат не годится. Решение: `NodeApplySpec` уже содержит всё семантически
(узлы, пиры, NAT/Forward флаги); каждый драйвер сам сериализует desired-state:

- linux → wg-quick `.conf` (как сейчас, без изменений в выводе);
- mikrotik → последовательность RouterOS-команд;
- openwrt → UCI-директивы + `/etc/config/wireguard`.

WG-параметры уровня peer (`persistent_keepalive`, preshared key) уже есть в модели —
сопоставить с возможностями платформ (§2.2, §3.2).

### 1.3 Реестр возможностей (capability matrix)

Новый файл `internal/drivers/capabilities.go`:

```go
type Caps struct {
    SupportsPSK        bool
    SupportsKeepalive  bool
    SupportsNAT        bool   // masquerade из коробки
    SupportsMultiRoute bool   // несколько exit-маршрутов на одной ноде
    NeedsPackageInstall bool  // opkg install ...
}
func Capabilities(platform string) Caps
```

Используется: валидатором (П-проверки ниже), CLI (`meshctl node capabilities <name>`),
TUI (скрытие недоступных опций формы ноды).

Валидация (добавка в `mesh/validator.go`): если нода `mikrotik` — предупредить об
отсутствии PSK на старых прошивках (<6.43 — нет wireguard вообще), проверить, что
`listen_port` ≠ default RouterOS Winbox порт и т.п.

---

## 2. Mikrotik driver (P0)

### 2.1 Транспорт и аутентификация

- Подключение: SSH (`golang.org/x/crypto/ssh`) к `host:ssh_port` (RouterOS: `ssh admin@host -p 22`,
  shell-режим RouterOS CLI). Пароль — основной метод; ключи поддерживаются RouterOS ≥6.x (`/user ssh-public-keys`).
- Новый пакет `internal/drivers/routeros/`:
  - `client.go` — `Run(cmd string) (Output, error)`: отправка команд, парсинг ответов,
    обработка `[syntax error]`, `[failure...]`; таймауты через ctx.
  - `parser.go` — разбор табличного вывода (`print as-table`, `detail-forward`) в `[]map[string]string`.
  - Токены безопасности: никогда не логировать пароли; секреты (private key) передавать
    через stdin-heredoc аналогично linux `write`, не через argv.
- Config-поля микротика (в `config.Node`, yaml `mikrotik:` блок, опционально):
  ```yaml
  mikrotik:
    wan_interface: "ether1"    # Явное указание WAN-интерфейса (если автоопределение по default route неподходящее)
    winbox_port_check: true   # перед apply убедиться, что API/SSH включены
    comment_prefix: "meshctl" # комментарии в правилах для идентификации
  ```

### 2.2 Алгоритм Apply (порядок команд)

Эталон для RouterOS ≥ 7 (wireguard встроен с v7.1; на v6.43+ нужен пакет — Preflight определяет версию):

1. `/interface wireguard add name=wg0 private-key=<key> listen-port=<port> comment="meshctl:<node>"`
   — если существует: `set` приватника/порта (idempotent upsert по имени интерфейса).
2. `/interface wireguard peers add public-key=... allowed-address=... endpoint-address=... endpoint-port=... persistent-keepalive=...`
   — upsert по `public-key`; лишние peers (не в desired-state, но с комментарием `meshctl:*`) — удалить.
   Чужие (ручные) peers не трогаем — фильтр по комментарию.
3. Адрес: `/ip address add address=<mesh_ip>/24 interface=wg0` (upsert).
4. Если `Forward`: убедиться `routing table main` ок; если `NAT` (exit):
   `/ip firewall nat add chain=srcnat out-interface=<wan> action=masquerade comment="meshctl-nat"`
   где WAN определяется либо по полю `wan_interface`, либо автоматически по интерфейсу default route (`/ip route print where dst-address=0.0.0.0/0`).
5. **Селективная маршрутизация по доменам/IP (Domain Split Tunneling)**:
   При наличии правил `match: { list: ... }` на ноде Mikrotik:
   - `/ip dns static add name=youtube.com match-subdomain=yes address-list=list-youtube-via-de` (динамическое автозаполнение IPSet при резолве доменов).
   - `/ip firewall mangle add chain=prerouting dst-address-list=list-youtube-via-de action=mark-routing new-routing-mark=to-de-exit comment="meshctl-route"`
   - `/ip route add dst-address=0.0.0.0/0 gateway=wg-via-de routing-table=to-de-exit`
6. Проверка: `/ping <peer_mesh_ip>` до непосредственного соседа (если сосед уже настроен),
   `/interface wireguard print stats` — handshake появился в течение 15 s.

Откат (§4) отключает созданные объекты по комментарий-префиксу `meshctl:`.

### 2.3 Статус и Remove

- `Status`: `/interface wireguard print detail`, `/interface wireguard peers print detail` →
  маппинг в `NodeStatus` (`last-handshake`, `rx`, `tx`).
- `Remove`: удалить peers с комментом `meshctl:`, интерфейс, адрес, NAT/filter-правила
  с комментами `meshctl-*`. Идемпотентно: отсутствие объекта = успех.

### 2.4 Тесты Mikrotik

- unit: parser таблиц (golden fixtures ответов RouterOS), генератор команд (desired-state →
  ожидаемый список команд, включая upsert/delete-лишнего), каппилити.
- интеграция: эмулятор не нужен — чистовая (smoke) проверка на реальном CRS/Hex через env-gated test
  (`//go:build mikrotik_e2e`, переменные `MT_HOST/MT_USER/MT_PASS`), в CI не запускается,
  запускается вручную перед релизом (чек-лист релиза).

---

## 3. OpenWRT driver (P1)

### 3.1 Транспорт

SSH (dropbear) + UCI. Новый пакет `internal/drivers/openwrt/`:
- `runner.go` — переиспользует общий `internal/sshx` (из Фазы 2) — то же known_hosts/TOFU.
- Все изменения — через `uci set/commit`, НЕ редактированием файлов напрямую;
  исключение: `/etc/config/wireguard` тоже пишется UCI-командами.

### 3.2 Алгоритм Apply

1. Preflight: `opkg list-installed | grep kmod-wireguard luci-proto-wireguard` ;
   при отсутствии — `opkg update && opkg install wireguard-tools kmod-wireguard`
   (флаг конфига `auto_install: true`, по умолчанию false — предупреждать).
2. desired-state → UCI:
   ```
   uci set network.mesh_wg=interface
   uci set network.mesh_wg.proto='wireguard'
   uci set network.mesh_wg.private_key='...'
   uci set network.mesh_wg.listen_port='51820'
   uci set network.mesh_wg.addresses='<mesh_ip>/24'
   uci add_wireguard peer ...(public_key, allowed_ips, endpoint_host/port, persistent_keepalive)
   uci commit network
   ```
   Upsert-стратегия: сначала собрать текущий профиль `uci show network`, diff с desired,
   применить минимум операций; лишние meshctl-пиры удалить.
3. `ifdown mesh_wg; ifup mesh_wg` (только при изменениях — см. M4-идемпотентность из Фазы 2).
4. Forward/NAT: firewall zone через UCI (`firewall.mesh` zone + forwarding rules),
   `masq=1` на exit. Откат по метке option `comment`/отдельному сегрегации списку.
5. Проверка handshake: `ubus call network.interface.mesh_wg status` (JSON) → `NodeStatus`.

### 3.3 Особенности/риски

- Dropbear: меньше функций (нет SFTP) — файлы писать heredoc'ом (паттерн из linux.go).
- Малая RAM/flash: не тащить агентов, только UCI/ubus.
- Прошивка без `luci-proto-wireguard` → Preflight даёт внятную ошибку со ссылкой на команду установки.

---

## 4. Транзакционность и откат (P0)

Общий механизм в `internal/drivers/rollback.go`:

```go
type Step struct {
    Name    string
    Do      func(ctx) error
    Undo    func(ctx) error // nil => шаг необратим, но безопасен (read-only)
}
func RunSteps(steps []Step) error // выполняет, при ошибке — Undo обратном порядке, ошибка-обёртка ApplyError{FailedStep, RollbackErr}
```

- Linux-драйвер тоже переводится на Steps (wg-quick down при неудаче проверки).
- CLI `apply` печатает пошаговый лог: `✔ wg-interface`, `✔ peers (3)`, `✘ nat: … → откат выполнен`.
- Команда `meshctl node teardown <name>` — публичный вызов `Remove` (быстро: сегодня удаления с ноды вообще нет).

---

## 5. Интеграция с CLI/TUI (P1)

- `meshctl node add --type mikrotik --host ... --user admin [--password|--ssh-key]` —
  валидатор требует credentials; подсказка про включённый SSH в RouterOS.
- `meshctl status <node>` (новое, базовая версия; полный мониторинг — Фаза 4):
  таблица нод: platform / online / peers handshaked / last handshake. Использует `Driver.Status`.
- `meshctl node teardown <name> [--yes]`.
- TUI: detail-панель ноды показывает `Extra` (версия RouterOS/OpenWRT), кнопка `r` — refresh статуса
  через `Driver.Status` (goroutine, ctx-timeout); для недоступных платформ — иконка ✖ вместо crash.
- apply pipeline: параллелизация по нодам (errgroup, max 4 одновременных SSH), порядок —
  сначала «дальние» от клиента hops, потом ближние (exit первым: чтобы relay было с кем договариваться).

---

## 6. Порядок работ

| Неделя | Задачи |
|---|---|
| 1 | §1 контракт + миграция linux + capability matrix + `status`/`teardown` CLI; rollback framework |
| 2 | §2 Mikrotik: routeros client/parser/generator + unit-тесты; e2e на железке |
| 3 | §3 OpenWRT + интеграция TUI (§5), параллельный apply, документация, релиз-чек-лист |

Мерж-гейты: после недели 1 — все зелёные тесты linux на новом контракте;
после недели 2 — mikrotik-e2e пройден вручную (приложена консоль в PR).

---

## 7. Acceptance Criteria

- [ ] AC1: `client → mikrotik-home → kz(exit)` работает: exit IP = kz, микротик виден в `meshctl status`.
- [ ] AC2: `client → openwrt-router → de(exit)` — аналогично (или зачёт с пометкой «нет железа»,
      если OpenWRT отложен как «опционально» в Plan-001 — тогда минимально: unit-покрытие генератора UCI).
- [ ] AC3: Повторный apply на mikrotik/openwrt идемпотентен (diff = пусто, handshake не сбрасывается).
- [ ] AC4: Сбой на шаге NAT → предыдущие шаги откачены, на ноде не осталось «полуконфига»
      (проверяется e2e-тестом с намеренно битым правилом).
- [ ] AC5: Чужие (не meshctl) интерфейсы/peers/firewall-правила не модифицируются —
      доказательство: unit-тест upsert с «ручным» peer в fixtures.
- [ ] AC6: Coverage `internal/drivers/*` ≥ 70% (без учёта env-gated e2e); README обновлён.

## 8. Риски

| Риск | Митигация |
|---|---|
| Разница синтаксиса RouterOS v6/v7 | Preflight читает `/system resource print version`, ветвление генератора; v6 без пакета — понятная ошибка |
| Блокировка себя firewall-правилами на роутере дома | всегда создавать только additive-правила с комментом; `teardown` документируем первым в runbook восстановления; warn при `--host` из приватной подсети |
| Потеря связи посреди apply (Wi-Fi роутер) | шаги мелкие + per-step undo; повторный apply продолжает с desired-state (convergent) |
| Dropbear лимиты сессий | одна SSH-сессия = один batch команд; keepalive |
| Нет железа для тестов | договориться заранее: CRS128/Hex (mikrotik) и GL-AR750S (openwrt) или эмулировать openwrt в docker (openwrt/rootfs image) для e2e |

## 9. Чек-лист выполнения

- [x] §1 Driver v2 + миграция linux + caps
- [x] §4 rollback framework + teardown/status CLI
- [x] §2 routeros пакет (client/parser/generator) + unit
- [x] §2 mikrotik e2e на железе (manual log в PR)
- [x] §3 openwrt пакет + UCI generator + unit
- [x] §5 TUI detail/refresh, параллельный apply
- [x] §7 AC-гейт, README, отметка в Plan-001
