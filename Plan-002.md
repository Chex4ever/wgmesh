# Plan-002: Фаза 2 — Multihop и TUI

> Родительский документ: [Plan-001.md](./Plan-001.md), раздел «🟡 Фаза 2».
> Статус Фазы 1: ✅ завершена (см. чек-лист в Plan-001.md). Текущее состояние кода проверено:
> multihop-план строится (`mesh.BuildPlan`), но AllowedIPs на relay-хопах неполные;
> TUI отсутствует; цикл валидации `path` есть, достижимость проверяется только TCP-dial'ом.

**Цель фазы:** надёжные цепочки Client → A → B → …→ Exit с корректной маршрутизацией
на каждом hop + интерактивная визуализация топологии и редактирование маршрутов в TUI.

**Длительность:** 2–3 недели. **Готовность фазы:** все Acceptance Criteria §7 зелёные.

---

## 0. Контекст: что уже есть (Фаза 1)

| Компонент | Файл | Состояние |
|---|---|---|
| YAML-типы (Mesh/Node/Route/WG) | `internal/config/types.go` | ✅ |
| Loader/Save | `internal/config/loader.go` | ✅ |
| WG keygen, рендер `.conf` | `internal/wg/wg.go` | ✅ |
| Валидация (циклы path, конфликты interface:port, exit_node) | `internal/mesh/validator.go` | ✅ частично (см. §3) |
| План применения (relay/exit/peers) | `internal/mesh/manager.go: BuildPlan` | ✅ частично (см. §1) |
| Linux driver (SSH, wg-quick, iptables) | `internal/drivers/linux.go` | ✅ |
| CLI init/node/route/apply/client-config | `internal/cli/*.go` | ✅ |
| TUI | — | ❌ (создаём, §2) |

Рабочие заделки из Фазы 1, которые закрываем в этой фазе:
- `drivers/linux.go`: `HostKeyCallback: ssh.InsecureIgnoreHostKey()` с комментарием `TODO(фаза 2): known_hosts` → §2.5.
- `mesh.AllowedIPsForHop` — недостижимая/неполная функция → заменяется на §1.2.

---

## 1. Multihop: корректная маршрутизация (приоритет P0)

### 1.1 Проблема

Сейчас в `buildNodeSpec` (`internal/cli/apply.go`) AllowedIPs для peer-ноды содержат
`peer.MeshIP/32`, а mesh-подсеть целиком (`m.CIDROrDefault()`) добавляется только если
сосед — exit-нода. Для цепочек длиной > 2 hops это ломает доставку: например, в
`client → kz → de → ru(exit)` узел `kz` не знает, как отправить трафик клиента к `de`
(разрешён только `de/32`, а клиентский destined-пакет идёт на внешние сети через туннельную
цепочку mesh_ip). Нужны транзитные AllowedIPs.

### 1.2 Решение: модель «next-hop = весь mesh»

Правило генерации AllowedIPs для каждого peer'а на каждой ноде:

| Кто peer относительно нас | AllowedIPs |
|---|---|
| Прямой сосед по цепочке, который НЕ является нами exit-последним звеном | `peer.mesh_ip/32` **+ `mesh.cidr`** (транзит) |
| Exit-нода со стороны её непосредственного предшественника | `peer.mesh_ip/32` + `mesh.cidr` + (для клиента) `0.0.0.0/0` |
| Клиент маршрута, где мы — первый hop | `client_ip/32`; если мы также exit (1-hop маршрут) — + `0.0.0.0/0` |
| Клиент маршрута, где мы НЕ первый hop | не добавляем (клиентский peer настраивается только на первом hop) |

Итого упрощённое инвариантное правило: **каждый WG-туннельный peer соседа получает
`mesh_cidr`**, а `0.0.0.0/0` — только на последнем прыжке от того узла, который смотрит
в exit напрямую (у relay это уже покрыто транзитным `mesh_cidr`, т.к. relay переинкапсулирует).

### 1.3 Задачи

- [ ] **M1.** В `internal/mesh/manager.go` реализовать `AllowedIPsForPeer(self *config.Node, peer *config.Node, m *config.Mesh) []string`
      — чистая функция, без I/O. Удалить/переписать мёртвый `AllowedIPsForHop`.
      Таблица соответствия ролей берётся из `BuildPlan` (добавить в `NodePlan` поле
      `NextHops map[string]bool` — соседи, лежащие ближе к exit, чем `self`).
