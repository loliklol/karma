package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/prymitive/karma/internal/config"
	"github.com/prymitive/karma/internal/eva"
	"github.com/prymitive/karma/internal/store"
)

var ticketStore store.TicketStore

func setupEva() error {
	if !config.Config.Eva.Enabled {
		slog.Info("EVA integration disabled")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pg, err := store.NewPostgresTicketStore(ctx, config.Config.Persistence.DSN, config.Config.Persistence.MaxOpenConns)
	if err != nil {
		return fmt.Errorf("failed to init ticket store: %w", err)
	}
	ticketStore = pg

	client, err := eva.NewRealClient(config.Config.Eva.BaseURL, config.Config.Eva.APIToken, config.Config.Eva.Timeout)
	if err != nil {
		_ = pg.Close()
		return fmt.Errorf("failed to init EVA client: %w", err)
	}

	evaService = eva.NewService(config.Config.Eva, client, ticketStore)
	slog.Info("EVA integration enabled",
		slog.String("baseURL", config.Config.Eva.BaseURL),
		slog.Int("targets", len(config.Config.Eva.Targets)),
	)
	return nil
}
