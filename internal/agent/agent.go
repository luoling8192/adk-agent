package agent

import (
	"context"
	"errors"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const (
	summarizerModel  = "x-ai/grok-4.1-fast:free"
	summarizerPrompt = `你是一个高效的群聊内容分析师。以下是一整天的群聊文本，请你按如下要求为一位新入群的成员总结群聊精华：

1. 提炼当天最有价值、最有趣或最被关注的话题、事件与观点。
2. 高亮值得关注的群友及其重要行为或发言，对每位重要参与者简要描述其特点、专长或标签，并指出与哪些事件相关联。
3. 信息要去重、聚合，避免冗余，以重点信息为主。可结合“群友—标签—事件/观点”的结构，将人与事关联展示。
4. 推荐输出条目以便后续检索：每条用分号分隔，包括【群友姓名或昵称（适当补充标签）】；【时间/范围（可略简化为“今日”）】；【事件或观点简述】；【情感色彩或影响（如有）】。

输出要求：只输出精炼后的要点，使用简洁、准确、结构化的中文纯文本；无需额外解释，也不需要引用原始消息或重复内容，适合直接入库使用。`

	extractorModel  = "x-ai/grok-4.1-fast:free"
	extractorPrompt = `下面是一份群聊内容的总结，请根据其内容将关键信息提取为结构化条目，格式如下：

[群友名字];[标签/专长];[事件/观点简述]

注：多个并列内容请用英文逗号分隔。

例子输出：
魔法小猫,Sukka;架构,多问;多tab Web应用状态同步（SharedWorker、BroadcastChannel选举leader、CRDT/OT、音频mutex）
Sukka,阿卡琳;性能,黑魔法;中文Web字体方案（cn-font-split Rust重构、nix构建）
阿卡琳,kinopio,Fish Wang;Rust,优化;Bun成攻击向量（HelixGuard博客）
kinopio,Fish Wang,阿卡琳;Web,规范;Cloudflare Zero Trust页面体验问题

请输出简明、去重的中文纯文本结构化列表，不需要解释，不要引用原文，专注于关键信息与人脉事件关联，方便后续数据库存储和查询。`
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

type ExtractedItem struct {
	FromName    []string
	Tags        []string
	Description string
}

func ExtractSummary(ctx context.Context, llmClient *LLMClient, summary string) ([]ExtractedItem, error) {
	if summary == "" {
		return []ExtractedItem{}, errors.New("no summary to extract")
	}

	response, err := llmClient.aiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: extractorModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: extractorPrompt,
			},
			{
				Role:    "user",
				Content: summary,
			},
		},
	})
	if err != nil {
		return []ExtractedItem{}, err
	}

	const partsPerItem = 3
	contents := strings.Split(response.Choices[0].Message.Content, "\n")
	items := make([]ExtractedItem, 0)
	for _, content := range contents {
		parts := strings.Split(content, ";")
		if len(parts) != partsPerItem {
			continue
		}

		items = append(items, ExtractedItem{
			FromName:    strings.Split(parts[0], ","),
			Tags:        strings.Split(parts[1], ","),
			Description: parts[2],
		})
	}

	return items, nil
}
