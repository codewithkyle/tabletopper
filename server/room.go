package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

func RoomRoutes(app *fiber.App, rdb *redis.Client) {
    app.Get("/room", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
        if err != nil {
            return c.Redirect("/sign-in")
        }
        if user.Id == "" {
            return c.Redirect("/sign-in")
        }
        return c.Render("pages/room/index", fiber.Map{
        }, "layouts/vtt")
    })
    app.Get("/room/:id", func(c *fiber.Ctx) error {
        return c.Render("pages/room/index", fiber.Map{}, "layouts/vtt")
    })
}
