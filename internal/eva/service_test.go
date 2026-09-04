package eva

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/raoptimus/evateamclient.go"
	"github.com/raoptimus/evateamclient.go/models"

	"github.com/prymitive/karma/internal/config"
	"github.com/prymitive/karma/internal/store"
)

type mockTeamClient struct {
	project *models.Project
	task    *models.Task
	err     error
}

func (m *mockTeamClient) Project(context.Context, string, []string) (*models.Project, *models.Meta, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return m.project, &models.Meta{}, nil
}

func (m *mockTeamClient) TaskCreate(context.Context, *evateamclient.TaskCreateParams) (*models.Task, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.task, nil
}

func testService(t *testing.T, client TeamClient, tickets store.TicketStore) *Service {
	t.Helper()
	cfg := config.EvaConfig{
		Enabled:         true,
		WebBaseURL:      "https://eva.example",
		TaskURLTemplate: "{{ .WebBaseURL }}/task/{{ .Code }}",
		DefaultTarget:   "OPS",
		IdentityLabels:  []string{"alertname", "cluster"},
		Targets: []config.EvaTarget{
			{Code: "OPS", Label: "Ops", Kind: "project"},
			{Code: "SD", Label: "Service Desk", Kind: "servicedesk"},
		},
		Task: config.EvaTaskTemplates{
			NameTemplate: "[{{ .Alertname }}]",
			TextTemplate: "body {{ index .Labels \"cluster\" }}",
			Tags:         []string{"karma"},
		},
	}
	return NewService(cfg, client, tickets)
}

func TestServiceCreateAndDedup(t *testing.T) {
	client := &mockTeamClient{
		project: &models.Project{ID: "CmfProject:1", Code: "OPS"},
		task:    &models.Task{ID: "CmfTask:1", Code: "OPS-1"},
	}
	mem := store.NewMemoryTicketStore()
	svc := testService(t, client, mem)

	req := CreateRequest{
		GroupID: "gid-1",
		GroupLabels: labels.FromMap(map[string]string{
			"alertname": "DiskFull",
			"cluster":   "prod",
		}),
		Receiver:  "by-name",
		StartsAt:  time.Now(),
		Target:    "OPS",
		CreatedBy: "alice",
	}

	view, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Created || view.Code != "OPS-1" || view.URL != "https://eva.example/task/OPS-1" {
		t.Fatalf("unexpected view: %+v", view)
	}

	_, err = svc.Create(context.Background(), req)
	if !errors.Is(err, ErrOpenExists) {
		t.Fatalf("expected ErrOpenExists, got %v", err)
	}

	client.task = &models.Task{ID: "CmfTask:2", Code: "OPS-2"}
	req.Force = true
	view, err = svc.Create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if view.Code != "OPS-2" {
		t.Fatalf("force create failed: %+v", view)
	}
}

func TestServiceUnknownTarget(t *testing.T) {
	svc := testService(t, &mockTeamClient{}, store.NewMemoryTicketStore())
	_, err := svc.Create(context.Background(), CreateRequest{
		GroupID:     "g",
		GroupLabels: labels.FromMap(map[string]string{"alertname": "X"}),
		Target:      "NOPE",
	})
	if !errors.Is(err, ErrUnknownTarget) {
		t.Fatalf("got %v", err)
	}
}
