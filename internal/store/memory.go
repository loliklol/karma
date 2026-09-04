package store

import (
	"context"
	"sync"
	"time"
)

// MemoryTicketStore is an in-memory TicketStore for tests.
type MemoryTicketStore struct {
	mu      sync.Mutex
	nextID  int64
	tickets []Ticket
}

func NewMemoryTicketStore() *MemoryTicketStore {
	return &MemoryTicketStore{nextID: 1}
}

func (s *MemoryTicketStore) Close() error { return nil }

func (s *MemoryTicketStore) FindOpenByIdentity(ctx context.Context, identityKey string) (*Ticket, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tickets {
		t := s.tickets[i]
		if t.IdentityKey == identityKey && t.Status == StatusOpen {
			cp := t
			return &cp, nil
		}
	}
	return nil, nil
}

func (s *MemoryTicketStore) ListByGroupID(ctx context.Context, groupID string) ([]Ticket, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Ticket, 0)
	for _, t := range s.tickets {
		if t.GroupID == groupID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (s *MemoryTicketStore) Insert(ctx context.Context, ticket *Ticket) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	ticket.ID = s.nextID
	s.nextID++
	if ticket.Provider == "" {
		ticket.Provider = ProviderEVA
	}
	if ticket.CreatedAt.IsZero() {
		ticket.CreatedAt = now
	}
	ticket.UpdatedAt = now
	cp := *ticket
	s.tickets = append(s.tickets, cp)
	return nil
}

func (s *MemoryTicketStore) UpdateStatus(ctx context.Context, provider, externalID, status string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tickets {
		if s.tickets[i].Provider == provider && s.tickets[i].ExternalID == externalID {
			s.tickets[i].Status = status
			s.tickets[i].UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return nil
}
