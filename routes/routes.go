package routes

import (
	v1 "server/api/v1"

	"github.com/gin-gonic/gin"
)

// SetupRouter creates the Gin engine and registers all routes.
func SetupRouter() *gin.Engine {
	router := gin.Default()

	registerV1Routes(router)

	return router
}

// registerV1Routes registers version 1 API routes.
func registerV1Routes(router *gin.Engine) {
	router.GET("/ping", v1.Ping)
}
