package datastore

import (
	"context"

	"entgo.io/ent/dialect"
	"github.com/luoling8192/adk-agent/ent"

	_ "github.com/lib/pq"
)

type Client struct {
	*ent.Client
}

func NewEntClient(databaseURL string) (*Client, error) {
	client, err := ent.Open(dialect.Postgres, databaseURL)
	if err != nil {
		return nil, err
	}

	return &Client{Client: client}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Client.ExecContext(ctx, "SELECT 1")
	return err
}
