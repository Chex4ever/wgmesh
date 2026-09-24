# Plan-004: Фаза 4 — Продвинутые фичи (Git sync, мониторинг, обфускация, экспорт)

> Родительский документ: [Plan-001.md](./Plan-001.md), раздел «🔴 Фаза 4».
> Предусловия: ✅ Фазы 1–3 (Plan-002, Plan-003). К началу фазы доступны:
> `drivers.Driver` v2 с `Status()` и `Preflight()` (П3 §1), `meshctl status` базовой версии
> (П3 §5), known_hosts/sshx (П2 §2.5.3), TUI с панелями нод/маршрутов (П2 §2).

**Цель фазы:** превратить meshctl из инструмента настройки в инструмент эксплуатации:
коллаборация через Git, непрерывный мониторинг mesh, обфускация трафика (AmneziaWG)
и экспорт конфигов в форматы сторонних клиентов.

**Длительность:** 3–4 недели. Все четыре блока независимы — можно выполнять
в любом порядке и патчами-релизами (каждый § — отдельная minor-версия: v0.4.0-git,
v0.4.1-watch, v0.4.2-amnezia, v0.4.3-export).

---

## 1. Git sync (P0 — коллаборация)

### 1.1 Модель

Конфиг (`mesh.yaml` + каталог `nodes/`) уже спроектирован как версионируемый.
Задача — сделать workflow безопасным для команды из N человек без центрального сервера.

Принципы:
- meshctl **не требует** git-репозиторий, но если конфиг внутри репозитория — умеет с ним работать;
- секреты не коммитятся: private keys выносятся в sidecar-файл (см. 1.3);
- все операции — обёртки над `git` CLI (не go-git: меньше зависимостей, поведение = привычное пользователю).
  Решение зафиксировать в ADR `docs/adr/004-git-cli-vs-gogit.md`.

### 1.2 Команды

| Команда | Действие |
|---|---|
| `meshctl git init` | `git init` + `.gitignore` (исключает `*.private.yaml`, `known_hosts`) + первый коммит конфига |
| `meshctl git pull` | fetch+merge/rebase; при конфликте в mesh.yaml — стоп с инструкцией (см. 1.4 structural merge) |
| `meshctl git push` | push текущей ветки; `-m "сообщение"` (по умолчанию авто-сообщение: кто/что/когда) |
| `meshctl git status` | что изменено в конфиге относительно HEAD (diff по YAML-секциям, а не по строкам) |
| `meshctl git log [<node>\|<route>]` | история изменений конкретного объекта (`git log -L` по якорям, см. 1.4) |
| `meshctl git remote add <url>` | делегирование `git remote` (для онбординга нового участника) |

Автокоммит: после любой мутации (`node add`, `route edit`, apply-генерация ключей) —
опциональный hook `sync.auto_commit: true` в mesh.yaml (по умолчанию false).

### 1.3 Секреты вне Git

Сейчас `wireguard.private_key` хранится прямо в mesh.yaml → утечка при пуше. Исправляем:

- [ ] **G1.** Новый флаг loader: приватные ключи сериализуются в `nodes/<name>.private.yaml`
      (chmod 600) и добавляются в `.gitignore`; в mesh.yaml остаётся только `public_key`.
- [ ] **G2.** Миграция существующих конфигов: `meshctl migrate --to v2-secrets` —
      выносит найденные private_key из mesh.yaml в sidecar, делает коммит «chore: move secrets out of VCS».
      В README — заметное предупреждение: «если ключи уже были в истории Git — перевыпустить пару на всех нодах»
      (`meshctl node rotate-keys <name>`: перегенерация + auto apply affected routes).
- [ ] **G3.** Опционально (P2): поддержка sops/age — `sync.age_recipients: [pubkey...]`
      шифрует sidecar перед коммитом. Только если команда просит; иначе оставляем как future work.

### 1.4 Structural merge вместо текстового

YAML-конфликты решаются на уровне модели:

- лоадер читает три версии (`git show :1/:2/:3 mesh.yaml` → base/ours/theirs в `config.Mesh`);
- правила merge: узлы/маршруты ключуются по `name`; изменение разных нод → авто-merge;
  изменение одной ноды в обеих ветках → конфликт по объекту с выводом diff полей;
