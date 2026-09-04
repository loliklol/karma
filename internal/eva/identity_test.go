package eva

import (
	"testing"

	"github.com/prometheus/prometheus/model/labels"
)

func TestIdentityKey(t *testing.T) {
	ls := labels.FromMap(map[string]string{
		"alertname": "DiskFull",
		"cluster":   "prod",
		"instance":  "a",
	})
	k1, err := IdentityKey([]string{"alertname", "cluster"}, ls)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := IdentityKey([]string{"cluster", "alertname"}, ls)
	if err != nil {
		t.Fatal(err)
	}
	if k1 != k2 {
		t.Fatalf("identity key must be order-independent: %s vs %s", k1, k2)
	}
	if _, err := IdentityKey([]string{"missing"}, ls); err == nil {
		t.Fatal("expected error when no identity labels present")
	}
}

func TestResolveTarget(t *testing.T) {
	routes := []Route{{MatchLabel: "severity", MatchValue: "critical", Target: "SD"}}
	got := ResolveTarget("OPS", routes, map[string]string{"severity": "critical"})
	if got != "SD" {
		t.Fatalf("got %q want SD", got)
	}
	got = ResolveTarget("OPS", routes, map[string]string{"severity": "warning"})
	if got != "OPS" {
		t.Fatalf("got %q want OPS", got)
	}
}

func TestRenderTemplate(t *testing.T) {
	out, err := renderTemplate("name", "[{{ .Alertname }}] {{ index .Labels \"cluster\" }}", TemplateData{
		Alertname: "DiskFull",
		Labels:    map[string]string{"cluster": "prod"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "[DiskFull] prod" {
		t.Fatalf("got %q", out)
	}
}
