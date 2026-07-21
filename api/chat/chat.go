package chat

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	chatrequest "server/models/chat/request"
	chatservice "server/services/chat"
)

// Stream 使用 SSE 把 DeepSeek 聊天结果流式返回给客户端。
func Stream(c *gin.Context) {
	var req chatrequest.StreamChatRequest

	// 只负责绑定请求参数，具体聊天逻辑交给 service 处理。
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	// 使用请求上下文创建 DeepSeek 流，客户端断开时能及时取消请求。
	stream, err := chatservice.NewDeepSeekStream(c.Request.Context(), req.Message, req.History)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	defer stream.Close()

	// 设置 SSE 响应头，关闭缓存和代理缓冲，保证前端可以实时收到分片。
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	for {
		// 每次从 Eino Stream 读取一个 DeepSeek 返回的消息分片。
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			c.SSEvent("done", "done")
			c.Writer.Flush()
			return
		}
		if err != nil {
			c.SSEvent("error", err.Error())
			c.Writer.Flush()
			return
		}
		if resp == nil || resp.Content == "" {
			continue
		}

		// 每个有效文本分片都立即刷新，方便前端边接收边渲染。
		c.SSEvent("message", resp.Content)
		c.Writer.Flush()
	}
}
