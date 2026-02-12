package distill

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"strings"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/luoling8192/mindwave/ent"
	"github.com/luoling8192/mindwave/ent/chatmessage"
	"github.com/luoling8192/mindwave/ent/identity"
	"github.com/luoling8192/mindwave/ent/joinedchat"
	"github.com/luoling8192/mindwave/internal/agent"
	"github.com/luoling8192/mindwave/internal/datastore"
	"github.com/luoling8192/mindwave/internal/graph"
	"github.com/luoling8192/mindwave/internal/metrics"
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
	graphWriter *graph.Writer,
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
			chatmessage.FieldID,
			chatmessage.FieldFromName,
			chatmessage.FieldFromID,
			chatmessage.FieldPlatform,
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
	nameToIdentity := make(map[string]struct {
		platform string
		userID   string
	})
	messageIDs := make([]uuid.UUID, 0, len(messages))
	var inChatType string
	for _, message := range messages {
		messageIDs = append(messageIDs, message.ID)
		if _, ok := nameToIdentity[message.FromName]; !ok {
			nameToIdentity[message.FromName] = struct {
				platform string
				userID   string
			}{platform: message.Platform, userID: message.FromID}
		}

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

	if inChatType == "" {
		joined, err := client.JoinedChat.Query().
			Where(joinedchat.ChatIDEQ(grouped[selectedIdx].InChatID)).
			Only(ctx)
		if err != nil {
			slog.Warn("failed to resolve chat type, defaulting to group", "error", err, "in_chat_id", grouped[selectedIdx].InChatID)
			inChatType = "group"
		} else {
			inChatType = joined.ChatType
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

	for _, item := range extractedItems {
		participants := item.FromName
		if len(participants) == 0 {
			participants = []string{"unknown"}
		}
		name := truncateRunes(item.Description, 64)
		fromName := strings.Join(participants, ",")
		platform := ""
		if len(messages) > 0 {
			platform = messages[0].Platform
		}

		eventEntity, err := client.Event.Create().
			SetPlatform(platform).
			SetName(name).
			SetTags(item.Tags).
			SetDescription(item.Description).
			SetFromName(fromName).
			SetInChatID(grouped[selectedIdx].InChatID).
			SetInChatType(inChatType).
			SetPlatformTimestamp(end.Unix()).
			SetEvidenceMessageIDs(messageIDs).
			Save(ctx)
		if err != nil {
			slog.Error("failed to create event", "error", err, "name", name)
			continue
		}

		for _, pname := range participants {
			ident, ok := nameToIdentity[pname]
			if !ok {
				continue
			}
			identityEntity, err := client.Identity.Query().
				Where(
					identity.PlatformEQ(ident.platform),
					identity.PlatformUserIDEQ(ident.userID),
				).
				Only(ctx)
			if err != nil {
				slog.Warn("failed to resolve identity for event", "error", err, "name", pname)
				continue
			}
			if err := eventEntity.Update().AddIdentities(identityEntity).Exec(ctx); err != nil {
				slog.Warn("failed to link identity to event", "error", err, "event_id", eventEntity.ID, "identity_id", identityEntity.ID)
			}
		}

		if graphWriter != nil {
			if err := graphWriter.UpsertEvent(ctx, eventEntity, item.Tags, messageIDs); err != nil {
				slog.Warn("failed to write event to graph", "error", err, "event_id", eventEntity.ID)
			}
			for _, pname := range participants {
				ident, ok := nameToIdentity[pname]
				if !ok {
					continue
				}
				if err := graphWriter.UpsertPerson(ctx, ident.platform, ident.userID, pname); err != nil {
					slog.Warn("failed to write person to graph", "error", err, "name", pname)
					continue
				}
				if err := graphWriter.LinkPersonEvent(ctx, ident.platform, ident.userID, eventEntity.ID.String()); err != nil {
					slog.Warn("failed to link person to event in graph", "error", err, "name", pname, "event_id", eventEntity.ID)
				}
			}
			for _, tag := range item.Tags {
				if err := graphWriter.UpsertTopic(ctx, tag); err != nil {
					slog.Warn("failed to write topic to graph", "error", err, "tag", tag)
					continue
				}
				if err := graphWriter.LinkEventTopic(ctx, eventEntity.ID.String(), tag); err != nil {
					slog.Warn("failed to link event to topic in graph", "error", err, "event_id", eventEntity.ID, "tag", tag)
				}
			}
		}
	}

	return extractedItems, nil
}
