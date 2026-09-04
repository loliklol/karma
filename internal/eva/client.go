package eva

import (
	"context"
	"fmt"
	"time"

	"github.com/raoptimus/evateamclient.go"
	"github.com/raoptimus/evateamclient.go/models"
)

// TeamClient is the subset of EVA client used by karma.
type TeamClient interface {
	Project(ctx context.Context, code string, fields []string) (*models.Project, *models.Meta, error)
	TaskCreate(ctx context.Context, params *evateamclient.TaskCreateParams) (*models.Task, error)
}

// RealClient wraps evateamclient.Client.
type RealClient struct {
	inner *evateamclient.Client
}

func NewRealClient(baseURL, apiToken string, timeout time.Duration) (*RealClient, error) {
	cfg := evateamclient.Config{
		BaseURL:  baseURL,
		APIToken: apiToken,
		Timeout:  timeout,
	}
	c, err := evateamclient.NewClient(&cfg)
	if err != nil {
		return nil, err
	}
	return &RealClient{inner: c}, nil
}

func (c *RealClient) Close() {
	if c.inner != nil {
		_ = c.inner.Close()
	}
}

func (c *RealClient) Project(ctx context.Context, code string, fields []string) (*models.Project, *models.Meta, error) {
	return c.inner.Project(ctx, code, fields)
}

func (c *RealClient) TaskCreate(ctx context.Context, params *evateamclient.TaskCreateParams) (*models.Task, error) {
	return c.inner.TaskCreate(ctx, params)
}

// Ensure target project exists / resolve ID.
func resolveProjectID(ctx context.Context, client TeamClient, code string) (id string, err error) {
	project, _, err := client.Project(ctx, code, []string{"id", "code", "name"})
	if err != nil {
		return "", fmt.Errorf("resolve EVA project %q: %w", code, err)
	}
	if project == nil || project.ID == "" {
		return "", fmt.Errorf("EVA project %q not found", code)
	}
	return project.ID, nil
}
