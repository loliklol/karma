package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/raoptimus/evateamclient.go"
	"github.com/raoptimus/evateamclient.go/models"

	"github.com/prymitive/karma/internal/config"
	"github.com/prymitive/karma/internal/eva"
	"github.com/prymitive/karma/internal/store"
)

type stubTeamClient struct{}

func (stubTeamClient) Project(context.Context, string, []string) (*models.Project, *models.Meta, error) {
	return &models.Project{ID: "CmfProject:1", Code: "OPS"}, &models.Meta{}, nil
}

func (stubTeamClient) TaskCreate(context.Context, *evateamclient.TaskCreateParams) (*models.Task, error) {
	return &models.Task{ID: "CmfTask:9", Code: "OPS-9"}, nil
}

func TestEvaTargetsDisabled(t *testing.T) {
	evaService = nil
	r := httptest.NewRequest(http.MethodGet, "/eva/targets.json", nil)
	w := httptest.NewRecorder()
	evaTargets(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestEvaCreateRequiresGroup(t *testing.T) {
	mem := store.NewMemoryTicketStore()
	evaService = eva.NewService(config.EvaConfig{
		Enabled:         true,
		WebBaseURL:      "https://eva.example",
		TaskURLTemplate: "{{ .WebBaseURL }}/task/{{ .Code }}",
		DefaultTarget:   "OPS",
		IdentityLabels:  []string{"alertname"},
		Targets:         []config.EvaTarget{{Code: "OPS", Label: "Ops", Kind: "project"}},
		Task: config.EvaTaskTemplates{
			NameTemplate: "[{{ .Alertname }}]",
			TextTemplate: "body",
		},
	}, stubTeamClient{}, mem)

	body, _ := json.Marshal(evaCreateRequest{GroupID: "", Target: "OPS"})
	r := httptest.NewRequest(http.MethodPost, "/eva/tasks.json", bytes.NewReader(body))
	w := httptest.NewRecorder()
	evaCreateTask(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestEvaListTasks(t *testing.T) {
	mem := store.NewMemoryTicketStore()
	_ = mem.Insert(context.Background(), &store.Ticket{
		IdentityKey:  "abc",
		GroupID:      "g1",
		Provider:     store.ProviderEVA,
		ProjectCode:  "OPS",
		ProjectID:    "CmfProject:1",
		ExternalID:   "CmfTask:1",
		ExternalCode: "OPS-1",
		URL:          "https://eva.example/task/OPS-1",
		Status:       store.StatusOpen,
		CreatedAt:    time.Now(),
	})
	evaService = eva.NewService(config.EvaConfig{
		Enabled: true,
		Targets: []config.EvaTarget{{Code: "OPS", Label: "Ops", Kind: "project"}},
	}, stubTeamClient{}, mem)

	r := httptest.NewRequest(http.MethodGet, "/eva/tasks.json?groupId=g1", nil)
	w := httptest.NewRecorder()
	evaListTasks(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp evaTasksResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Tickets) != 1 || resp.Tickets[0].Code != "OPS-1" {
		t.Fatalf("unexpected %+v", resp)
	}
}
