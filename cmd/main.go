package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"

	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
	"github.com/luoling8192/adk-agent/ent"
	"github.com/luoling8192/adk-agent/ent/chatmessage"
	"github.com/luoling8192/adk-agent/internal/datastore"
	"github.com/mattn/go-runewidth"
	"github.com/nekomeowww/fo"
	"github.com/olekukonko/tablewriter"
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

	runewidth.DefaultCondition.EastAsianWidth = true
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"InChatID", "ChatName", "ChatType", "Count"})

	for _, g := range grouped {
		joinedChat, ok := lo.Find(joinedChats, func(jc *ent.JoinedChat) bool {
			return jc.ChatID == g.InChatID
		})
		if !ok {
			continue
		}

		table.Append([]string{
			g.InChatID,
			joinedChat.ChatName,
			joinedChat.ChatType,
			fmt.Sprint(g.Count),
		})
	}

	table.Render()
}
