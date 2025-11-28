package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/AlecAivazis/survey/v2"
	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
	"github.com/luoling8192/adk-agent/ent"
	"github.com/luoling8192/adk-agent/ent/chatmessage"
	"github.com/luoling8192/adk-agent/internal/datastore"
	"github.com/nekomeowww/fo"
	"github.com/samber/lo"
)

const (
	defaultDatabaseURL = "postgresql://postgres:postgres@localhost:5432/adk-agent"
)

func truncateRunes(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}

	return string(rs[:n]) + "..."
}

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

	var timeRange time.Duration
	var timeRangeStr string
	err = survey.AskOne(&survey.Select{
		Message: "Select a time range to inspect:",
		Options: []string{
			"1 hour",
			"1 day",
		},
		Default: "1 day",
	}, &timeRangeStr)
	if err != nil {
		slog.Error("selection aborted", "error", err)
		return
	}

	switch timeRangeStr {
	case "1 hour":
		timeRange = time.Hour
	case "1 day":
		timeRange = 24 * time.Hour //nolint:mnd
	}

	messages, err := client.ChatMessage.Query().
		Where(
			chatmessage.InChatID(grouped[selectedIdx].InChatID),
			chatmessage.ContentNEQ(""),
			chatmessage.PlatformTimestampGTE(time.Now().Add(-timeRange).Unix()),
			chatmessage.PlatformTimestampLTE(time.Now().Unix()),
		).
		Select(
			chatmessage.FieldFromName,
			chatmessage.FieldContent,
			chatmessage.FieldReplyToID,
			chatmessage.FieldPlatformTimestamp,
		).
		Order(chatmessage.ByPlatformTimestamp(sql.OrderDesc())).
		All(ctx)
	if err != nil {
		slog.Error("failed to get chat messages", "error", err)
		return
	}

	slog.Info("Chat messages", "count", len(messages))

	msgs := make([]string, 0, len(messages))
	for _, message := range messages {
		replyMsg := ""
		if message.ReplyToID != "" {
			replyContent, err := client.ChatMessage.Query().
				Where(
					chatmessage.PlatformMessageID(message.ReplyToID),
					chatmessage.ContentNEQ(""),
				).
				Select(chatmessage.FieldContent).
				First(ctx)
			if err != nil {
				slog.Error("failed to get reply message", "error", err)
				continue
			}

			replyMsg = fmt.Sprintf("(reply to: %s)", truncateRunes(replyContent.Content, 20)) //nolint:mnd
		}

		msgs = append(msgs, fmt.Sprintf("[%s] %s: %s %s",
			time.Unix(message.PlatformTimestamp, 0).Format("2006-01-02 15:04:05"),
			message.FromName,
			message.Content,
			replyMsg,
		))
	}

	fmt.Println(strings.Join(msgs, "\n"))
}
