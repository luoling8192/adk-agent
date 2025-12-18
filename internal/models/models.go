package models

import (
	"github.com/luoling8192/adk-agent/ent"
)

type Client struct {
	client *ent.Client
}

func NewClient(client *ent.Client) *Client {
	return &Client{client: client}
}
