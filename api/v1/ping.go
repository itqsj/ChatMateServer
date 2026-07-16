package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Ping returns a simple health-check response.
func Ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}