- результат пересериализуется канонически (стабильный порядок полей, 2-space indent) —
  это же даёт детерминированный diff и годится для `git log -L` по секции ноды.

Реализация: `internal/gitx/{repo.go,merger.go,secrets.go}` + тесты на табличных merge-кейсах.

### 1.5 Acceptance (§1)

- [ ] AC-G1: два оператора клонируют репозиторий, каждый добавляет свою ноду, оба пушят — merge чистый.
- [ ] AC-G2: одновременное правление одной ноды → понятный конфликт уровня объекта, ничего не потеряно.
- [ ] AC-G3: ни один private key не попадает в `git history` после миграции (проверка `git log -p | grep private_key` пуста).
- [ ] AC-G4: `meshctl git log kz-server` показывает, кто и когда менял эту ноду.

---

## 2. Мониторинг (P0)

### 2.1 `meshctl status` v2 (развивает П3 §5)

```
$ meshctl status
NODE          TYPE      ONLINE  PEERS(ok/all)  LATENCY   EXIT ROUTES        LAST HANDSHAKE
kz-server     linux     ●       3/3            18 ms     via-kz, via-kz-de  2s ago
de-server     linux     ●       2/2            142 ms    via-kz-de          5s ago
home-router   mikrotik  ○       0/2            —         —                  never
```

Источники данных:
- `Driver.Status()` (handshake/bytes per peer) — есть с П3;
- latency: ICMP ping mesh_ip соседей + внешний `1.1.1.1` с exit-нод (проверка, что интернет с exit реально есть);
- «exit check»: с каждой exit-ноды `curl -s ifconfig.me` (через SSH) — совпадает ли ожидаемый публичный IP
  (детект тихой деградации NAT).

Флаги: `--json` (контракт схемы в `docs/status-schema.md`, semver-стабильна), `--route <name>`
(цепочка по хопам с per-hop статусом), `--no-ssh` (пропуск ssh-зависимых колонок).

### 2.2 `meshctl watch` — непрерывный режим

- Реализация поверх TUI: `meshctl watch` == `meshctl tui --mode monitor` (переиспользует topology-рендер П2).
- Poll-цикл: интервал `--interval 10s`, fan-out по нодам errgroup (лимит 4, из П3),
  ctx-отменяемый; ошибки сети не роняют UI — нода переходит в `○` с причиной в detail.
- Деградация состояния видна цветом: зелёный (все handshake свежее 3×interval) →
  жёлтый (stale) → красный (нет peers ok).

### 2.3 Алерты

Минимальная система без внешних зависимостей, интерфейс `alertnotifier`:

```yaml
alerts:
  enabled: true
  rules:
    - on: node_down          # online -> offline
    - on: handshake_stale    # peer без handshake > 5m
      threshold: 5m
    - on: exit_ip_mismatch
  notify:
    webhook: "https://hooks.example.com/mesh"   # POST JSON {event, node, ts}
    telegram:                                    # опционально (P2)
      bot_token_env: MESHCTL_TG_TOKEN            # токен только из env!
      chat_id: "..."
    command: "/usr/local/bin/mesh-alert.sh"      # local hook, argv-safe
```

- `meshctl alert-test` — послать тестовое уведомление по всем каналам.
- Хранение последнего состояния — `~/.local/state/meshctl/watch-state.json`
  (дедупликация алертов: один инцидент = одно уведомление до восстановления).

### 2.4 Acceptance (§2)

- [ ] AC-W1: `status --json` стабилен по схеме (golden-тест), потребители (TUI, алерты) читают его, а не парсят текст.
- [ ] AC-W2: вырубить ноду → в течение 2×interval она красная в `watch`, webhook получил ровно 1 событие `node_down`, восстановление → 1 событие `node_up`.
- [ ] AC-W3: `watch` корректно работает 24h (утечки: pprof heap snapshot в CI-смоке — рост < 5 MB/сут).
- [ ] AC-W4: убитый webhook (timeout) не блокирует poll-цикл.

---

## 3. Обфускация — AmneziaWG (P1)

### 3.1 Контекст

AmneziaWG — форк WireGuard с параметрами обфускации первого пакета
(`H1,H2,H3,H4` — заголовки пакетов, `S1..S4` — размеры, `Jc/Jmin/Jmax` — джиттер).
Ключевое: **обе стороны туннеля должны использовать одинаковые параметры**, иначе handshake не состоится.
Client-конфиг AmneziaWG понимает; на серверах нужна установка kernel-модуля или userspace-вариант.

