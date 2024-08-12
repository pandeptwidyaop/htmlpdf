package main

import (
	"github.com/gofiber/fiber/v2"
	"log"
)

func main() {
	initWebServer()
}

func initWebServer() {
	app := fiber.New()

	app.Static("/", "./public")

	log.Fatal(app.Listen(":80"))
}
