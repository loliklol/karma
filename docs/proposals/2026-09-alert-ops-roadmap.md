# Roadmap: историчность алертов, EVA, пресеты окружений, list/strips UI

**Статус:** проектирование (без реализации)  
**Репозиторий:** форк [prymitive/karma](https://github.com/prymitive/karma) — dashboard поверх Alertmanager  
**Дата:** 2026-09-03

## 1. Текущее состояние (факт)

| Область | Как сейчас | Ограничение |
|--------|------------|-------------|
| Данные | Stateless: poll Alertmanager в память | Нет persistence; после рестарта/resolve след теряется |
| «History» | `history.enabled` → Prometheus `ALERTS_FOR_STATE` за 24ч, heatmap | Это счётчик срабатываний, **не** журнал событий и не тикеты |
| Тикеты | `silences.comments.linkDetect` — regex ID в комментарии silence → URL | Только отображение ссылки; нет create/track open/resolved |
| Фильтры | `filters.default` + `localStorage` (`savedFilters`, history фильтров) | Один «сохранённый» набор; нет именованных пресетов окружений |
| Multi-grid | UI: один label (`multiGridConfig`) | Нет сохранённых «наборов лейблов» для быстрой смены окружений |
| UI групп | Bootstrap `card` + masonry (`bricks.js`), `minimalGroupWidth≈420` | «Плитки»; плотная лента (strips) не поддержана |
| БД | Отсутствует | Любая качественная оценка/связь с EVA потребует store |

Ключевые точки кода:

- backend history: `cmd/karma/alert_history.go`
- модель алерта / `LabelsFP`: `internal/models/alert.go`
- UI группа-карточка: `ui/src/Components/Grid/AlertGrid/AlertGroup/`
- settings/localStorage: `ui/src/Stores/Settings.ts`
- silence ticket detect: `silences.comments.linkDetect` в `docs/CONFIGURATION.md`

## 2. Цели

1. **Историчность алертов** — кто/когда firing→resolved, повторные вспышки, длительность, корреляция с тикетами.
2. **EVA** — из алерта создать задачу; видеть, есть ли открытый тикет; видеть resolved/closed.
3. **Пресеты окружений** — сохранять и переключать наборы фильтров/лейблов для показа окружений.
4. **UI strips** — компактные полоски вместо крупных плиток (режим, не ломая tiles).

Клиент EVA: [`github.com/raoptimus/evateamclient.go`](https://github.com/raoptimus/evateamclient.go)  
(`TaskCreate`, `TasksList` + `cache_status_type` / `StatusTypeOpen`, `Task` get, status history).

## 3. Архитектурные решения (зафиксировано 2026-09-03)

### ADR

| Вопрос | Решение |
|--------|---------|
| БД | **Сразу PostgreSQL** (единственный драйвер v1). SQLite не делаем. |
| EVA target | **Выбор в UI**: проект или Service Desk (как project). Не хардкодить один project. |
| Пресеты | **Shared на сервере с релиза 1**, видны всей команде. |

### 3.1 Persistence: PostgreSQL

Prometheus-history оставить как есть (опциональный heatmap). Для ops-оценки — **свой event store на Postgres**.

Слой `internal/store` + драйвер `postgres` (pgx), миграции (goose/golang-migrate — выбрать при реализации).

Конфиг:

```yaml
persistence:
  driver: postgres
  dsn: "${KARMA_DATABASE_URL}"   # postgres://user:pass@host:5432/karma?sslmode=require
  maxOpenConns: 20
  retention:
    events: 90d
```

Минимальные сущности:

```
alert_identity
  id, fingerprint (LabelsFP), labels_hash, labels_json, alertname, first_seen, last_seen

alert_event
  id, alert_id, ts, type (fired|resolved|silenced|unsilenced|inhibited|updated),
  state, starts_at, cluster, receiver, snapshot_json

alert_ticket
  id, alert_id, provider=eva, project_code, project_id,
  external_id, external_code, url,
  status (open|closed|unknown), created_at, updated_at, created_by

ui_preset
  id, name, filters_json, grid_label,
  created_by, updated_by, created_at, updated_at
  -- shared team-wide (нет personal-only режима в v1)
```

**Источник событий:** на каждом успешном poll Alertmanager (`internal/alertmanager`) diff с предыдущим snapshot по `LabelsFP` (+ cluster при необходимости) → запись `alert_event`.  
Не дублировать полный dump AM — только переходы и периодический heartbeat `last_seen`.

**Ключ идентичности алерта:** `LabelsFP` (уже есть) + опционально нормализованный набор labels из конфига (`history.identityLabels` / `eva.identityLabels`), чтобы не плодить сущности при шумных labels.

### 3.2 EVA integration — выбор проекта / Service Desk

```
┌─────────┐   poll    ┌──────────────┐  diff   ┌──────────┐
│ Alertmgr│ ────────► │ karma core   │ ──────► │ Postgres │
└─────────┘           └──────┬───────┘         └────┬─────┘
                             │ API                  │
                      ┌──────▼───────┐       ┌──────▼─────┐
                      │ UI / REST    │ ◄────► │ EVA client │
                      └──────────────┘       └────────────┘
```

**Как устроен Service Desk в либе:** отдельного `ServiceDeskCreate` в [`evateamclient.go`](https://github.com/raoptimus/evateamclient.go) нет. SD — это проект EVA (в OAS есть флаги `show_servicedesk_*` на project). Создание тикета = `TaskCreate` с `parent` = выбранный project (обычный или SD). Либа это поддерживает: `Projects` / `Project` + `TaskCreate(ProjectID)`.

Поток в UI:

1. «Создать в EVA» → модалка.
2. Выбор **target** из allowlist (OPS / SD / другие).
3. Опционально preselect: `defaultProject` или правило по labels (`severity` → project).
4. Submit → `TaskCreate` → запись в `alert_ticket` (с `project_code`).

Конфиг (черновик):

```yaml
eva:
  enabled: true
  baseURL: https://api.eva.team   # или on-prem
  apiToken: "${EVA_TOKEN}"
  webBaseURL: https://eva.example
  timeout: 15s
  syncInterval: 1m
  # allowlist целей для UI picker (project code → метка)
  targets:
    - code: OPS
      label: "Ops board"
      kind: project          # project | servicedesk (только UI-метка)
    - code: SD
      label: "Service Desk"
      kind: servicedesk
  defaultTarget: OPS         # preselect в модалке; юзер может сменить
  # опциональный авто-роут (не вместо выбора, а подсказка default)
  routes:
    - match:
        label: team
        value: platform
      target: OPS
    - match:
        label: severity
        value: critical
      target: SD
  task:
    nameTemplate: "[{{ .Alertname }}] {{ .Labels.cluster }}/{{ .Labels.instance }}"
    textTemplate: |
      ...
    tags: ["karma", "alert"]
  identityLabels: ["alertname", "cluster", "service"]
```

Backend API (черновик):

| Method | Path | Назначение |
|--------|------|------------|
| GET | `/eva/targets` | allowlist проектов/SD для picker |
| POST | `/eva/tasks` | body: `alertId` + `target` (project code) |
| GET | `/eva/tasks?alertId=` | статус тикета(ов) для алерта |
| POST | `/eva/tasks/sync` | force sync статусов (или только внутренний ticker) |

Поведение:

1. **Create:** `TaskCreate` (`Name`, `ProjectID`=выбранный target, `Text`, `Tags`, …) → `alert_ticket`.
2. **Дедуп:** open-ticket для того же identity — не создавать второй (вернуть существующий + soft-warn). Scope дедупа: global по identity (не per-project), чтобы не плодить OPS+SD на один алерт; override — явный force в UI.
3. **Sync:** `Task` / list по code/id; `cache_status_type` (`OPEN` → open, иначе closed/unknown).
4. **UI:** picker target + бейдж `EVA OPS-123 · open|done` со ссылкой.
5. **Фильтр (позже):** `@eva_ticket=…`, `@eva_status=open`, `@eva_project=OPS`.

Auth: token только на сервере; UI → karma API. ACL на create — отдельная подзадача.

### 3.3 Пресеты окружений — shared team-wide с v1

Без localStorage-only фазы. Пресеты в Postgres, общие для всей команды.

```yaml
# seed / defaults при старте (опционально), дальше CRUD в UI
ui:
  presets:
    - name: prod
      filters: ["cluster=prod", "@state=active"]
      gridLabel: namespace
    - name: stage
      filters: ["cluster=stage"]
      gridLabel: namespace
```

API:

| Method | Path | Назначение |
|--------|------|------------|
| GET | `/presets` | список team presets |
| POST | `/presets` | создать |
| PUT | `/presets/{id}` | обновить |
| DELETE | `/presets/{id}` | удалить |

UI: dropdown «Окружения» — выбрать / сохранить текущие фильтры+gridLabel / перезаписать / удалить.  
Права: любой аутентифицированный пользователь команды (ужесточение ACL — позже).

«Наборы лейблов» = **фильтры + (опц.) multi-grid label**, не отдельная модель labels AM.

### 3.4 UI: strips вместо плиток

Новый режим `ui.layout: tiles | strips` (toggle в settings / YAML default).

| | tiles (as-is) | strips |
|--|---------------|--------|
| Контейнер | masonry card, fixed min width | full-width row / CSS grid 1 col |
| Высота | высокая карточка | одна линия: state · alertname · key labels · age · EVA · actions |
| Детали | сразу в card-body | expand/accordion или side drawer |
| History heatmap | в карточке | в expanded / tooltip |

Затрагивает: `AlertGroup`, `Grid.tsx` / `useGrid`, CSS; bricks.js в strips-режиме отключить.

Рекомендация: **feature flag / setting**, не выкидывать tiles сразу.

## 4. Разбивка на подзадачи

### Epic A — Foundation / PostgreSQL store

| ID | Подзадача | Зависит |
|----|-----------|---------|
| A1 | `internal/store` + Postgres (pgx) + миграции | — |
| A2 | Конфиг `persistence:` (dsn, pool, retention) | A1 |
| A3 | Diff poll → `alert_event` / `alert_identity` | A1 |
| A4 | Retention job (N дней events) + метрики | A3 |
| A5 | API `GET /history/alerts`, `GET /history/alerts/{id}/events` | A3 |

### Epic B — EVA (picker project / Service Desk)

| ID | Подзадача | Зависит |
|----|-----------|---------|
| B1 | go.mod: `evateamclient.go`, wrapper `internal/eva` | — |
| B2 | Конфиг `eva.targets` / `defaultTarget` / `routes` + templates | B1 |
| B3 | `GET /eva/targets` (resolve codes → id через `Projects`) | B2 |
| B4 | Таблица `alert_ticket` + `POST/GET /eva/tasks` с `target` | A1, B2 |
| B5 | Дедуп open-ticket по identity (+ force override) | B4 |
| B6 | Background sync статусов | B4 |
| B7 | UI: модалка выбора target + create + badge | B3–B6 |
| B8 | Фильтры `@eva_*` + ACL на create | B7 |
| B9 | Обогащение `/alerts.json` полями ticket (batch) | B6 |

### Epic C — Shared пресеты окружений

| ID | Подзадача | Зависит |
|----|-----------|---------|
| C1 | Таблица `ui_preset` + CRUD API | A1 |
| C2 | YAML seed `ui.presets` → upsert при старте | C1 |
| C3 | UI switcher / save / overwrite / delete (team-wide) | C1 |
| C4 | (позже) ACL: кто может edit/delete | C3 |

### Epic D — Strips UI

| ID | Подзадача | Зависит |
|----|-----------|---------|
| D1 | Setting `layout: tiles\|strips` + CSS tokens | — |
| D2 | `AlertRow` compact component | D1 |
| D3 | Grid: отключить masonry в strips | D2 |
| D4 | Expanded details + EVA actions в row | D2, B7 |
| D5 | Mobile / a11y / e2e visual stories | D3 |

### Epic E — «Качественная оценка» (аналитика)

| ID | Подзадача | Зависит |
|----|-----------|---------|
| E1 | MTTA/MTTR, flapping score по identity | A5 |
| E2 | Отчёт «алерты без тикета / с открытым тикетом» | B6, A5 |
| E3 | UI history timeline (не только Prometheus heatmap) | A5 |

## 5. Рекомендуемый порядок поставки

```
D1–D3 (strips UX, без БД)
  ∥ A1–A2 (Postgres foundation)
    → C1–C3 (shared presets)     # нужны сразу всей команде
    → A3–A5 (events + history API)
      → B1–B7 (EVA picker + create + status)
        → E1–E3 (оценка / timeline)
```

**D** единственный эпик, который стартует без Postgres. **C и B** ждут **A1**.

## 6. Риски и ложные допущения

- **«History в karma уже есть»** — нет event-store, только Prometheus heatmap.
- **«Тикеты уже есть»** — только парсинг ID в silence comment, не lifecycle.
- **«Отдельный Service Desk API в либе»** — нет; SD = project + `TaskCreate(parent)`. Нужен allowlist реальных project codes.
- **Fingerprint AM ≠ LabelsFP karma** — для EVA: явный identity + `external_code`.
- **Шумные labels** — без `identityLabels` тикеты и история разъедутся.
- **EVA rate limits / latency** — sync/enrich batch + cache, не на каждый UI poll.
- **Shared presets без ACL** — любой может затереть team preset; C4 желателен рано.
- **Upstream merge** — держать diff в `internal/eva`, `internal/store`, UI flags.

## 7. Открытые вопросы — закрыты

| # | Было | Решение |
|---|------|---------|
| 1 | SQLite vs Postgres | **Postgres сразу** |
| 2 | Один project vs labels | **Picker проекта/SD** + optional routes; defaultTarget |
| 3 | Personal vs shared presets | **Shared team-wide с v1** |

Остаётся уточнить при реализации (не блокирует дизайн):

- точные project codes OPS/SD в вашем EVA;
- нужен ли force-create второго тикета в другой target при уже open;
- auth model karma (header/proxy) для `created_by` на presets/tickets.

## 8. Out of scope сейчас

- Реализация кода (этот документ — проектирование).
- Замена Alertmanager / Prometheus.
- Полноценный incident management (postmortem, on-call) — только связь alert↔EVA task.
- Отдельный ServiceDesk API в форке либы (используем project + TaskCreate).
