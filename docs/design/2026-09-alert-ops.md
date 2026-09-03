# RFC: история алертов, EVA-тикеты, виды окружений, компактный список

Статус: **черновик для согласования, код не пишем**.
Дата: 2026-09-03.
Репозиторий: форк `prymitive/karma` (`loliklol/karma`).

---

## 1. Вердикт

Делаем четыре связанных изменения. Два из них требуют persistence, два — нет.

| Тема | Нужна БД | Можно стартовать без БД |
|---|---|---|
| Компактный список вместо плиток | нет | да, сразу |
| Именованные наборы лейблов/видов окружений | желательно | да, localStorage → потом сервер |
| Настоящая историчность алертов | **да** | нет |
| Тикет в EVA + статус open/resolved | **да** | нет (иначе после рестарта всё пропадает) |

Рекомендация по стеку:

- **PostgreSQL** как единственное хранилище v1. SQLite сознательно не берём: karma часто крутится в нескольких репликах за ingress, а тикеты/история должны быть общими.
- Историю пишем **из poll-цикла karma** (уже есть ticker на Alertmanager), а не из Prometheus `ALERTS_FOR_STATE`. Текущий sparkline 24h оставляем как дополнительный сигнал, не как source of truth.
- EVA — только через [`github.com/raoptimus/evateamclient.go`](https://github.com/raoptimus/evateamclient.go). Тикет создаёт оператор явно, автосоздания в v1 нет.
- UI по умолчанию — **полоски на всю ширину**. Карточки masonry (`bricks.js`) остаются как density=`cards`, чтобы не ломать привычку одним релизом.

Порядок поставки: compact UI и named views можно катить параллельно с фундаментом БД. EVA и качество алертов — строго после persistence + identity.

---

## 2. Как есть сейчас (и чем это не является)

Karma — **stateless прокси + UI**. Состояние живёт в Alertmanager и в `localStorage` браузера.

### 2.1. «История» уже есть, но это не историчность

`history:enabled` ходит в Prometheus по `source` алерта и считает `changes(ALERTS_FOR_STATE[1h])` за 24 часа. Это heatmap «сколько раз дёргалось за час», с LRU-кэшем на 5 минут.

Не даёт:

- момент fire / resolve,
- длительность эпизода,
- кто взял в работу,
- связь с тикетом,
- статистику качества (флап, MTTA, MTTR),
- историю после рестарта Prometheus / смены `external-url`.

Alertmanager сам по себе resolved-алерты не хранит. Как только алерт погас — UI его не видит.

### 2.2. Тикеты уже «есть», но только как парсинг комментария silence

`LinkDetectRule` вытаскивает JIRA-подобный ID из текста сайленса и клеит URL. Нет создания задачи, нет статуса, нет «для этого алерта уже открыт тикет».

### 2.3. «Сохранённые фильтры» — один безымянный набор на браузер

`Settings.savedFilters` + история последних фильтров в `localStorage`. Multi-grid — один `gridLabel` (например `cluster` / `env`). Нельзя:

- сохранить несколько именованных видов («prod», «stage+infra»),
- шарить вид между людьми,
- ограничить, какие значения окружения показывать.

### 2.4. Плитки

Группа = Bootstrap `card` фиксированной ширины (`MinimalGroupWidth` ≈ 420px), раскладка masonry через `bricks.js`. На большом экране это сетка крупных блоков, а не список инцидентов.

### 2.5. Идентичность алерта

- AM `fingerprint` — per-instance, меняется между перезапусками правил / разными AM.
- karma `LabelsFP` — xxhash **всех** лейблов. Подходит для текущей дедупликации, плохо как ключ тикета: `pod`/`instance` плодят «новые» алерты.

Без явного identity-key тикеты и история разъедутся.

---

## 3. Ложные допущения

1. **«История karma = история алертов».** Нет. Это 24h Prometheus sparkline.
2. **«Fingerprint AM достаточно как PK».** Нет. Нужен стабильный ключ из subset лейблов.
3. **«EVA сама скажет karma про статус, если положить ссылку в annotation».** Нет. Нужно локальное хранение `task_code` и периодический sync (`cache_status_type` / status history).
4. **«localStorage хватит для видов окружений».** Хватит как прототип. Для команды на дежурстве — нет: разные браузеры, инкогнито, N реплик UI.
5. **«БД можно не вводить, если писать в EVA и Prometheus».** Prometheus не знает про тикеты. EVA не знает про эпизоды fire/resolve. Склейка без своей таблицы будет хрупкой.
6. **«Карточки можно просто сузить».** Masonry + card-header/footer/annotations — другая информационная плотность. Нужен отдельный row-layout, не `groupWidth: 200`.

---

## 4. Целевая картина

Оператор видит **список полосок**: severity, `alertname`, ключевые лейблы, возраст, silence, бейдж EVA.

С полоски:

- создать задачу в EVA (если открытой ещё нет),
- открыть существующую,
- понять, открыта она или уже resolved/closed.

Наборы видов: «Prod», «Stage», «DB» — переключаются одним кликом, содержат фильтры + лейбл группировки окружений + опционально whitelist значений.

Отдельно (не на той же полоске): страница/модалка качества — сколько раз алерт стрелял за 7/30 дней, средняя длительность, доля эпизодов с тикетом, MTTA.

```
[env=prod ▼]  [вид: Prod ▾]  [фильтры…]
────────────────────────────────────────────────────────────
prod
  ▸ Watchdog          severity=none   14d   silenced
  ● KubePodCrashLoop  ns=api pod=*    23m   EVA OPS-1421 open
  ● DiskWillFill      instance=db-1   2h    [Создать задачу]
stage
  ● CertExpiring      ingress=foo     6d    EVA OPS-1390 closed
```

---

## 5. Почему БД и какая

Писать историю «в память процесса» бесполезно: рестарт = дыра, две реплики = два мира.

Минимальный набор таблиц, без которого EVA и качество не живут:

```
alert_identity     стабильный ключ + набор лейблов
alert_episode      один непрерывный firing (starts_at / ends_at / last_seen)
alert_event        append-only: fired | resolved | silenced | ticket_*
ticket             связь identity → EVA task, текущий статус
view_preset        именованные виды (после localStorage-прототипа)
```

### 5.1. PostgreSQL, не SQLite

- Несколько реплик karma за одним ingress — нормальная схема деплоя.
- Уникальность «один open-тикет на identity» должна быть constraint в БД, не «надеемся на одного лидера».
- Качество = SQL-агрегации по эпизодам, не JSON-файлы.

SQLite допустим только если позже появится жёсткий single-instance режим. В v1 не делаем два драйвера.

### 5.2. Как пишем историю

В существующий poll Alertmanager (`alertmanager.Interval`) после сборки групп:

1. Для каждого firing-алерта вычислить `identity_key`.
2. Upsert `alert_identity`, открыть/продлить `alert_episode` (`last_seen = now`).
3. Identity, которые были firing в прошлом цикле и пропали дольше чем `2 * interval` — закрыть эпизод (`resolved`).
4. Grace в 2 интервала, чтобы не плодить эпизоды на сетевых блинках AM.

Это **не webhook receiver**. Webhook от AM был бы точнее по timestamps, но это второй процесс, отдельный ingress, и он не видит дедуп/кластеры, которые karma уже умеет. v1 — recorder внутри karma. Webhook — отдельная подзадача, если timestamps poll-цикла окажутся слишком грубыми.

### 5.3. Схема (черновик)

```sql
CREATE TABLE alert_identity (
  identity_key   TEXT PRIMARY KEY,          -- sha256 канонических лейблов
  labels         JSONB NOT NULL,
  alertname      TEXT NOT NULL,
  first_seen_at  TIMESTAMPTZ NOT NULL,
  last_seen_at   TIMESTAMPTZ NOT NULL
);

CREATE TABLE alert_episode (
  id             BIGSERIAL PRIMARY KEY,
  identity_key   TEXT NOT NULL REFERENCES alert_identity(identity_key),
  started_at     TIMESTAMPTZ NOT NULL,
  ended_at       TIMESTAMPTZ,               -- NULL = ещё firing
  last_seen_at   TIMESTAMPTZ NOT NULL,
  peak_severity  TEXT,
  UNIQUE (identity_key, started_at)
);

CREATE TABLE alert_event (
  id             BIGSERIAL PRIMARY KEY,
  identity_key   TEXT NOT NULL,
  episode_id     BIGINT REFERENCES alert_episode(id),
  ts             TIMESTAMPTZ NOT NULL,
  kind           TEXT NOT NULL,             -- fired|resolved|silenced|ticket_open|ticket_close
  payload        JSONB
);

CREATE TABLE ticket (
  id             BIGSERIAL PRIMARY KEY,
  identity_key   TEXT NOT NULL REFERENCES alert_identity(identity_key),
  episode_id     BIGINT REFERENCES alert_episode(id),
  provider       TEXT NOT NULL DEFAULT 'eva',
  task_id        TEXT NOT NULL,
  task_code      TEXT NOT NULL,
  url            TEXT NOT NULL,
  status         TEXT NOT NULL,             -- open|resolved|unknown
  status_raw     TEXT,                      -- cache_status_type / имя статуса EVA
  created_by     TEXT,
  created_at     TIMESTAMPTZ NOT NULL,
  updated_at     TIMESTAMPTZ NOT NULL,
  UNIQUE (provider, task_code)
);

-- один открытый тикет на identity
CREATE UNIQUE INDEX ticket_one_open
  ON ticket (identity_key)
  WHERE status = 'open' AND provider = 'eva';
```

Индексы под UI: `(identity_key, started_at DESC)` на episode, `(ts DESC)` на event, GIN по `labels`.

Retention: события сырые 90 дней, эпизоды 13 месяцев (для годовой оценки качества). Настраивается.

---

## 6. Стабильный identity

Ключ **не** равен AM fingerprint и не равен hash всех лейблов.

```
canonical_labels = labels
  − strip (instance, pod, container, uid, __name__, и конфиг)
  ∪ keep, если keep задан явно (тогда keep побеждает)
canonical_json   = отсортированный JSON
identity_key     = sha256(canonical_json)
```

Конфиг:

```yaml
identity:
  keep: []                 # если непусто — только эти лейблы
  strip:
    - instance
    - pod
    - container
    - uid
```

Рекомендация v1: **strip-список**, не keep. Keep слишком легко забыть `env`/`cluster` и склеить prod со stage.

Текущий firing матчим так: считаем identity_key у живых алертов → JOIN `ticket` / last episode. В API `/alerts.json` добавляем блок:

```json
"ops": {
  "identityKey": "…",
  "episodeStartedAt": "…",
  "ticket": {
    "code": "OPS-1421",
    "url": "https://eva.example/OPS-1421",
    "status": "open"
  }
}
```

`ops` клеим на бэкенде, UI не ходит в БД сам.

---

## 7. EVA

Клиент: [`github.com/raoptimus/evateamclient.go`](https://github.com/raoptimus/evateamclient.go) (`evateamclient.NewClient`).

Используемые методы:

| Действие | API клиента |
|---|---|
| Создать задачу | `TaskCreate` (`kwargs` + `parent` = проект, без `id`) |
| Прочитать | `Task(ctx, code, fields)` |
| Список/поиск | `Tasks(ctx, kwargs)` с `filter` |
| Статус | `cache_status_type` (`OPEN` / closed) + при необходимости `TaskStatusHistory` |
| Ссылка между тикетами | `TaskLinkCreate` — не в v1 |

### 7.1. Конфиг

```yaml
eva:
  enabled: false
  baseURL: https://eva.example.com
  token: ${EVA_API_TOKEN}          # service account, не персональный
  timeout: 15s
  project: OPS                     # code проекта
  logicType: ""                    # если нужен подтип (bug/task)
  taskName: '{{ .AlertName }} {{ .Labels.severity }} {{ .Labels.env }}'
  taskDescription: |
    identity: {{ .IdentityKey }}
    {{ range .Annotations }}**{{ .Name }}**: {{ .Value }}
    {{ end }}
  status:
    open: ["OPEN"]                 # cache_status_type
    resolved: ["CLOSED", "DONE"]   # уточняется по инстансу EVA
  syncInterval: 1m
  uiURL: https://eva.example.com/task/{{ .Code }}
```

Токен — **service account** в конфиге karma. Авторство пишем в описание/поле задачи из уже существующей auth karma (header/basic). Персональные EVA-токены пользователей в v1 не тащим: это SSO-проект сам по себе.

### 7.2. Создание тикета

`POST /tickets.json`

1. Посчитать `identity_key` из тела (лейблы алерта).
2. Если есть open ticket → вернуть его, HTTP 200, `created: false`.
3. Иначе `TaskCreate` в EVA.
4. Insert в `ticket` (уникальный индекс ловит гонку двух реплик).
5. Event `ticket_open`.

UI: пункт в `AlertMenu` + кнопка на полоске. Если тикет open — кнопка «Открыть в EVA», не «Создать».

### 7.3. Sync статуса

Фоновый воркер раз в `syncInterval`:

- берёт все `ticket.status = 'open'`,
- батчит `Tasks` / `Task` по code,
- мапит `cache_status_type` → `open|resolved|unknown`,
- если resolved — event `ticket_close`, эпизод **не** закрываем (алерт может ещё firing).

Не дёргать EVA на каждый `/alerts.json`. Join идёт из локальной таблицы.

### 7.4. Что считаем «решён» для оператора

Два независимых флага на полоске:

- **алерт**: firing / resolved (из AM + наших эпизодов),
- **тикет**: open / resolved (из EVA).

«Закрыли тикет, алерт жив» — валидное состояние, подсвечиваем. «Алерт погас, тикет open» — тоже, это хвост в EVA.

Автозакрытие тикета в EVA из karma в v1 **не делаем**.

---

## 8. Компактный список (полоски)

Сейчас: `AlertGroup` = `card` + `bricks.js` masonry.

Цель: density `compact` (default) | `cards`.

Compact:

- убрать masonry (`useGrid`/`bricks.js` не вызываются),
- группа — секция на 100% ширины, не плитка,
- каждый алерт — одна строка ~32–40px,
- слева цветовая полоска state (`BorderClassMap`),
- в строке: `alertname`, 2–4 ключевых лейбла (`labels:order` / `valueOnly`), возраст, silence icon, EVA badge, меню,
- annotations / links / history sparkline — по раскрытию строки или группы,
- swimlane multi-grid **оставляем**: это и есть нарезка по окружениям.

`groupWidth` в compact игнорируется. Настройка «Minimal group width» живёт только в `cards`.

Это самый заметный UX-дифф и единственный кусок, который можно смержить до БД. Слоты под EVA badge и age рисуем сразу, даже если бэкенд ещё отдаёт `ticket: null`.

---

## 9. Наборы лейблов / виды окружений

Это **именованные views**, не «ещё один savedFilters».

```ts
type ViewPreset = {
  id: string;
  name: string;            // "Prod", "Stage+DB"
  filters: string[];       // karma filter expressions, env=prod
  gridLabel: string;       // cluster | env | @auto | ""
  includeValues?: string[]; // whitelist значений gridLabel; пусто = все
};
```

v1 UX:

- dropdown рядом с фильтрами: список пресетов, save current, overwrite, delete;
- `includeValues` прячет swimlane, которых нет в списке (остальные алерты не показываем, это фильтр, не только группировка);
- дефолтный пресет можно назначить.

Хранение:

1. Сначала `localStorage` (массив, миграция с текущего единственного `savedFilters`).
2. Как только есть Postgres — таблица `view_preset`, shared на всех. localStorage остаётся override «мой последний выбранный id».

Серверные пресеты важнее персональных: дежурство смотрит одни и те же «Prod / Stage».

---

## 10. Качество алерта (после истории)

Не отдельный продукт в v1, но схема сразу должна это выдерживать.

Считаем по `alert_episode` + `ticket`:

- `fire_count_7d` / `30d`
- `mttr` = avg(`ended_at - started_at`) по закрытым эпизодам
- `mtta` = avg(время до `ticket_open` от `started_at`), только эпизоды с тикетом
- `ticket_coverage` = доля эпизодов с тикетом
- `flap` = эпизоды короче N минут / все эпизоды
- `open_without_ticket` = текущие firing без open ticket

Отдать:

- бейдж на полоске (опционально, 7d count),
- модалка «история этого identity» (лента эпизодов + тикеты),
- позже — `/quality.json` / отдельная страница топа шумных алертов.

Текущий Prometheus sparkline можно оставить в модалке как «что видел Prom за 24h».

---

## 11. API (добавки к текущим)

Существующие: `POST /alerts.json`, `POST /history.json`, silences, autocomplete.

Новые:

| Метод | Назначение |
|---|---|
| `GET /ops.json?id=` | эпизоды + события + тикеты по identity |
| `POST /tickets.json` | создать или вернуть open EVA task |
| `GET /views.json` | список пресетов (когда серверные) |
| `PUT /views.json` | создать/обновить пресет |
| `DELETE /views.json?id=` | удалить |
| `GET /quality.json` | агрегаты (фаза 4) |

`/alerts.json` расширяем полем `ops`, без нового round-trip на каждый ряд.

Auth: те же header/basic. Создание тикета требует authenticated user, `created_by` пишем из него. Health/metrics по-прежнему bypass.

---

## 12. Конфиг (сводка новых секций)

```yaml
store:
  enabled: false
  postgres: postgres://karma:@db:5432/karma?sslmode=require
  retention:
    events: 2160h      # 90d
    episodes: 9480h    # ~13m

identity:
  strip: [instance, pod, container, uid]

eva:
  enabled: false
  baseURL: https://eva.example.com
  token: ""
  project: OPS
  # … см. §7.1

ui:
  density: compact     # compact | cards
```

Без `store.enabled` EVA и серверные views не стартуют (явный fail на boot, не тихий no-op). Compact UI от БД не зависит.

---

## 13. Риски

- **Грубые timestamps.** Poll 10–30s → starts/ends с этой дискретностью. Для MTTR дежурства обычно ок. Если нет — webhook receiver (подзадача 2.5).
- **Склейка identity.** Слишком агрессивный strip склеит разные инциденты; слишком слабый — размножит тикеты. Нужен конфиг и один проход по реальному лейблсету до включения EVA.
- **Маппинг статусов EVA.** `cache_status_type` надо сверить с боевым инстансом. Пока unknown — не помечаем resolved.
- **Форк vs upstream.** Persistence + EVA — это уже не drop-in karma. Держим изменения в `internal/ops`, `internal/eva`, `internal/store`, UI-слоты изолированно, чтобы rebase с `prymitive/karma` не был адом.
- **PII в истории.** В БД уйдут лейблы. Retention и доступ к `/ops.json` — те же ACL, что у UI. В EVA description не класть сырые секреты из annotations.
- **Нагрузка.** Recorder O(alerts) на каждый poll — дешёвый. EVA sync O(open tickets). History sparkline не трогаем.

---

## 14. Подзадачи

Нумерация — будущие issue. Зависимости указаны явно.

### Фаза 0 — согласование (этот RFC)

- [ ] **0.1** Зафиксировать identity: strip vs keep, список лейблов на боевом AM.
- [ ] **0.2** Зафиксировать EVA: `baseURL`, проект, logic type, маппинг OPEN/CLOSED.
- [ ] **0.3** Зафиксировать density default = compact, cards остаётся.

### Фаза 1 — compact UI (без БД)

Независимо от 2–4.

- [ ] **1.1** `ui.density` + setting в MainModal, persist в `localStorage`.
- [ ] **1.2** Row-layout группы: полная ширина, секция вместо `card`+masonry.
- [ ] **1.3** Полоска алерта: state bar, alertname, key labels, age, silence, меню.
- [ ] **1.4** Раскрытие строки: annotations, links, sparkline 24h.
- [ ] **1.5** Слоты EVA badge и episode age (пусто, пока нет бэкенда).
- [ ] **1.6** Тесты Grid/AlertGroup + e2e visual compact vs cards.
- [ ] **1.7** Не ломать `cards`: `bricks.js` только в этом режиме.

### Фаза 2 — named views

1.x желателен, но не блокер.

- [ ] **2.1** Модель `ViewPreset`, миграция с одиночного `savedFilters`.
- [ ] **2.2** UI: dropdown пресетов, save/overwrite/delete.
- [ ] **2.3** `includeValues` для gridLabel (показывать только выбранные окружения).
- [ ] **2.4** Серверные пресеты (`/views.json`) — после 3.1.

### Фаза 3 — persistence + recorder

Блокер для EVA и качества.

- [ ] **3.1** `store` пакет: Postgres, миграции, boot fail если `enabled` и нет DSN.
- [ ] **3.2** Таблицы identity / episode / event + индексы + retention job.
- [ ] **3.3** Identity-key: strip/keep, тесты на склейку/разделение.
- [ ] **3.4** Recorder в poll-цикле: open/extend/close эпизодов, grace 2 интервала.
- [ ] **3.5** Вшить `ops` в `/alerts.json` (join ticket, когда появится 4.x).
- [ ] **3.6** `GET /ops.json` — лента эпизодов.
- [ ] **3.7** Метрики recorder: episodes open, events/s, lag, errors.

### Фаза 4 — EVA

Нужны 3.1–3.5.

- [ ] **4.1** Обёртка над `evateamclient.go`: client, config, slog, timeout.
- [ ] **4.2** Шаблоны name/description, `created_by` из karma auth.
- [ ] **4.3** `POST /tickets.json` + unique open-ticket + идемпотентность.
- [ ] **4.4** UI: «Создать задачу» / «Открыть в EVA» на полоске и в AlertMenu.
- [ ] **4.5** Sync-воркер статусов, маппинг open/resolved/unknown.
- [ ] **4.6** Бейдж open/resolved + расхождение «тикет закрыт, алерт жив».
- [ ] **4.7** Фильтр `@ticket=open|none|OPS-123` (по аналогии с `@silence_ticket`).
- [ ] **4.8** Тесты на гонку двух реплик (unique index) и на EVA 5xx (тикет не помечаем созданным).

### Фаза 5 — качество

Нужны 3.x, лучше после 4.x.

- [ ] **5.1** Агрегаты 7d/30d на identity (SQL).
- [ ] **5.2** Модалка истории identity: эпизоды, тикеты, sparkline Prom.
- [ ] **5.3** `GET /quality.json` + топ шумных / без тикета.
- [ ] **5.4** (опционально) webhook receiver, если poll-timestamps недостаточны.

---

## 15. Рекомендуемый порядок мержа

```
1.1–1.7 compact     ─┐
2.1–2.3 views local ─┴─► можно в прод без Postgres
3.1–3.7 store
   └─► 2.4 server views
   └─► 4.1–4.8 EVA
         └─► 5.1–5.3 quality
```

Первый пользовательский выигрыш — полоски и пресеты окружений. EVA без истории в БД делать не стоит: после рестарта «открыт ли тикет» превратится в гадание по EVA search.

---

## 16. Сознательно не в v1

- Автосоздание тикета на каждый firing.
- Автозакрытие тикета в EVA, когда алерт resolved.
- Персональные EVA OAuth-токены пользователей.
- SQLite dual-backend.
- Полноценный webhook receiver (только если 3.4 не хватит).
- Замена Prometheus sparkline — он остаётся дополнительным.
- Редактор произвольных полей EVA (спринт, исполнитель) — только шаблон + проект.
- Переписывание silence ACL / kthxbye ack.

---

## 17. Открытые вопросы (без них не начинать 3.x/4.x)

1. **Identity.** Какие лейблы на боевых алертах надо strip-ать, чтобы «тот же» инцидент с новым подом оставался одним тикетом, а prod/stage не склеились? Нужен дамп 20–30 реальных labelset.
2. **EVA.** Код проекта, logic type, и какие `cache_status_type` (или кастомные статусы) считать resolved. Без этого sync нарисует `unknown` на всём.
3. **Где крутится Postgres.** Уже есть инстанс, куда можно положить БД `karma`, или это новый сервис в том же namespace? От этого зависит, можно ли фазу 3 планировать сразу после compact.

Пока нет ответа на (1)–(2), фазы 1–2 всё равно можно делать.
