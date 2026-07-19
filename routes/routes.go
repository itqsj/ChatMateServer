package routes

import (
	chatapi "server/api/chat"
	v1 "server/api/v1"
	"server/middleware"

	"github.com/gin-gonic/gin"
)

// SetupRouter 创建 Gin 引擎并注册全部路由。
func SetupRouter() *gin.Engine {
	router := gin.Default()
	router.Use(middleware.CORS())

	registerV1Routes(router)
	registerChatRoutes(router)

	return router
}

// registerV1Routes 注册 v1 API 路由。
func registerV1Routes(router *gin.Engine) {
	router.GET("/ping", v1.Ping)
}

// registerChatRoutes 注册聊天模块路由。
func registerChatRoutes(router *gin.Engine) {
	chatGroup := router.Group("/api/chat")
	chatGroup.POST("/stream", chatapi.Stream)
}