### 3.2 Модель конфига

```yaml
nodes:
  - name: kz-server
    wireguard:
      interface: wg0
      obfuscation:            # null = обычный WG
        preset: "ampere"      # пресеты AmneziaWG: ampere|kwanyee|mirai|rise|rsal|voyager|null
        # либо ручные поля: h1,h2,h3,h4,s1,s2,s3,s4,jc,jmin,jmax
```

- [ ] **A1.** `internal/wg/obfuscation.go`: тип `ObfuscationParams`, пресеты (таблица из
      AmneziaWG repo, зафиксирована снапшот-тестом), валидация диапазонов, `Equal(a,b)` для
      проверки согласованности пар peer'ов.
- [ ] **A2.** Согласование при построении конфига: если у ноды-A и ноды-B разные
      `obfuscation` — ошибка валидации уровня пары («route via-kz: kz-server и de-server имеют
      разные параметры обфускации») — ловится ещё до apply.
- [ ] **A3.** Рендер: клиентский `.conf` получает блок AmneziaWG-параметров;
      server-side wg-quick конфиг — тоже (amnezia-key совместим с обычным wg).
- [ ] **A4.** Установка: linux-driver при `obfuscation != null` ставит модуль:
      скрипт `scripts/install-amnezia.sh` (DKMS) как PostUp-проверка Preflight
      (`modinfo amneziawg`); без модуля — понятная ошибка со ссылкой на инструкцию.
      Mikrotik/OpenWRT: обфускация НЕ поддерживается → capability matrix (П3 §1.3)
      `SupportsObfuscation=false`, валидатор запрещает смешивать hop-AmneziaWG с hop-Mikrotik в одной цепи.
- [ ] **A5.** CLI: `meshctl route add ... --obfuscate [preset]` (из Plan-001) — проставляет
      параметр всем нодам маршрута + предупреждение о перезагрузке WG-модуля;
      `meshctl node set-obfuscation <node> <preset>|none`.

### 3.3 Acceptance (§3)

- [ ] AC-A1: маршрут `client(amnezia) → kz(amnezia) → de(exit)` поднимается, трафик идёт,
      Wireshark на внешнем интерфейсе kz не видит magic WG handshake (проверка вручную, лог в PR).
- [ ] AC-A2: рассогласование параметров между соседями диагностируется до apply.
- [ ] AC-A3: mikrotik в цепи с obfuscation-нодой → ошибка валидации с объяснением.

---

## 4. Экспорт конфигов (P1)

### 4.1 Форматы

| Формат | Команда | Описание |
|---|---|---|
| wireguard | `meshctl export <route> --format wireguard -o client.conf` | стандартный `.conf` (перенос существующего client-config; `client-config` становится алиасом) |
| amnezia | `meshctl export <route> --format amnezia -o profile.json` | формат AmneziaVPN import (JSON-профиль: `type: "amnezia-wireguard"`, поля H/S/J + keys) — открывать в приложении Amnezia |
| qr | `meshctl export <route> --format wireguard --qr` | PNG QR (уже есть в client-config — переиспользовать рендер `internal/wg`) |

Shadowsocks-совместимость — ❌ НЕ в этой фазе (future work, зафиксировать в docs/roadmap-backlog.md).

### 4.2 Задачи

- [ ] **E1.** `internal/export/` — registry форматов: `Format{Name, Render(mesh, route, clientKeys) ([]byte, error)}`.
      Добавление нового формата = новый файл + регистрация (open/closed).
- [ ] **E2.** Amnezia-профиль: точное соответствие схеме импорта Amnezia (проверить на актуальной
      версии приложения; снапшот golden-файл `testdata/amnezia_profile.golden.json`).
      Приватный ключ клиента — генерируется/читается из client-state (механизм `clientPublicKey`
      из cli/client.go вынести в `internal/clientstore`).
- [ ] **E3.** `meshctl export --all-formats --outdir ./dist-configs` — пакет для раздачи
      (имена файлов `<mesh>-<route>.<ext>`).
- [ ] **E4.** Безопасность: экспорт содержит private key клиента → warning при записи
      в world-writable каталог; stdout при pipe остаётся чистым (warnings — в stderr).
