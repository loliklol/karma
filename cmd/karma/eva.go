package main

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"time"

	"github.com/prymitive/karma/internal/alertmanager"
	"github.com/prymitive/karma/internal/config"
	"github.com/prymitive/karma/internal/eva"
	"github.com/prymitive/karma/internal/models"
)

var evaService *eva.Service

type evaCreateRequest struct {
	GroupID string `json:"groupId"`
	Target  string `json:"target"`
	Force   bool   `json:"force"`
}

type evaCreateResponse struct {
	Created bool           `json:"created"`
	Error   string         `json:"error,omitempty"`
	Ticket  eva.TicketView `json:"ticket"`
}

type evaTargetsResponse struct {
	Enabled       bool               `json:"enabled"`
	DefaultTarget string             `json:"defaultTarget"`
	Targets       []config.EvaTarget `json:"targets"`
}

type evaTasksResponse struct {
	Tickets []eva.TicketView `json:"tickets"`
}

func buildEvaSettings() models.EvaSettings {
	cfg := config.Config.Eva
	out := models.EvaSettings{
		Enabled:       cfg.Enabled,
		DefaultTarget: cfg.DefaultTarget,
		Targets:       make([]models.EvaTargetSettings, 0, len(cfg.Targets)),
		Routes:        make([]models.EvaRouteSettings, 0, len(cfg.Routes)),
	}
	for _, t := range cfg.Targets {
		out.Targets = append(out.Targets, models.EvaTargetSettings{
			Code:  t.Code,
			Label: t.Label,
			Kind:  t.Kind,
		})
	}
	for _, r := range cfg.Routes {
		out.Routes = append(out.Routes, models.EvaRouteSettings{
			Label:  r.Match.Label,
			Value:  r.Match.Value,
			Target: r.Target,
		})
	}
	return out
}

func findAlertGroup(groupID string) *models.AlertGroup {
	for _, g := range alertmanager.DedupAlerts() {
		if g.ID == groupID {
			cp := g
			return &cp
		}
	}
	return nil
}

func evaTargets(w http.ResponseWriter, _ *http.Request) {
	noCache(w)
	if evaService == nil || !evaService.Enabled() {
		http.Error(w, "eva integration disabled", http.StatusNotFound)
		return
	}
	resp := evaTargetsResponse{
		Enabled:       true,
		DefaultTarget: evaService.DefaultTarget(),
		Targets:       evaService.Targets(),
	}
	data, _ := marshalJSON(resp)
	mimeJSON(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func evaListTasks(w http.ResponseWriter, r *http.Request) {
	noCache(w)
	if evaService == nil || !evaService.Enabled() {
		http.Error(w, "eva integration disabled", http.StatusNotFound)
		return
	}
	groupID := r.URL.Query().Get("groupId")
	if groupID == "" {
		http.Error(w, "groupId is required", http.StatusBadRequest)
		return
	}
	tickets, err := evaService.ListByGroupID(r.Context(), groupID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data, _ := marshalJSON(evaTasksResponse{Tickets: tickets})
	mimeJSON(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func evaCreateTask(w http.ResponseWriter, r *http.Request) {
	noCache(w)
	if evaService == nil || !evaService.Enabled() {
		http.Error(w, "eva integration disabled", http.StatusNotFound)
		return
	}

	var payload evaCreateRequest
	if err := json.UnmarshalRead(r.Body, &payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if payload.GroupID == "" {
		http.Error(w, "groupId is required", http.StatusBadRequest)
		return
	}

	group := findAlertGroup(payload.GroupID)
	if group == nil {
		http.Error(w, "alert group not found", http.StatusNotFound)
		return
	}

	startsAt := time.Now()
	if len(group.Alerts) > 0 {
		startsAt = group.Alerts[0].StartsAt
	}

	view, err := evaService.Create(r.Context(), eva.CreateRequest{
		GroupID:     payload.GroupID,
		GroupLabels: group.Labels,
		Receiver:    group.Receiver,
		StartsAt:    startsAt,
		Target:      payload.Target,
		Force:       payload.Force,
		CreatedBy:   getUserFromContext(r),
	})

	resp := evaCreateResponse{Ticket: view}
	status := http.StatusOK
	switch {
	case errors.Is(err, eva.ErrOpenExists):
		resp.Created = false
		resp.Error = err.Error()
		status = http.StatusConflict
	case errors.Is(err, eva.ErrUnknownTarget):
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	case err != nil:
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	default:
		resp.Created = true
	}

	data, _ := marshalJSON(resp)
	mimeJSON(w)
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
