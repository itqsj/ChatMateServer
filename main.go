package main

import "server/routes"

const defaultAddr = ":8888"

// main starts the Gin HTTP server.
func main() {
	router := routes.SetupRouter()

	if err := router.Run(defaultAddr); err != nil {
		panic(err)
	}
}
