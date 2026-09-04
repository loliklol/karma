package store

import (
	"context"
	"time"
)

const (
	StatusOpen    = "open"
	StatusClosed  = "closed"
	StatusUnknown = "unknown"
	ProviderEVA   = "eva"
)

// Ticket is a persisted link between a karma alert identity and an external task.
type Ticket struct {
	ID           int64
	IdentityKey  string
	GroupID      string
	Provider     string
	ProjectCode  string
	ProjectID    string
	ExternalID   string
	ExternalCode string
	URL          string
	Status       string
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TicketStore persists alert↔ticket mappings.
type TicketStore interface {
	Close() error
	FindOpenByIdentity(ctx context.Context, identityKey string) (*Ticket, error)
	ListByGroupID(ctx context.Context, groupID string) ([]Ticket, error)
	Insert(ctx context.Context, ticket *Ticket) error
	UpdateStatus(ctx context.Context, provider, externalID, status string) error
}
