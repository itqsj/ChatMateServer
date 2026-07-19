package chat

import (
	"context"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/schema"

	"server/config"
)

// NewDeepSeekStream 根据用户消息创建 DeepSeek 流式响应。
func NewDeepSeekStream(ctx context.Context, message string) (*schema.StreamReader[*schema.Message], error) {
	cfg, err := config.LoadDeepSeekConfig()
	if err != nil {
		return nil, err
	}

	chatModel, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		Timeout: 60 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	// 通过 Eino Stream 把用户消息发送给 DeepSeek，并接收分片结果。
	return chatModel.Stream(ctx, []*schema.Message{
		schema.UserMessage(message),
	})
}