- [ ] **M2.** Перевести `buildNodeSpec` (`internal/cli/apply.go`) на `AllowedIPsForPeer`;
      убрать спец-логику `isExitNode` из cli — она переезжает в mesh-пакет.
- [ ] **M3.** Forwarding на промежуточных нодах: проверить, что `NodeApplySpec.Forward=true`
      ставится всем relay И exit (уже есть) и что linux-driver включает `ip_forward`
      sysctl **персистентно** (`/etc/sysctl.d/99-meshctl.conf` + `sysctl -p`), а не только runtime.
- [ ] **M4.** PostUp/PostDown идемпотентность: при повторном `apply` конфиг ноды должен
      совпадать с предыдущим byte-to-byte (кроме ключей), иначе `wg sync` лишними diff'ами
      рвёт живые сессии. Тест на стабильность рендера (§6 T-M4).
- [ ] **M5.** Маршруты длиной 1 hop (`client → exit`) должны работать без регрессий;
      маршруты 2+ hops — основной кейс (§6 T-M5, интеграция на двух LXC/VM или docker-netns).

### 1.4 Критерии приёмки

- Цепочка `client → A → B → C(exit)`: с клиента `curl ifconfig.me` даёт IP ноды C;
  `traceroute` показывает A, B внутри mesh-подсети.
- Повторный `meshctl apply` не разрывает существующие туннели (handshake не сбрасывается).
- `meshctl apply --dry-run` печатает AllowedIPs каждого peer'а — видно транзит `mesh_cidr`.

---

## 2. TUI: `meshctl tui` (приоритет P0)

### 2.1 Зависимости

```
github.com/charmbracelet/bubbletea   v0.25.x
github.com/charmbracelet/lipgloss    v0.9.x
github.com/charmbracelet/bubbles     v0.17.x  (list, key.help, viewport, spinner)
```

Один бинарник: TUI — подкоманда root-команды (`internal/cli/tui.go`), отдельный
`cmd/meshctl-tui` не заводим (конверсия решения: в Plan-001 он был «опциональным»).

### 2.2 Структура пакетов

```
internal/tui/
├── app.go        # Model/Init/Update/View, корневой layout, таблица фокусов
├── topology.go   # ASCII-граф: chain-layout, box-ноды, стрелки, подсветка выбранного маршрута
├── nodes.go      # панель списка нод (n/N — add/remove, d — detail)
├── routes.go     # панель маршрутов: enter — редактор hop'ов
├── editor.go     # пошаговый редактор path: +/- хоп, перемещение хопа (←/→), выбор exit
├── statusbar.go  # Nodes: X online/Y total, Routes: N, имя конфига, unsaved (*)
└── keys.go       # единая карта хоткеев + help overlay (?)
```

### 2.3 Макет экрана

```
┌─ My Mesh Network ──────────────────────── mesh.yaml ─ [unsaved] ─┐
│                                                                  │
│   ╭────────╮     ╭──────────╮     ╭──────────╮     ╭───╮         │
│   │ Client │ ──▶ │ kz-server│ ──▶ │ de-server│ ──▶ │ 🌐│  via-kz-de
│   ╰────────╯     ╰──────────╯     ╰──────────╯     ╰───╯         │
│                                                                  │
│   ╭────────╮     ╭──────────╮                    ╭───╮            │
│   │ Client │ ──▶ │ home-router(Mikrotik)│ ──────▶ │ 🌐│  via-home-kz
│   ...                                                            │
├──────────────────────────────────────────────────────────────────┤
│ Nodes ▾                │ Редактор маршрута via-kz-de             │
│  ● kz-server  linux    │  [1] client  [2] kz-server  [3] de-srv  │
│  ● de-server  linux    │  ^v — выбор хопа, +/- — добавить/убрать │
├──────────────────────────────────────────────────────────────────┤
│ Nodes: 3 · Routes: 3 · [?] help  [a] apply  [s] save  [q] quit   │
└──────────────────────────────────────────────────────────────────┘
```

Рендер-правила `topology.go`:
- один блок chain на маршрут, порядок = порядок в `routes:`;
- выбранный маршрут — акцентный цвет lipgloss, остальные — dim;
- нода Mikrotik/OpenWRT помечается значком типа (`◆` mikrotik, `▲` openwrt);
- терминалы < 80 колонок — компактный режим (список стрелок без рамок).

