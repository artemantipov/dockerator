package main

import (
	"dockerator/docker"
	"fmt"
	// "net/http"
	// "github.com/labstack/echo/v4"
	// "github.com/labstack/echo/v4/middleware"
)

func main() {
	fmt.Println("Hello App")
	// // Echo instance
	// e := echo.New()

	// // Middleware
	// e.Use(middleware.Logger())
	// e.Use(middleware.Recover())

	// // Routes
	// e.GET("/", hello)

	// // Start server
	// e.Logger.Fatal(e.Start(":8080"))

	docker.PS("up")
}
