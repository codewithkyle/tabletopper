package main

import (
	"context"
	"encoding/json"
	"main/models"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/charmbracelet/log"
	"github.com/clerkinc/clerk-sdk-go/clerk"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/django/v3"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var ctx = context.Background();

func CalculateModifier(base int) string {
    modifier := math.Floor(float64(base - 10) / 2);
    modifierStr := strconv.Itoa(int(modifier));
    if modifier >= 0 {
        return "+" + modifierStr;
    } else {
        return modifierStr;
    }
}

func CalculateProficiencyBonus(cr int) string {
    if (cr >= 0 && cr <=4){
        return "+2";
    } else if (cr >= 5 && cr <= 8){
        return "+3";
    } else if (cr >= 9 && cr <= 12){
        return "+4";
    } else if (cr >= 13 && cr <= 16){
        return "+5";
    } else if (cr >= 17 && cr <= 20){
        return "+6";
    } else if (cr >= 21 && cr <= 24){
        return "+7";
    } else if (cr >= 26 && cr <= 28){
        return "+8";
    } else {
        return "+9";
    }
}

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
    engine.AddFunc("CalculateModifier", CalculateModifier)
    engine.AddFunc("CalculateProficiencyBonus", CalculateProficiencyBonus)
	app := fiber.New(fiber.Config{
		Views: engine,
        BodyLimit: 1024 * 1024 * 1024,
        StreamRequestBody: true,
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

    app.Get("/tos", func(c *fiber.Ctx) error {
        return c.Render("pages/tos/index", fiber.Map{}, "layouts/main")
    })
    app.Get("/privacy", func(c *fiber.Ctx) error {
        return c.Render("pages/privacy-policy/index", fiber.Map{}, "layouts/main")
    })

    app.Get("/user/verify", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
        if err != nil {
            return c.SendStatus(500)
        }
        if user.Id == "" {
            return c.SendStatus(401)
        }
        return c.SendStatus(200)
    })

    app.Get("/logout", func(c *fiber.Ctx) error {
        sessionId := c.Cookies("session_id", "")
        if sessionId == "" {
            return c.Redirect("/")
        }
        err := rdb.Del(ctx, "session:" + sessionId).Err()
        if err != nil {
            log.Error("Failed to delete session from Redis: " + err.Error());
            return c.SendStatus(500)
        }
        c.ClearCookie("session_id")
        return c.Redirect("/")
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

    app.Get("/*", func(c *fiber.Ctx) error {
        return c.Redirect("/")
    })

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

func CreateSpacesClient() *s3.S3 {
    key := os.Getenv("SPACES_KEY")
    secret := os.Getenv("SPACES_SECRET")

    s3Config := &aws.Config{
        Credentials: credentials.NewStaticCredentials(key, secret, ""),
        Endpoint:    aws.String("https://nyc3.digitaloceanspaces.com"),
        Region:      aws.String("us-east-1"),
        S3ForcePathStyle: aws.Bool(false),
    }

    newSession := session.New(s3Config)
    s3Client := s3.New(newSession)
    return s3Client
}