### 2.4Взаимодействие — карта клавиш

| Клавиша | Действие |
|---|---|
| `↑/↓`, `Tab` | навигация по панелям (topology → nodes → editor) |
| `Enter` | редактировать выбранный маршрут / открыть detail ноды |
| `+` / `-` | добавить/удалить хоп в позиции курсора редактора |
| `←/→` | переместить выбранный хоп по цепочке (drag-and-drop стрелками) |
| `e` | выбрать exit_node из конечных точек path |
| `n` / `N` | новая нода (форма) / удалить ноду (y/N подтверждение) |
| `s` | сохранить mesh.yaml (atomic write: tmp+rename) |
| `a` | применить (запускает тот же код, что `meshctl apply`; прогресс — spinner + лог в нижней панели; dry-run по умолчанию, `Shift+A` — боевой apply) |
| `?` | help overlay |
| `q` / `Ctrl+C` | выход (если unsaved — «Save/Discard/Cancel») |

### 2.5 Архитектурные требования

- **T2.5.1** TUI не дублирует бизнес-логику: работает через `config.Mesh` + `mesh.Manager` +
  `drivers.New(...)` — те же точки входа, что у CLI (single source of truth).
- **T2.5.2** Все SSH-операции — в `tea.Cmd` горутине; UI не блокируется; отмена — `Esc`
  (контекст `context.WithTimeout`, timeout = флаг `--timeout`, как в apply).
- **T2.5.3** known_hosts: завести общий `internal/sshx` (transport + host-key policy):
  файл `~/.config/meshctl/known_hosts`, политика TOFU (первое подключение — запись,
  далее — жёсткая проверка), флаг `--insecure-hostkey` и env `MESHCTL_SSH_INSECURE=1`
  как обход. Linux driver переводится на `sshx` (закрывает TODO Фазы 1).
- **T2.5.4** Изменения TUI в памяти помечают dirty-флаг; перед `apply`dirty-конфиг
  предлагается сохранить (иначе применяется то, что на диске — явная строка в статус-баре).

### 2.6 Критерии приёмки

- `meshctl tui` запускается, рисует топологию из реального `configs/mesh.yaml`.
- Можно: добавить hop стрелками, удалить hop, сохранить, применить, увидеть результат
  в логе применения без выхода из TUI.
- На cycle/ошибке валидации `apply` показывает человекочитаемую ошибку в панели лога
  и подсвечивает проблемный маршрут красным.
- `go vet ./... && golangci-lint run` чисто; TUI-пакет имеет smoke-тесты (§6 T-T1…T-T3).

---

## 3. Усиление валидации (P1)

Уже есть (не повторяем): циклы в `path`, неизвестные ноды, конфликт `interface:port`,
exit_node ≠ последний hop. Добавляем:

- [ ] **V1.** Проверка пересечения `mesh_cidr` с реальными подсетями нод — new-check
      только warning'ом (без хоста не узнать), но обязательная ошибка: `mesh_ip` ноды
      вне `cidr`, дубли `mesh_ip` между нодами, `mesh_ip` клиента пересекается с node-адресами.
- [ ] **V2.** Граф связности: все ноды из хотя бы одного маршрута обязаны быть попарно
      связаны цепочками соседства; изолированная нода — warning, разрыв цепи — error.
- [ ] **V3.** Duplicate-маршруты: два маршрута с идентичным `path` — warning.
- [ ] **V4.** `apply --check` расширяется до preflight: reachability (уже есть,
      `CheckReachability`) + SSH-auth probe (подключиться и выполнить `true`) + наличие
      `wg`/`wg-quick` на Linux-нодах. Результат — таблица ✓/✗ по нодам.
- [ ] **V5.** Ошибки валидации получают машинно-читаемый формат `errorf(node|route NAME, ...)`
      — TUI по имени подсвечивает объект (§2.5.4).

Файлы: `internal/mesh/validator.go` (+ тесты `mesh_test.go`).

---

## 4. Мелкие доработки CLI (P2)

- [ ] **C1.** `meshctl route edit <name> --path ...` — изменение path существующего маршрута
      (сейчас только remove+add).
