package agent

import (
	"context"
	"errors"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const (
	summarizerModel  = "deepseek/deepseek-v3.2"
	summarizerPrompt = `你是一位卓越高效的群聊内容分析师。以下是一整天的群聊记录，请按照要求为新成员凝练核心内容：

1. 全面梳理话题：涵盖所有重要事件，并保留具备技术价值的长尾讨论（如具体Bug修复、工具配置等），以最大化信息召回率。
2. 关联人物与行为：突出所有关键参与者，简明描述其特点/专长，并明确其在话题中扮演的具体角色或贡献观点。
3. 精确保留关键词：务必保留原文中的技术术语、项目名称、错误码、关键人名等，避免因过度归纳遗漏关键信息。
4. 结构化输出：以话题为单元，将每段内容清晰展示“谁（标签）做了/说了什么（含关键细节）”。
5. 规范化输出人名：仅限使用我提供的名字，不得自创或补充。

输出要求：仅输出凝练后的重点要点，使用简洁、准确的中文纯文本；无需补充说明，方便直接入库及后续结构化处理。`

	extractorModel  = "deepseek/deepseek-v3.2"
	extractorPrompt = `下面是一份群聊内容的总结，请根据其内容将关键信息提取为结构化条目，目标是最大化信息的召回能力。格式如下：

[群友名字];[标签/专长];[事件/观点简述]

要求：
1. [群友名字]：列出相关的所有参与者，用英文逗号分隔。
2. [标签/专长]：提取参与者的技术栈或角色标签，用英文逗号分隔。
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
