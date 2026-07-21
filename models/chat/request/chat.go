package request

// ChatHistoryMessage 是前端传给模型的历史消息。
type ChatHistoryMessage struct {
	Role    string `json:"role" binding:"required,oneof=user assistant"`
	Content string `json:"content" binding:"required"`
}

// StreamChatRequest 是聊天流式接口的请求体。
type StreamChatRequest struct {
	History []ChatHistoryMessage `json:"history" binding:"omitempty,dive"`
	Message string               `json:"message" binding:"required"`
}
