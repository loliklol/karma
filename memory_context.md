# memory_context

> Живой контекст диалога для агента. Обновлять при каждом значимом ходе.
> Блокирующий артефакт: без актуального memory_context не продолжать работу «вслепую».

## Мета

| Поле | Значение |
|------|----------|
| Repo | `github.com/loliklol/karma` (форк prymitive/karma) |
| Branch | `cursor/alert-history-eva-design-2bad` |
| Режим | реализация: EVA create ticket (срез B) |

## ADR

| # | Решение |
|---|---------|
| БД | PostgreSQL сразу |
| EVA | Picker project/SD (`TaskCreate` + parent) |
| Пресеты | Shared team-wide (ещё не в этом срезе) |

## Текущий фокус

**Создание тикета в EVA** — детальный срез + код.

Артефакты:
- `docs/proposals/2026-09-eva-create-ticket.md` — детальный дизайн среза
- `docs/proposals/2026-09-alert-ops-roadmap.md` — общий roadmap
- `docs/CONFIGURATION.md` — секции `persistence` / `eva`

Реализовано в коде (WIP push):
- `internal/config` — `eva` + `persistence`
- `internal/store` — TicketStore (memory + postgres)
- `internal/eva` — identity/templates/service + client wrapper (`evateamclient.go` v1.5.3)
- `cmd/karma` — `/eva/targets.json`, `/eva/tasks.json` GET/POST, wiring
- UI — `Components/EvaTicket`, пункт меню группы

Ещё не сделано в этом срезе: status sync, batch enrich alerts.json, `@eva_*` filters, badge на полоске, e2e с живым EVA/Postgres.

## Следующий ход

Добить UI тесты / commit+push; потом sync статусов (B6) или badge.