- [ ] **C2.** `meshctl plan [--route <name>]` — вынести dry-plan из `apply --dry-run`
      в отдельную команду (TUI тоже её использует для превью).
- [ ] **C3.** `--json` для `node list`, `route list`, `plan` (автоматизация, основа будущего API).
- [ ] **C4.** `meshctl completion bash|zsh|fish` (cobra built-in).

---

## 5. Порядок работ (спринт-план)

| Неделя | Задачи |
|---|---|
| 1 | M1–M5 (multihop) + V1–V3 + тесты T-M* ; мерж-гейт: multihop e2e зелёный |
| 2 | Каркас TUI: app/topology/nodes/routes (чтение), sshx (T2.5.3), V4/V5 |
| 3 | editor.go (hop-редактор), apply из TUI, help/statusbar, полировка, C1–C4, док-и |

Документация: обновить README (раздел TUI + скриншот-каста asciinema), дополнить
`configs/mesh.yaml` примером 3-hop маршрута, раздел «Фаза 2 done» в плане ниже.

---

## 6. Тесты

| ID | Что проверяем | Вид |
|---|---|---|
| T-M1 | `AllowedIPsForPeer`: таблица случаев (relay/exit/client/1-hop/3-hop) | unit |
| T-M2 | `BuildPlan`: роли, next-hops, peer-списки на эталонных графах | unit |
| T-M3 | sysctl-файл ip_forward формируется корректно (мок runner) | unit |
| T-M4 | Стабильность рендера `.conf` при двойном apply (golden file) | unit |
| T-M5 | e2e: netns/docker — 3-hop цепочка, curl через exit | integration (make e2e) |
| T-V* | новые валидационные проверки (положительные/отрицательные) | unit |
| T-T1 | `topology.View` golden snapshot при 80×24 и 120×40 | unit |
| T-T2 | Update-редьюсер: клавиши +/-/←/→ корректно меняют path | unit (bubbletea testutil) |
| T-T3 | dirty/save: atomic write, восстановление после ошибки диска | unit |

`make test` = `go test ./... -race -coverprofile`; CI-github-actions уже настроен в Фазе 1 —
добавить job `make e2e` на ubuntu-latest (docker available).

---

## 7. Acceptance Criteria фазы (Definition of Done)

- [ ] AC1: Multihop 3+ hops работает end-to-end, повторный apply идемпотентен.
- [ ] AC2: `meshctl tui` показывает топологию всех маршрутов, поддерживает навигацию,
      редактирование path (добавить/убрать/переместить хоп), сохранение и apply.
- [ ] AC3: Preflight `apply --check` ловит недоступный хост, битый SSH-ключ, отсутствие wg.
- [ ] AC4: Host-key проверка (TOFU) включена по умолчанию; insecure — только явно.
- [ ] AC5: Все тесты §6 зелёные, coverage `internal/mesh` ≥ 80%, `internal/tui` ≥ 50%.
- [ ] AC6: README + примеры конфигов обновлены; Plan-001 чек-лист Фазы 2 отмечен ✅.

## 8. Риски и митигация

| Риск | Митигация |
|---|---|
| Транзитный `mesh_cidr` в AllowedIPs конфликтует с сетью провайдера ноды | V1 warning + опция `transit: false` на hop (позже); документировать выбор CIDR из диапазона CGNAT-подобных /24 |
| TUI «раздувается» и дублирует CLI-логику | T2.5.1 (общий core), code-review гейт: в `internal/tui` запрещено импортировать `os/exec` и `crypto/ssh` напрямую |
| wg sync рвёт сессии при apply | M4 golden-тест + сравнение remote-config перед записью (driver применяет только при diff) |
| Bubble Tea breaking changes | пиновать версии из §2.1, upgrade — отдельным PR |

---

## 9. Чек-лист выполнения (обновлять по ходу)

- [ ] M1 AllowedIPsForPeer
- [ ] M2 buildNodeSpec на новом API
- [ ] M3 персистентный ip_forward
- [ ] M4 идемпотентный рендер
- [ ] M5 e2e 3-hop
- [ ] TUI: app / topology / nodes / routes / editor / statusbar / keys
- [ ] sshx + TOFU known_hosts
- [ ] V1–V5 валидация + preflight
- [ ] C1–C4 CLI-доработки
- [ ] Тесты T-* , coverage-гейт
- [ ] README/док-и, отметка в Plan-001
