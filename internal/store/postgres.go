package store

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// PostgresTicketStore stores tickets in PostgreSQL.
type PostgresTicketStore struct {
	pool *pgxpool.Pool
}

func NewPostgresTicketStore(ctx context.Context, dsn string, maxOpenConns int) (*PostgresTicketStore, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	if maxOpenConns > 0 {
		cfg.MaxConns = int32(maxOpenConns)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	s := &PostgresTicketStore{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *PostgresTicketStore) migrate(ctx context.Context) error {
	sqlBytes, err := migrationsFS.ReadFile("migrations/001_alert_ticket.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if _, err := s.pool.Exec(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}
	return nil
}

func (s *PostgresTicketStore) Close() error {
	s.pool.Close()
	return nil
}

func (s *PostgresTicketStore) FindOpenByIdentity(ctx context.Context, identityKey string) (*Ticket, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, identity_key, group_id, provider, project_code, project_id,
		       external_id, external_code, url, status, created_by, created_at, updated_at
		FROM alert_ticket
		WHERE identity_key = $1 AND status = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, identityKey, StatusOpen)
	return scanTicket(row)
}

func (s *PostgresTicketStore) ListByGroupID(ctx context.Context, groupID string) ([]Ticket, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, identity_key, group_id, provider, project_code, project_id,
		       external_id, external_code, url, status, created_by, created_at, updated_at
		FROM alert_ticket
		WHERE group_id = $1
		ORDER BY created_at DESC
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Ticket, 0)
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		if t != nil {
			out = append(out, *t)
		}
	}
	return out, rows.Err()
}

func (s *PostgresTicketStore) Insert(ctx context.Context, ticket *Ticket) error {
	if ticket.Provider == "" {
		ticket.Provider = ProviderEVA
	}
	now := time.Now().UTC()
	if ticket.CreatedAt.IsZero() {
		ticket.CreatedAt = now
	}
	ticket.UpdatedAt = now
	err := s.pool.QueryRow(ctx, `
		INSERT INTO alert_ticket (
			identity_key, group_id, provider, project_code, project_id,
			external_id, external_code, url, status, created_by, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id
	`,
		ticket.IdentityKey, ticket.GroupID, ticket.Provider, ticket.ProjectCode, ticket.ProjectID,
		ticket.ExternalID, ticket.ExternalCode, ticket.URL, ticket.Status, ticket.CreatedBy,
		ticket.CreatedAt, ticket.UpdatedAt,
	).Scan(&ticket.ID)
	return err
}

func (s *PostgresTicketStore) UpdateStatus(ctx context.Context, provider, externalID, status string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE alert_ticket
		SET status = $1, updated_at = $2
		WHERE provider = $3 AND external_id = $4
	`, status, time.Now().UTC(), provider, externalID)
	return err
}

type scannable interface {
	Scan(dest ...any) error
}

func scanTicket(row scannable) (*Ticket, error) {
	var t Ticket
	err := row.Scan(
		&t.ID, &t.IdentityKey, &t.GroupID, &t.Provider, &t.ProjectCode, &t.ProjectID,
		&t.ExternalID, &t.ExternalCode, &t.URL, &t.Status, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}
