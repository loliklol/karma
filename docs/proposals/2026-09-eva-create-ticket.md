# EVA: создание тикета из алерта (детальный срез v1)

**Статус:** в реализации  
**Зависит от ADR:** Postgres, picker project/SD, shared presets (пресеты — отдельно)  
**Клиент:** `github.com/raoptimus/evateamclient.go` v1.5.3+

## Цель v1

Из UI karma создать задачу в EVA для alert group, выбрать target (project / Service Desk), сохранить связь в Postgres, показать ссылку/статус. Не создавать второй open-тикет на тот же identity (кроме `force`).

## User flow

1. В меню группы → **Create EVA ticket**.
2. Модалка: select target (из `eva.targets`), preselect = route match || `defaultTarget`.
3. Submit → `POST /eva/tasks.json`.
4. Успех: бейдж/ссылка на задачу; ошибка: текст в модалке.
5. Если уже есть open ticket → 409 + существующий тикет (UI предлагает Open / Force).

## API

### `GET /eva/targets.json`

Список allowlist для picker (без секретов).

```json
{
  "enabled": true,
  "defaultTarget": "OPS",
  "targets": [
    { "code": "OPS", "label": "Ops board", "kind": "project" },
    { "code": "SD", "label": "Service Desk", "kind": "servicedesk" }
  ]
}
```

404/400 если `eva.enabled=false`.

### `POST /eva/tasks.json`

Request:

```json
{
  "groupId": "<AlertGroup.ID / LabelsFingerprint>",
  "target": "OPS",
  "force": false
}
```

Response 200:

```json
{
  "created": true,
  "ticket": {
    "code": "OPS-123",
    "id": "CmfTask:...",
    "url": "https://eva.example/.../OPS-123",
    "projectCode": "OPS",
    "status": "open",
    "identityKey": "sha256:..."
  }
}
```

Response 409 (уже open, `force=false`):

```json
{
  "created": false,
  "error": "open ticket already exists",
  "ticket": { "...": "existing" }
}
```

### `GET /eva/tasks.json?groupId=...`

Текущие тикеты для identity группы (для бейджа).

## Identity

```
identityKey = sha256( sorted join of identityLabels: "k=v\n" )
```

`eva.identityLabels` default: `["alertname"]` + group labels values for listed keys.  
Если label отсутствует — пропускаем ключ (не падаем).  
Минимум один ключ должен дать значение, иначе 400.

Также храним `group_id` (karma group fingerprint) для UI lookup.

## Создание в EVA

1. Resolve `target` → project via `client.Project(ctx, code, ...)`.
2. Render `nameTemplate` / `textTemplate` (`text/template`) с данными группы.
3. `TaskCreate({ Name, ProjectID: project.ID|code, Text, Tags })`.
4. Insert `alert_ticket`.
5. URL: `{webBaseURL}` + template или `{webBaseURL}/task/{code}` (конфиг `eva.taskURLTemplate`).

## Конфиг

```yaml
persistence:
  dsn: "${KARMA_DATABASE_URL}"   # required if eva.enabled
  maxOpenConns: 10

eva:
  enabled: true
  baseURL: https://api.eva.team
  apiToken: "${EVA_TOKEN}"
  webBaseURL: https://eva.example
  taskURLTemplate: "{{ .WebBaseURL }}/task/{{ .Code }}"
  timeout: 15s
  defaultTarget: OPS
  identityLabels: [alertname, cluster, service]
  targets:
    - code: OPS
      label: Ops board
      kind: project
    - code: SD
      label: Service Desk
      kind: servicedesk
  routes:
    - match: { label: severity, value: critical }
      target: SD
  task:
    nameTemplate: '[{{ .Alertname }}] {{ index .Labels "cluster" }}'
    textTemplate: |
      Alert group in karma.

      {{ range $k, $v := .Labels }}- {{ $k }}: {{ $v }}
      {{ end }}
    tags: [karma, alert]
```

## Схема Postgres (минимум)

```sql
CREATE TABLE alert_ticket (
  id            BIGSERIAL PRIMARY KEY,
  identity_key  TEXT NOT NULL,
  group_id      TEXT NOT NULL,
  provider      TEXT NOT NULL DEFAULT 'eva',
  project_code  TEXT NOT NULL,
  project_id    TEXT NOT NULL,
  external_id   TEXT NOT NULL,
  external_code TEXT NOT NULL,
  url           TEXT NOT NULL,
  status        TEXT NOT NULL, -- open|closed|unknown
  created_by    TEXT NOT NULL DEFAULT '',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX alert_ticket_identity_status_idx ON alert_ticket (identity_key, status);
CREATE INDEX alert_ticket_group_id_idx ON alert_ticket (group_id);
CREATE UNIQUE INDEX alert_ticket_external_id_uidx ON alert_ticket (provider, external_id);
```

## Пакеты

| Пакет | Роль |
|-------|------|
| `internal/store` | TicketStore interface, Postgres, Memory (тесты) |
| `internal/eva` | Client wrapper, templates, Create/Dedup service |
| `cmd/karma/eva_*.go` | HTTP handlers, wiring |
| UI `Components/EvaTicket` | Modal + menu entry + badge |

## Settings → UI

В `/alerts.json` → `settings.eva`:

```json
{
  "enabled": true,
  "defaultTarget": "OPS",
  "targets": [ ... ]
}
```

Routes считаются на клиенте при открытии модалки (labels группы → preselect).

## Тесты (минимум)

- identity key stable / missing labels
- template render
- create happy path (mock EVA + memory store)
- dedup 409 without force; create with force
- handler 400 when disabled / unknown target / group not found
- UI: menu hidden when disabled; modal submit

## Out of scope этого среза

- Background sync статусов (следующий кусок B6)
- Обогащение всех alerts в `/alerts.json` batch tickets
- Фильтры `@eva_*`
- Shared presets / strips / alert event history
- ACL на create (пока любой auth user; anonym — пустой created_by)

## Порядок коммитов реализации

1. config + store + eva service (без UI)
2. HTTP API + wiring + tests
3. UI modal/menu
4. docs CONFIGURATION.md
