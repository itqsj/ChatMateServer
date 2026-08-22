package chat

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"server/config"
	chatrequest "server/models/chat/request"
	"server/services"
)

const defaultAgentInstruction = "You are a helpful assistant."

// TextChunk 是返回给 SSE 接口的一段模型文本。
type TextChunk struct {
	Content string
}

// RAGInput 定义 RAG Chain 的输入数据结构 (固定写法/标准结构)
type RAGInput struct {
	Message string
	History []chatrequest.ChatHistoryMessage
}

// RAGState 定义 RAG Chain 在检索完成后的中间状态 (固定写法/标准结构)
type RAGState struct {
	Input   RAGInput
	Context string
}

// DeepSeekAgentStream 把 Eino Chain 输出的流式响应包装为前端需要的分片流。
type DeepSeekAgentStream struct {
	messageStream *schema.StreamReader[*schema.Message]
}

// NewDeepSeekStream 根据历史消息和当前用户消息创建 RAG Chain 并开启流式响应 (固定写法/标准用法)
func NewDeepSeekStream(ctx context.Context, message string, history []chatrequest.ChatHistoryMessage) (*DeepSeekAgentStream, error) {
	cfg, err := config.LoadDeepSeekConfig()
	if err != nil {
		return nil, err
	}

	// 1. 初始化 ChatModel 聊天模型节点
	chatModel, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		Timeout: 60 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	// 2. 初始化线性编排链条 compose.NewChain (固定写法/标准用法)
	// 输入类型为 RAGInput (问答+历史)，输出最终的模型消息
	chain := compose.NewChain[RAGInput, *schema.Message]()

	// 节点 1：调用 Eino Retriever 检索向量数据库 (固定写法/标准用法)
	// 作用：根据当前 Query 检索 Qdrant，拿到 Top3 相关背景并封装到 RAGState 中传递
	chain.AppendLambda(compose.InvokableLambda(func(ctx context.Context, in RAGInput) (RAGState, error) {
		contextStr, err := services.RetrieveContext(in.Message)
		if err != nil {
			log.Printf("[RAG] Retrieve context failed: %v", err)
			// 检索异常作降级处理，不打断核心问答
		}
		return RAGState{Input: in, Context: contextStr}, nil
	}), compose.WithNodeName("RetrieveContext"))

	// 节点 2：拼装与格式化系统及用户提示词 (固定写法/标准用法)
	// 作用：接收中间状态，动态格式化 System Instruction（拼接召回的 RAG 上下文）以及把多轮 History 转化为 Eino 标准的消息列表
	chain.AppendLambda(compose.InvokableLambda(func(ctx context.Context, state RAGState) ([]*schema.Message, error) {
		instruction := defaultAgentInstruction
		if state.Context != "" {
			instruction = "你是一个智能助理，请根据提供的上下文来回答用户的问题。如果上下文中没有包含相关信息，请如实回答不知道，不要瞎编。\n\n上下文信息如下：\n" + state.Context
		}

		messages := []*schema.Message{
			schema.SystemMessage(instruction),
		}

		for _, item := range state.Input.History {
			switch item.Role {
			case "user":
				messages = append(messages, schema.UserMessage(item.Content))
			case "assistant":
				messages = append(messages, schema.AssistantMessage(item.Content, nil))
			}
		}

		messages = append(messages, schema.UserMessage(state.Input.Message))

		return messages, nil
	}), compose.WithNodeName("FormatPrompt"))

	// 节点 3：关联 LLM 大模型生成节点 (固定写法/标准用法)
	// 作用：将格式化后的消息列表送给 DeepSeek 模型进行推理生成
	chain.AppendChatModel(chatModel)

	// 3. 编译整条编排链路为可运行对象 (固定写法/标准用法)
	runnable, err := chain.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("compile RAG chain failed: %w", err)
	}

	// 4. 以流式方法（Stream）执行链条 (固定写法/标准用法)
	// 特性：由于最终节点是 ChatModel 且支持 Stream，Eino 会自动将前端非流式的 Lambda 节点
	// 以 Invoke 方式顺序调用，到 ChatModel 节点时自动开启 Stream 输出。
	messageStream, err := runnable.Stream(ctx, RAGInput{Message: message, History: history})
	if err != nil {
		return nil, fmt.Errorf("run RAG chain stream failed: %w", err)
	}

	return &DeepSeekAgentStream{
		messageStream: messageStream,
	}, nil
}

// Recv 读取下一段 assistant 文本，屏蔽 MessageStream 的细节。
func (s *DeepSeekAgentStream) Recv() (*TextChunk, error) {
	if s.messageStream == nil {
		return nil, io.EOF
	}
	chunk, err := s.messageStream.Recv()
	if err != nil {
		return nil, err // 包含 io.EOF 退出信号
	}
	if chunk == nil {
		return nil, io.EOF
	}
	return &TextChunk{Content: chunk.Content}, nil
}

// Close 关闭当前正在读取的模型分片流。
func (s *DeepSeekAgentStream) Close() {
	if s.messageStream != nil {
		s.messageStream.Close()
		s.messageStream = nil
	}
}
