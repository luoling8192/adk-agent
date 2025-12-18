package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
	"github.com/luoling8192/adk-agent/ent"
	"github.com/luoling8192/adk-agent/ent/chatmessage"
	"github.com/luoling8192/adk-agent/internal/agent"
	"github.com/luoling8192/adk-agent/internal/datastore"
	"github.com/luoling8192/adk-agent/internal/metrics"
	"github.com/luoling8192/adk-agent/internal/services/distill"
	"github.com/nekomeowww/fo"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc"
)

const (
	defaultDatabaseURL = "postgresql://postgres:postgres@localhost:5432/adk-agent"
	hoursPerDay        = 24 * time.Hour
)

func main() {
	_ = godotenv.Load()
	metrics.StartMetricsServer(os.Getenv("METRICS_ADDR"))

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

	err = client.Migrate(ctx)
	if err != nil {
		slog.Error("failed to migrate database", "error", err)
		return
	}
	slog.Info("Database migrated successfully")

	count, err := client.ChatMessage.Query().Where(chatmessage.ContentNEQ("")).Count(ctx)
	if err != nil {
		slog.Error("failed to get chat messages", "error", err)
		return
	}

	slog.Info("Chat messages", "count", count)

	joinedChats, err := client.JoinedChat.Query().All(ctx)
	if err != nil {
		slog.Error("failed to get joined chats", "error", err)
		return
	}

	slog.Info("Joined chats", "count", len(joinedChats))

	grouped := []struct {
		InChatID string `json:"in_chat_id"`
		Count    int    `json:"count"`
	}{}

	err = client.ChatMessage.Query().
		GroupBy(chatmessage.FieldInChatID).
		Aggregate(ent.Count()).
		Scan(ctx, &grouped)
	if err != nil {
		slog.Error("failed to group chat messages", "error", err)
	}

	sort.Slice(grouped, func(i, j int) bool {
		return grouped[i].Count > grouped[j].Count
	})

	options := make([]string, len(grouped))
	for i, g := range grouped {
		joinedChat, ok := lo.Find(joinedChats, func(jc *ent.JoinedChat) bool {
			return jc.ChatID == g.InChatID
		})
		if !ok {
			continue
		}

		options[i] = fmt.Sprintf(
			"[%d] %s (%s, %s) - %d msgs",
			i,
			g.InChatID,
			joinedChat.ChatName,
			joinedChat.ChatType,
			g.Count,
		)
	}

	var selectedIdx int
	err = survey.AskOne(&survey.Select{
		Message: "Select a chat to inspect:",
		Options: options,
		Default: 0,
	}, &selectedIdx)
	if err != nil {
		slog.Error("selection aborted", "error", err)
		return
	}

	var dayCount string
	err = survey.AskOne(&survey.Input{
		Message: "Select a time range to inspect (day):",
		Default: "1",
	}, &dayCount)
	if err != nil {
		slog.Error("selection aborted", "error", err)
		return
	}

	dayCountInt, err := strconv.Atoi(dayCount)
	if err != nil {
		slog.Error("failed to parse day count", "error", err)
		return
	}

	llmClient, err := agent.NewLLMClient(os.Getenv("LLM_BASE_URL"), os.Getenv("LLM_API_KEY"))
	if err != nil {
		slog.Error("failed to create llm client", "error", err)
		return
	}

	var wg conc.WaitGroup
	for day := range dayCountInt {
		wg.Go(func() {
			start := time.Now().Add(-time.Duration(day) * hoursPerDay)
			end := start.Add(hoursPerDay)

			extractedItems, err := distill.DistillOneRound(ctx, client, grouped, selectedIdx, start, end, llmClient)
			if err != nil {
				slog.Error("failed to distill one round", "error", err)
				return
			}

			for _, item := range extractedItems {
				slog.Info("Extracted item", "from_name", item.FromName, "tags", item.Tags, "description", item.Description)
			}
		})
	}

	wg.Wait()
}
