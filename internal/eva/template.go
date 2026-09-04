package eva

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"
)

// TemplateData is available to name/text/url templates.
type TemplateData struct {
	Alertname  string
	Receiver   string
	Labels     map[string]string
	StartsAt   time.Time
	WebBaseURL string
	Code       string
	Project    string
}

func renderTemplate(name, tmpl string, data TemplateData) (string, error) {
	t, err := template.New(name).Option("missingkey=zero").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse %s template: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute %s template: %w", name, err)
	}
	return strings.TrimSpace(buf.String()), nil
}
