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

	client, err := clerk.NewClient(os.Getenv("CLERK_API_KEY"))
    if err != nil {
        log.Fatalf("failed to create Clerk client: %v", err)
    }
    rdb := redis.NewClient(&redis.Options{
        Addr:     os.Getenv("REDIS_SERVER"),
        Password: os.Getenv("REDIS_PASSWORD"),
        DB:       0,
    })

	engine := django.New("./views", ".html")
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	app.Static("/css", "../client/public/css")
	app.Static("/js", "../client/public/js")
    app.Static("/static", "../client/public/static")
    app.Static("/audio", "../client/public/audio")
    app.Static("/images", "../client/public/images")

    app.Get("/", func(c *fiber.Ctx) error {
        return c.Render("pages/homepage/index", fiber.Map{}, "layouts/main")
    })
    app.Get("/stub/home", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
        if err != nil {
            return c.SendStatus(500)
        }
        return c.Render("stubs/home/welcome", fiber.Map{
            "User": user,
        })
    })
    app.Get("/stub/home/join", func(c *fiber.Ctx) error {
        return c.Render("stubs/home/join", fiber.Map{})
    })

    app.Get("/sign-in", func(c *fiber.Ctx) error {
        return c.Render("pages/sign-in/index", fiber.Map{
            "Styles": []string{
                "/css/homepage.css",
            },
        }, "layouts/main")
    })
    app.Get("/sign-up", func(c *fiber.Ctx) error {
        return c.Render("pages/sign-up/index", fiber.Map{
            "Styles": []string{
                "/css/homepage.css",
            },
        }, "layouts/main")
    })
	app.Get("/authorize", func(c *fiber.Ctx) error {
		token := c.Cookies("__session", "")
		if token == "" {
            log.Error("No token found in cookie")
			return c.Redirect("/")
		}
		sessClaims, err := client.VerifyToken(token)
		if err != nil {
            log.Error("Failed to verify token: " + err.Error() + " - " + token)
			return c.Redirect("/")
		}
		user, err := client.Users().Read(sessClaims.Claims.Subject)
		if err != nil {
            log.Error("Failed to read user: " + err.Error())
			return c.Redirect("/")
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
			Secure:   os.Getenv("ENV") == "production",
			HTTPOnly: true,
			SameSite: "Strict",
		})

		return c.Redirect("/")
	})

    RoomRoutes(app, rdb)

	log.Fatal(app.Listen(":3000"))
}

func GetSession(c *fiber.Ctx, rdb *redis.Client) (models.User, error) {
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
