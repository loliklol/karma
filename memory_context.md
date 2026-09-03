# memory_context

> Живой контекст диалога для агента. Обновлять при каждом значимом ходе.
> Блокирующий артефакт: без актуального memory_context не продолжать работу «вслепую».

## Мета

| Поле | Значение |
|------|----------|
| Repo | `github.com/loliklol/karma` (форк prymitive/karma) |
| Branch design | `cursor/alert-history-eva-design-2bad` |
| Режим | проектирование (код фич — после явного старта реализации) |

## Запрос (суть)

Оценка репо + предложения: история алертов, EVA тикеты, пресеты окружений, strips UI.  
Клиент: `https://github.com/raoptimus/evateamclient.go`. Пока не реализовывать.

## ADR — зафиксировано

| # | Решение |
|---|---------|
| БД | **PostgreSQL сразу** (SQLite не делаем) |
| EVA | **Picker в UI**: project или Service Desk. `defaultTarget` + optional label routes как preselect. SD в либе = обычный project (`TaskCreate` + `parent`); отдельного ServiceDesk API нет |
| Пресеты | **Shared team-wide с v1** (Postgres `ui_preset`), не personal localStorage first |

## Артефакт

`docs/proposals/2026-09-alert-ops-roadmap.md` — обновлён под ADR.

## Эпики / порядок

- **D** strips (можно без БД)
- **A** Postgres store — foundation
- **C** shared presets (после A1)
- **B** EVA picker + create/sync (после A1)
- **E** аналитика

Порядок: `D ∥ A → C∥events → B → E`

## Следующий ход

Ждать старт реализации / какой эпик первым. Кандидаты:
1. D1–D3 strips (быстрый UX)
2. A1–A2 Postgres foundation (нужен для C/B)

Мелочи при старте: коды проектов OPS/SD, force второго тикета, auth → `created_by`.
