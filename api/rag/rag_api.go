package ragapi

import (
	"net/http"
	ragreq "server/models/rag/request"
	"server/services"

	"github.com/gin-gonic/gin"
)

// UploadMarkdown 处理 Markdown 文件上传，将其存入知识库
func UploadMarkdown(c *gin.Context) {
	var req ragreq.UploadMarkdownRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := services.ProcessAndStoreMarkdown(req.File)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process markdown: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}
