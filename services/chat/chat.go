package chat

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"server/config"
	chatrequest "server/models/chat/request"
)

const defaultAgentInstruction = "You are a helpful assistant."

// TextChunk 是返回给 SSE 接口的一段模型文本。
type TextChunk struct {
	Content string
}

// DeepSeekAgentStream 把 ADK Runner 事件流转换成简单的文本分片流。
type DeepSeekAgentStream struct {
	events        *adk.AsyncIterator[*adk.AgentEvent]
	messageStream *schema.StreamReader[*schema.Message]
}

// NewDeepSeekStream 根据历史消息和当前用户消息创建 Agent 流式响应。
func NewDeepSeekStream(ctx context.Context, message string, history []chatrequest.ChatHistoryMessage) (*DeepSeekAgentStream, error) {
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

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "ChatMateAgent",
		Description: "A stateless ChatModelAgent using frontend persisted history.",
		Instruction: defaultAgentInstruction,
		Model:       chatModel,
	})
	if err != nil {
		return nil, err
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	messages := BuildModelMessages(history, message)

	return &DeepSeekAgentStream{
		events: runner.Run(ctx, messages),
	}, nil
}

// BuildModelMessages 把前端历史消息和当前用户输入转换成 Eino 标准消息。
func BuildModelMessages(history []chatrequest.ChatHistoryMessage, message string) []adk.Message {
	messages := make([]adk.Message, 0, len(history)+1)

	for _, item := range history {
		// 前端只允许 user / assistant；这里仍按 role 显式转换，避免消息角色混乱。
		switch item.Role {
		case "user":
			messages = append(messages, schema.UserMessage(item.Content))
		case "assistant":
			messages = append(messages, schema.AssistantMessage(item.Content, nil))
		}
	}

	messages = append(messages, schema.UserMessage(message))

	return messages
}

// Recv 读取下一段 assistant 文本，屏蔽 ADK event 和 MessageStream 的细节。
func (s *DeepSeekAgentStream) Recv() (*TextChunk, error) {
	for {
		if s.messageStream != nil {
			chunk, err := s.messageStream.Recv()
			if errors.Is(err, io.EOF) {
				s.messageStream.Close()
				s.messageStream = nil
				continue
			}
			if err != nil {
				return nil, err
			}
			if chunk == nil || chunk.Content == "" {
				continue
			}

			return &TextChunk{Content: chunk.Content}, nil
		}

		event, ok := s.events.Next()
		if !ok {
			return nil, io.EOF
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			return nil, event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		output := event.Output.MessageOutput
		if output.IsStreaming && output.MessageStream != nil {
			s.messageStream = output.MessageStream
			continue
		}
		if output.Message != nil && output.Message.Content != "" {
			return &TextChunk{Content: output.Message.Content}, nil
		}
	}
}

// Close 关闭当前正在读取的模型分片流。
func (s *DeepSeekAgentStream) Close() {
	if s.messageStream != nil {
		s.messageStream.Close()
		s.messageStream = nil
	}
}
