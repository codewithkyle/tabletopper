package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"main/models"
	"time"

	"github.com/charmbracelet/log"
	"github.com/clerkinc/clerk-sdk-go/clerk"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/django/v3"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var ctx = context.Background();

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("failed to load environment variables: %v", err)
	}

	client, _ := clerk.NewClient(os.Getenv("CLERK_API_KEY"))
    rdb := redis.NewClient(&redis.Options{
        Addr:     os.Getenv("REDIS_SERVER"),
        Password: os.Getenv("REDIS_PASSWORD"),
        DB:       0,
    })

	engine := django.New("./views", ".django")
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	app.Static("/css", "../client/public/css")
	app.Static("/js", "../client/public/js")
    app.Static("/static", "../client/public/static")
    app.Static("/audio", "../client/public/audio")
    app.Static("/images", "../client/public/images")

    app.Get("/", func(c *fiber.Ctx) error {
        return c.Render("pages/homepage/index", fiber.Map{
            "Styles": []string{
                "/css/homepage.css",
            },
        }, "layouts/main")
    })
    app.Get("/stub/home", func(c *fiber.Ctx) error {
        user, err := getSession(c, rdb)
        if err != nil {
            return c.SendStatus(500)
        }
        return c.Render("stubs/home", fiber.Map{
            "User": user,
        })
    })

	app.Get("/register", func(c *fiber.Ctx) error {
		return c.Render("pages/register/index", fiber.Map{})
	})
	app.Get("/sign-in", func(c *fiber.Ctx) error {
		return c.Render("pages/sign-in/index", fiber.Map{})
	})
	app.Get("/sign-out", func(c *fiber.Ctx) error {
		c.ClearCookie("session_id")
		return c.Render("pages/sign-out/index", fiber.Map{})
	})
	app.Get("/authorize", func(c *fiber.Ctx) error {
		token := c.Cookies("__session", "")
		if token == "" {
			return c.Redirect("/sign-in")
		}
		sessClaims, err := client.VerifyToken(token)
		if err != nil {
			return c.Redirect("/sign-in")
		}
		user, err := client.Users().Read(sessClaims.Claims.Subject)
		if err != nil {
			return c.Redirect("/sign-in")
		}

		email := ""
		if len(user.EmailAddresses) > 0 {
			email = user.EmailAddresses[0].EmailAddress
		}

		username := ""
		if user.Username != nil {
			username = *user.Username
		} else {
			username = strings.Trim(user.ID, "user_")
		}

		customUser := models.User{
			Id:       user.ID,
			Username: username,
			Email:    email,
			Avatar:   user.ProfileImageURL,
		}
		sessionId := uuid.New().String()
		sessionId = strings.ReplaceAll(sessionId, "-", "")
		expires := time.Now().Add(168 * time.Hour) // 7 days

        marshalledSession, err := json.Marshal(customUser)
        err = rdb.Set(ctx, "session:" + sessionId, marshalledSession, 168 * time.Hour).Err();
        if err != nil {
            log.Error("Failed to set session in Redis: " + err.Error());
            return c.SendStatus(500);
        }

		c.Cookie(&fiber.Cookie{
			Name:     "session_id",
			Value:    sessionId,
			Expires:  expires,
			Secure:   true,
			HTTPOnly: true,
			SameSite: "Strict",
		})

		return c.Redirect("/")
	})

	log.Fatal(app.Listen(":3000"))
}

func getSession(c *fiber.Ctx, rdb *redis.Client) (models.User, error) {
    sessionId := c.Cookies("session_id", "")
    if sessionId == "" {
        return models.User{}, nil
    }

    marshalledSession, err := rdb.Get(ctx, "session:" + sessionId).Result()
    if err != nil {
        log.Error("Failed to get session from Redis: " + err.Error());
        return models.User{}, err
    }

    var customUser models.User
    err = json.Unmarshal([]byte(marshalledSession), &customUser)
    if err != nil {
        log.Error("Failed to unmarshal session from Redis: " + err.Error());
        return models.User{}, err
    }

    return customUser, nil
}
