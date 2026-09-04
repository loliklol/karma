package eva

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/raoptimus/evateamclient.go"

	"github.com/prymitive/karma/internal/config"
	"github.com/prymitive/karma/internal/store"
)

var (
	ErrDisabled      = errors.New("eva integration is disabled")
	ErrUnknownTarget = errors.New("unknown eva target")
	ErrOpenExists    = errors.New("open ticket already exists")
)

// TicketView is API-facing ticket info.
type TicketView struct {
	Code        string `json:"code"`
	ID          string `json:"id"`
	URL         string `json:"url"`
	ProjectCode string `json:"projectCode"`
	Status      string `json:"status"`
	IdentityKey string `json:"identityKey"`
	GroupID     string `json:"groupId"`
	Created     bool   `json:"-"`
}

// CreateRequest is input for creating a ticket from an alert group.
type CreateRequest struct {
	GroupID     string
	GroupLabels labels.Labels
	Receiver    string
	StartsAt    time.Time
	Target      string
	Force       bool
	CreatedBy   string
}

// Service implements EVA ticket create/lookup.
type Service struct {
	cfg     config.EvaConfig
	client  TeamClient
	tickets store.TicketStore
	routes  []Route
}

func NewService(cfg config.EvaConfig, client TeamClient, tickets store.TicketStore) *Service {
	routes := make([]Route, 0, len(cfg.Routes))
	for _, r := range cfg.Routes {
		routes = append(routes, Route{
			MatchLabel: r.Match.Label,
			MatchValue: r.Match.Value,
			Target:     r.Target,
		})
	}
	return &Service{cfg: cfg, client: client, tickets: tickets, routes: routes}
}

func (s *Service) Enabled() bool { return s != nil && s.cfg.Enabled }

func (s *Service) Targets() []config.EvaTarget {
	if s == nil {
		return nil
	}
	return s.cfg.Targets
}

func (s *Service) DefaultTarget() string {
	if s == nil {
		return ""
	}
	return s.cfg.DefaultTarget
}

func (s *Service) SuggestedTarget(ls labels.Labels) string {
	return ResolveTarget(s.cfg.DefaultTarget, s.routes, LabelsMap(ls))
}

func (s *Service) targetAllowed(code string) bool {
	for _, t := range s.cfg.Targets {
		if t.Code == code {
			return true
		}
	}
	return false
}

func ticketViewFromStore(t *store.Ticket) TicketView {
	return TicketView{
		Code:        t.ExternalCode,
		ID:          t.ExternalID,
		URL:         t.URL,
		ProjectCode: t.ProjectCode,
		Status:      t.Status,
		IdentityKey: t.IdentityKey,
		GroupID:     t.GroupID,
	}
}

func (s *Service) ListByGroupID(ctx context.Context, groupID string) ([]TicketView, error) {
	if !s.Enabled() {
		return nil, ErrDisabled
	}
	rows, err := s.tickets.ListByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	out := make([]TicketView, 0, len(rows))
	for i := range rows {
		out = append(out, ticketViewFromStore(&rows[i]))
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (TicketView, error) {
	if !s.Enabled() {
		return TicketView{}, ErrDisabled
	}
	if req.GroupID == "" {
		return TicketView{}, fmt.Errorf("groupId is required")
	}
	target := req.Target
	if target == "" {
		target = s.SuggestedTarget(req.GroupLabels)
	}
	if !s.targetAllowed(target) {
		return TicketView{}, fmt.Errorf("%w: %s", ErrUnknownTarget, target)
	}

	identityKey, err := IdentityKey(s.cfg.IdentityLabels, req.GroupLabels)
	if err != nil {
		return TicketView{}, err
	}

	if !req.Force {
		existing, err := s.tickets.FindOpenByIdentity(ctx, identityKey)
		if err != nil {
			return TicketView{}, err
		}
		if existing != nil {
			view := ticketViewFromStore(existing)
			return view, ErrOpenExists
		}
	}

	labelMap := LabelsMap(req.GroupLabels)
	data := TemplateData{
		Alertname:  labelMap["alertname"],
		Receiver:   req.Receiver,
		Labels:     labelMap,
		StartsAt:   req.StartsAt,
		WebBaseURL: s.cfg.WebBaseURL,
		Project:    target,
	}
	name, err := renderTemplate("name", s.cfg.Task.NameTemplate, data)
	if err != nil {
		return TicketView{}, err
	}
	if name == "" {
		return TicketView{}, errors.New("rendered EVA task name is empty")
	}
	text, err := renderTemplate("text", s.cfg.Task.TextTemplate, data)
	if err != nil {
		return TicketView{}, err
	}

	projectID, err := resolveProjectID(ctx, s.client, target)
	if err != nil {
		return TicketView{}, err
	}

	task, err := s.client.TaskCreate(ctx, &evateamclient.TaskCreateParams{
		Name:      name,
		ProjectID: projectID,
		Text:      text,
		Tags:      append([]string(nil), s.cfg.Task.Tags...),
	})
	if err != nil {
		return TicketView{}, fmt.Errorf("eva TaskCreate: %w", err)
	}
	if task == nil || task.ID == "" {
		return TicketView{}, errors.New("eva TaskCreate returned empty task")
	}

	data.Code = task.Code
	url, err := renderTemplate("url", s.cfg.TaskURLTemplate, data)
	if err != nil {
		return TicketView{}, err
	}
	if url == "" && s.cfg.WebBaseURL != "" && task.Code != "" {
		url = stringsTrimSlash(s.cfg.WebBaseURL) + "/task/" + task.Code
	}

	ticket := &store.Ticket{
		IdentityKey:  identityKey,
		GroupID:      req.GroupID,
		Provider:     store.ProviderEVA,
		ProjectCode:  target,
		ProjectID:    projectID,
		ExternalID:   task.ID,
		ExternalCode: task.Code,
		URL:          url,
		Status:       store.StatusOpen,
		CreatedBy:    req.CreatedBy,
	}
	if err := s.tickets.Insert(ctx, ticket); err != nil {
		return TicketView{}, fmt.Errorf("persist ticket: %w", err)
	}
	view := ticketViewFromStore(ticket)
	view.Created = true
	return view, nil
}

func stringsTrimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
