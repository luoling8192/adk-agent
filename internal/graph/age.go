package graph

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/luoling8192/mindwave/ent"
	"github.com/luoling8192/mindwave/internal/datastore"
)

type Writer struct {
	client    *datastore.Client
	graphName string
}

func NewWriter(client *datastore.Client, graphName string) (*Writer, error) {
	graphName = strings.TrimSpace(graphName)
	if graphName == "" {
		return nil, fmt.Errorf("graph name is required")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(graphName) {
		return nil, fmt.Errorf("invalid graph name: %s", graphName)
	}
	return &Writer{
		client:    client,
		graphName: graphName,
	}, nil
}

func (w *Writer) EnsureGraph(ctx context.Context) error {
	stmt := fmt.Sprintf(`DO $$
BEGIN
  PERFORM ag_catalog.create_graph('%s');
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;`, w.graphName)
	_, err := w.client.ExecContext(ctx, stmt)
	return err
}

func (w *Writer) UpsertPerson(ctx context.Context, platform, userID, displayName string) error {
	query := fmt.Sprintf(
		`MERGE (p:Person {platform: '%s', platform_user_id: '%s'})
SET p.name = '%s'`,
		escape(platform),
		escape(userID),
		escape(displayName),
	)
	return w.execCypher(ctx, query)
}

func (w *Writer) UpsertEvent(ctx context.Context, event *ent.Event, tags []string, evidenceMessageIDs []uuid.UUID) error {
	tagList := formatList(tags)
	evidenceList := formatUUIDList(evidenceMessageIDs)
	query := fmt.Sprintf(
		`MERGE (e:Event {uuid: '%s'})
SET e.name = '%s',
    e.description = '%s',
    e.platform = '%s',
    e.in_chat_id = '%s',
    e.in_chat_type = '%s',
    e.platform_timestamp = %d,
    e.tags = %s,
    e.evidence_message_ids = %s`,
		escape(event.ID.String()),
		escape(event.Name),
		escape(event.Description),
		escape(event.Platform),
		escape(event.InChatID),
		escape(event.InChatType),
		event.PlatformTimestamp,
		tagList,
		evidenceList,
	)
	return w.execCypher(ctx, query)
}

func (w *Writer) UpsertTopic(ctx context.Context, name string) error {
	query := fmt.Sprintf(
		`MERGE (t:Topic {name: '%s'})`,
		escape(name),
	)
	return w.execCypher(ctx, query)
}

func (w *Writer) LinkPersonEvent(ctx context.Context, platform, userID, eventUUID string) error {
	query := fmt.Sprintf(
		`MATCH (p:Person {platform: '%s', platform_user_id: '%s'}), (e:Event {uuid: '%s'})
MERGE (p)-[:CONTRIBUTED_TO]->(e)`,
		escape(platform),
		escape(userID),
		escape(eventUUID),
	)
	return w.execCypher(ctx, query)
}

func (w *Writer) LinkEventTopic(ctx context.Context, eventUUID, topic string) error {
	query := fmt.Sprintf(
		`MATCH (e:Event {uuid: '%s'}), (t:Topic {name: '%s'})
MERGE (e)-[:MENTIONS]->(t)`,
		escape(eventUUID),
		escape(topic),
	)
	return w.execCypher(ctx, query)
}

func (w *Writer) execCypher(ctx context.Context, query string) error {
	stmt := fmt.Sprintf(
		"SELECT * FROM ag_catalog.cypher('%s', $$%s$$) as (v agtype);",
		w.graphName,
		query,
	)
	_, err := w.client.ExecContext(ctx, stmt)
	return err
}

func escape(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func formatList(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	escaped := make([]string, 0, len(values))
	for _, value := range values {
		escaped = append(escaped, fmt.Sprintf("'%s'", escape(value)))
	}
	return "[" + strings.Join(escaped, ", ") + "]"
}

func formatUUIDList(values []uuid.UUID) string {
	if len(values) == 0 {
		return "[]"
	}
	escaped := make([]string, 0, len(values))
	for _, value := range values {
		escaped = append(escaped, fmt.Sprintf("'%s'", escape(value.String())))
	}
	return "[" + strings.Join(escaped, ", ") + "]"
}
