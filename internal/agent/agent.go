package agent

import (
	"context"
	"errors"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const (
	summarizerModel  = "deepseek/deepseek-v3.2"
	summarizerPrompt = `你是一个高效的群聊内容分析师。以下是一整天的群聊文本，请你按如下要求为一位新入群的成员总结群聊精华：

1. 全面提炼话题：不仅涵盖热门事件，也要保留具有技术价值的长尾讨论（如具体Bug修复、特定工具配置等），以提高信息召回率。
2. 关联人物与行为：高亮所有重要参与者，简述其特点/专长，并明确其在话题中的具体贡献或观点。
3. 保留关键索引词：务必保留原文中的技术术语、项目名称、错误码、特定人名等，避免过度概括导致关键词丢失。
4. 结构化输出：按话题组织内容，清晰展示“谁（标签）做了什么/说了什么（包含关键细节）”。

输出要求：只输出精炼后的要点，使用简洁、准确的中文纯文本；无需额外解释，适合直接入库及后续结构化提取。`

	extractorModel  = "deepseek/deepseek-v3.2"
	extractorPrompt = `下面是一份群聊内容的总结，请根据其内容将关键信息提取为结构化条目，目标是最大化信息的召回能力。格式如下：

[群友名字];[标签/专长];[事件/观点简述]

要求：
1. [群友名字]：列出相关的所有参与者，用逗号分隔。
2. [标签/专长]：提取参与者的技术栈或角色标签，用逗号分隔。
3. [事件/观点简述]：详细描述事件或观点，**必须包含所有提及的技术名词、工具名、关键参数或核心论点**，以增加搜索命中率。不要过度精简导致关键词丢失。

例子输出：
魔法小猫,Sukka;架构,多问;多tab Web应用状态同步（SharedWorker、BroadcastChannel选举leader、CRDT/OT、音频mutex）
Sukka,阿卡琳;性能,黑魔法;中文Web字体方案（cn-font-split Rust重构、nix构建、Subsetting）
阿卡琳,kinopio,Fish Wang;Rust,安全,优化;Bun运行时成为攻击向量（HelixGuard博客分析、内存安全问题）

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
