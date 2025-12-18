package distill

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/luoling8192/adk-agent/ent"
	"github.com/luoling8192/adk-agent/ent/chatmessage"
	"github.com/luoling8192/adk-agent/ent/identity"
	"github.com/luoling8192/adk-agent/internal/agent"
	"github.com/luoling8192/adk-agent/internal/datastore"
	"github.com/luoling8192/adk-agent/internal/metrics"
	"github.com/samber/lo"
)

const defaultMaxReplyLength = 20

func truncateRunes(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}

	return string(rs[:n]) + "..."
}

func DistillOneRound(
	ctx context.Context,
	client *datastore.Client,
	grouped []struct {
		InChatID string `json:"in_chat_id"`
		Count    int    `json:"count"`
	},
	selectedIdx int,
	start, end time.Time,
	llmClient *agent.LLMClient,
) (extractedItems []agent.ExtractedItem, err error) {
	startTotal := time.Now()
	defer func() {
		status := "success"
		if err != nil {
			status = "error"
		}
		metrics.DistillDuration.WithLabelValues("total", status).Observe(time.Since(startTotal).Seconds())
	}()

	slog.Info("Fetching messages", "start", start, "end", end)

	queryDurationStart := time.Now()
	messages, err := client.ChatMessage.Query().
		Where(
			chatmessage.InChatID(grouped[selectedIdx].InChatID),
			chatmessage.ContentNEQ(""),
			chatmessage.PlatformTimestampGTE(start.Unix()),
			chatmessage.PlatformTimestampLTE(end.Unix()),
		).
		Select(
			chatmessage.FieldFromName,
			chatmessage.FieldFromID,
			chatmessage.FieldContent,
			chatmessage.FieldReplyToID,
			chatmessage.FieldPlatformTimestamp,
			chatmessage.FieldPlatformMessageID,
		).
		Order(chatmessage.ByPlatformTimestamp(sql.OrderDesc())).
		All(ctx)
	if err != nil {
		slog.Error("failed to get chat messages", "error", err)
		metrics.DistillDuration.WithLabelValues("query_messages", "error").Observe(time.Since(queryDurationStart).Seconds())
		return nil, err
	}
	metrics.DistillDuration.WithLabelValues("query_messages", "success").Observe(time.Since(queryDurationStart).Seconds())
	metrics.DistillItemsCount.WithLabelValues("messages_fetched").Add(float64(len(messages)))

	slog.Info("Chat messages fetched", "count", len(messages), "query_duration", time.Since(queryDurationStart))

	formattedMsgs := make([]string, 0, len(messages))
	for _, message := range messages {
		replyMsg := ""
		if message.ReplyToID != "" {
			replyContent, ok := lo.Find(messages, func(m *ent.ChatMessage) bool {
				return m.PlatformMessageID == message.ReplyToID
			})
			if ok {
				replyMsg = fmt.Sprintf("(reply to: %s)", truncateRunes(replyContent.Content, defaultMaxReplyLength))
			}
		}

		formattedMsgs = append(formattedMsgs, fmt.Sprintf("[%s] %s: %s %s",
			time.Unix(message.PlatformTimestamp, 0).Format("2006-01-02 15:04:05"),
			message.FromName,
			message.Content,
			replyMsg,
		))

		err := client.Identity.Create().
			SetPlatform(message.Platform).
			SetPlatformUserID(message.FromID).
			SetDisplayName(message.FromName).
			OnConflictColumns(identity.FieldPlatform, identity.FieldPlatformUserID).
			Update(func(u *ent.IdentityUpsert) {
				u.SetDisplayName(message.FromName)
			}).
			Exec(ctx)
		if err != nil {
			slog.Error("failed to create identity", "error", err, "from_id", message.FromID, "from_name", message.FromName)
			continue
		}
	}

	slog.Info("Messages processed", "count", len(formattedMsgs), "duration", time.Since(startTotal))

	summaryDurationStart := time.Now()
	summary, err := agent.SummaryMessages(ctx, llmClient, formattedMsgs)
	if err != nil {
		slog.Error("failed to summarize messages", "error", err)
		metrics.DistillDuration.WithLabelValues("summarize", "error").Observe(time.Since(summaryDurationStart).Seconds())
		return nil, err
	}
	metrics.DistillDuration.WithLabelValues("summarize", "success").Observe(time.Since(summaryDurationStart).Seconds())
	slog.Info("Summary generated", "summary", summary, "duration", time.Since(summaryDurationStart))

	extractedItemsDurationStart := time.Now()
	extractedItems, err = agent.ExtractSummary(ctx, llmClient, summary)
	if err != nil {
		slog.Error("failed to extract summary", "error", err)
		metrics.DistillDuration.WithLabelValues("extract", "error").Observe(time.Since(extractedItemsDurationStart).Seconds())
		return nil, err
	}
	metrics.DistillDuration.WithLabelValues("extract", "success").Observe(time.Since(extractedItemsDurationStart).Seconds())
	metrics.DistillItemsCount.WithLabelValues("items_extracted").Add(float64(len(extractedItems)))

	slog.Info("Extracted items", "count", len(extractedItems), "duration", time.Since(extractedItemsDurationStart))

	return extractedItems, nil
}