- [ ] **E5.** TUI: в detail-панели маршрута — action `x` → выбор формата → сохранение + показ QR.

### 4.3 Acceptance (§4)

- [ ] AC-E1: профиль amnezia импортируется в AmneziaVPN (Android/desktop — ручной тест, скриншот в PR) и коннектится к mesh.
- [ ] AC-E2: wireguard-экспорт байт-в-байт совместим с текущим `client-config` (regression golden).
- [ ] AC-E3: `export --all-formats` для 3-нодового примера из `configs/` создаёт корректный набор файлов.

---

## 5. Общие инженерные задачи фазы

- [ ] **X1.** Версионирование конфига: поле `version` в mesh.yaml реально используется лоадером
      (миграции v1→v2(secrets)→v3(obfuscation) — цепочка `config.Migrate`), плюс `meshctl migrate --check`.
- [ ] **X2.** Сборка релизов: goreleaser (бинарники linux amd64/arm64, darwin arm64, checksums, brew tap) —
      финальный штрих «один бинарник, zero deps» из философии Plan-001.
- [ ] **X3.** Документация: `docs/` — runbook восстановления (teardown+reapply), git-workflow для команды,
      таблица пресетов обфускации, схема `status --json`. README — секция «Эксплуатация».
- [ ] **X4.** e2e-матрица в CI: docker-compose стенд (2 linux-ноды + mock-интернет) прогоняет
      multihop + status + export на каждом PR (nightly, чтобы не греть квоты).
- [ ] **X5.** скоуп-срез: daemon/HTTP API в этой фазе НЕ делаем — фиксируем в backlog.

## 6. Порядок работ

| Неделя | Задачи |
|---|---|
| 1 | §1 Git sync (G1–G2, merge) + X1 миграции версии конфига |
| 2 | §2 Мониторинг (status v2, watch, alerts) |
| 3 | §4 Экспорт (E1–E5) + §3 AmneziaWG (A1–A3) |
| 4 | A4–A5 (серверная установка обфускации), X2–X4, стабилизация, v0.4.0 release |

Приоритеты позволяют отрезать §3 (обфускация) без влияния на остальные блоки —
это «по желанию» из Plan-001, при нехватке времени уходит в v0.5.

## 7. Definition of Done всей фазы

- [ ] Все AC подразделов §1–§4 закрыты или явно перенесены в backlog с отметкой в Plan-001.
- [ ] `meshctl doctor` (новая meta-команда: версия конфига, секрет-гигиена, git-clean, reachable ноды,
      согласованность obfuscation) — зелёный на демо-конфиге.
- [ ] Релиз v0.4.0 на GitHub Releases с бинарниками (goreleaser), CHANGELOG.md ведётся с этой фазы.
- [ ] Чек-листы Plan-001 (Фаза 4) отмечены; план-документы Plan-002…004 помечены ✅ completed.

## 8. Риски

| Риск | Митигация |
|---|---|
| Смена схемы AmneziaVPN import (приложение обновляется часто) | golden-тест + pin версии приложения в docs; экспорт-форматы версионируются (`--amnezia-schema v1/v2`) |
| Секретные ключи уже в git-истории у ранних пользователей | G2 миграция + rotate-keys + предупреждение в README/release notes |
| Watch-цикл долбит слабые ноды (роутеры) | adaptive interval: микротик-ноды опрашиваются вдвое реже; лимит concurrency |
| DKMS-модуль AmneziaWG ломает апгрейд ядра на ноде | Preflight проверяет `modprobe amneziawg` dry-run; инструкция rollback; userspace-вариант как plan B |
| Конфликт автокоммита с ручными коммитами пользователя | auto_commit stage-ит только файлы meshctl-конфигов и пишет префикс `meshctl:` в сообщение; никогда не `commit -a`, не `push --force` |

## 9. Чек-лист выполнения

- [ ] §1 gitx + secrets sidecar + migrate + structural merge + AC-G*
- [ ] §2 status v2 + watch + alerts + AC-W*
- [ ] §3 obfuscation model + presets + validation + install + AC-A*
- [ ] §4 export registry + amnezia profile + TUI action + AC-E*
- [ ] X1–X5, doctor, v0.4.0 release, отметки в Plan-001
