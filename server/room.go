package main

import (
	"os"
	"time"

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
            "GM": true,
        }, "layouts/vtt")
    })
    app.Get("/room/:id", func(c *fiber.Ctx) error {
        isGM := c.Cookies("gm", "")
        return c.Render("pages/room/index", fiber.Map{
            "GM": isGM == "true",
        }, "layouts/vtt")
    })
    app.Post("/session/gm/:room", func(c *fiber.Ctx) error {
        room := c.Params("room")
        c.Cookie(&fiber.Cookie{
			Name: "gm",
			Value: room,
			Expires: time.Now().Add(24 * time.Hour),
			Secure: os.Getenv("ENV") == "production",
			HTTPOnly: true,
			SameSite: "Strict",
		})
        return c.SendStatus(200)
    })
    app.Delete("/session/gm", func(c *fiber.Ctx) error {
        c.ClearCookie("gm")
        return c.SendStatus(200)
    })

    app.Get("/stub/toolbar/room", func(c *fiber.Ctx) error {
        return c.Render("stubs/toolbar/room", fiber.Map{})
    })
    app.Get("/stub/toolbar/window", func(c *fiber.Ctx) error {
        return c.Render("stubs/toolbar/window", fiber.Map{})
    })
    app.Get("/stub/toolbar/tabletop", func(c *fiber.Ctx) error {
        return c.Render("stubs/toolbar/tabletop", fiber.Map{})
    })
    app.Get("/stub/toolbar/initiative", func(c *fiber.Ctx) error {
        return c.Render("stubs/toolbar/initiative", fiber.Map{})
    })
    app.Get("/stub/toolbar/view", func(c *fiber.Ctx) error {
        return c.Render("stubs/toolbar/view", fiber.Map{})
    })
    app.Get("/stub/toolbar/help", func(c *fiber.Ctx) error {
        return c.Render("stubs/toolbar/help", fiber.Map{})
    })
}
