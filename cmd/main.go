package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
	"github.com/luoling8192/adk-agent/internal/datastore"
	"github.com/nekomeowww/fo"
	"github.com/samber/lo"
)

const (
	defaultDatabaseURL = "postgresql://postgres:postgres@localhost:5432/adk-agent"
)

func main() {
	_ = godotenv.Load()

	ctx := context.Background()

	slog.SetDefault(slog.New(tint.NewHandler(os.Stdout, nil)))

	dsn := fo.May(lo.Coalesce(os.Getenv("DATABASE_URL"), defaultDatabaseURL))

	client, err := datastore.NewEntClient(dsn)
	if err != nil {
		slog.Error("failed to create ent client", "error", err)
		return
	}
	defer client.Close()

	if err := client.Ping(ctx); err != nil {
		slog.Error("failed to ping database", "error", err)
		return
	}

	slog.Info("Client created successfully")

	count, err := client.ChatMessage.Query().Count(ctx)
	if err != nil {
		slog.Error("failed to get chat messages", "error", err)
		return
	}

	slog.Info("Chat messages", "count", count)
}
