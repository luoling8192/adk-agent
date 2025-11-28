package agent

import (
	"context"
	"errors"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const (
	summarizerModel  = "x-ai/grok-4.1-fast:free"
	summarizerPrompt = `You are a chat summarizer. Below is a list of chat messages.
Please analyze and summarize the key events, interesting discussions, or notable people mentioned.
Imagine you are a new group member: what would you find important, fun, or noteworthy to record from this chat?
Output only a concise, clear summary in plain text, in Chinese.

# Notes:
- No need for pleasantries or quoting the original text.
- Just use your own words to briefly and clearly summarize the interesting events or notable people in the group.
- Output plain text only, without any extra explanation.
`
)

type LLMClient struct {
	aiClient *openai.Client
}

func NewLLMClient(baseURL, apiKey string) (*LLMClient, error) {
	if baseURL == "" || apiKey == "" {
		return nil, errors.New("baseURL and apiKey are required")
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL

	client := openai.NewClientWithConfig(config)

	return &LLMClient{aiClient: client}, nil
}

func SummaryMessages(ctx context.Context, llmClient *LLMClient, messages []string) (string, error) {
	if len(messages) == 0 {
		return "", errors.New("no messages to summarize")
	}

	response, err := llmClient.aiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: summarizerModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: summarizerPrompt,
			},
			{
				Role:    "user",
				Content: strings.Join(messages, "\n"),
			},
		},
	})
	if err != nil {
		return "", err
	}

	return response.Choices[0].Message.Content, nil
}
