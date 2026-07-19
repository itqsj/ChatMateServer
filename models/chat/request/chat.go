package request

// StreamChatRequest 是聊天流式接口的请求体。
type StreamChatRequest struct {
	Message string `json:"message" binding:"required"`
}
