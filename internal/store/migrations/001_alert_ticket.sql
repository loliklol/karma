CREATE TABLE IF NOT EXISTS alert_ticket (
    id            BIGSERIAL PRIMARY KEY,
    identity_key  TEXT NOT NULL,
    group_id      TEXT NOT NULL,
    provider      TEXT NOT NULL DEFAULT 'eva',
    project_code  TEXT NOT NULL,
    project_id    TEXT NOT NULL,
    external_id   TEXT NOT NULL,
    external_code TEXT NOT NULL,
    url           TEXT NOT NULL,
    status        TEXT NOT NULL,
    created_by    TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS alert_ticket_identity_status_idx
    ON alert_ticket (identity_key, status);

CREATE INDEX IF NOT EXISTS alert_ticket_group_id_idx
    ON alert_ticket (group_id);

CREATE UNIQUE INDEX IF NOT EXISTS alert_ticket_external_id_uidx
    ON alert_ticket (provider, external_id);
