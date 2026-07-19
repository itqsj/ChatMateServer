package main

import (
	"server/config"
	"server/routes"
)

const defaultAddr = ":8888"

// main starts the Gin HTTP server.
func main() {
	if err := config.InitMySQL(); err != nil {
		panic(err)
	}

	router := routes.SetupRouter()

	if err := router.Run(defaultAddr); err != nil {
		panic(err)
	}
}
