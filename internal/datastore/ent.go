package datastore

import (
	"context"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql/schema"
	"github.com/luoling8192/mindwave/ent"
	"github.com/luoling8192/mindwave/ent/migrate"

	_ "github.com/lib/pq"
)

type Client struct {
	*ent.Client
}

func NewEntClient(databaseURL string) (*Client, error) {
	client, err := ent.Open(dialect.Postgres, databaseURL+"?sslmode=disable")
	if err != nil {
		return nil, err
	}

	return &Client{Client: client}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.ExecContext(ctx, "SELECT 1")
	return err
}

func (c *Client) Migrate(ctx context.Context) error {
	return migrate.Create(
		ctx,
		c.Schema,
		[]*schema.Table{
			migrate.EventsTable,
			migrate.IdentitiesTable,
		},
		migrate.WithForeignKeys(true),
	)
}
