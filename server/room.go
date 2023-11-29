package main

import (
	"main/helpers"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/charmbracelet/log"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Image struct {
    Id string `gorm:"column:id"`
    UserId string `gorm:"column:userId"`
    FileId string `gorm:"column:fileId"`
    Name string `gorm:"column:name"`
}

type Monster struct {
    Id               string
    Name             string
    Size             string
    Type             string
    Subtype          string
    Alignment        string
    AC               int
    HP               int
    HitDice          string
    Strength         int `gorm:"column:str"`
    Dexterity        int `gorm:"column:dex"`
    Constitution     int `gorm:"column:con"`
    Intelligence     int `gorm:"column:int"`
    Wisdom           int `gorm:"column:wis"`
    Charisma         int `gorm:"column:cha"`
    Languages        string
    CR               int
    XP               int
    Speed            string
    Vulnerabilities  string
    Resistances      string
    Immunities       string
    Senses           string
    SavingThrows     string
    Skills           string
    Abilities        string `gorm:"type:text"`
    Actions          string `gorm:"type:text"`
    LegendaryActions string `gorm:"column:legendaryActions;type:text"`
    Reactions        string `gorm:"type:text"`
    UserId           string
}

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

    app.Get("/stub/toolbar", func(c *fiber.Ctx) error {
        isGM := c.Cookies("gm", "")
        return c.Render("stubs/toolbar/toolbar", fiber.Map{
            "GM": isGM != "",
        })
    })
    app.Get("/stub/toolbar/room", func(c *fiber.Ctx) error {
        isGM := c.Cookies("gm", "")
        return c.Render("stubs/toolbar/room", fiber.Map{
            "GM": isGM != "",
        })
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
        isGM := c.Cookies("gm", "")
        return c.Render("stubs/toolbar/view", fiber.Map{
            "GM": isGM != "",
        })
    })
    app.Get("/stub/toolbar/help", func(c *fiber.Ctx) error {
        return c.Render("stubs/toolbar/help", fiber.Map{})
    })

    app.Get("/stub/tabletop/settings", func(c *fiber.Ctx) error {
        return c.Render("stubs/tabletop/settings", fiber.Map{})
    })
    app.Get("/stub/tabletop/images", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
        if err != nil {
            c.Response().Header.Set("HX-Redirect", "/sign-in")
            return c.SendStatus(401)
        }
        if user.Id == "" {
            c.Response().Header.Set("HX-Redirect", "/sign-in")
            return c.SendStatus(401)
        }

        db := helpers.ConnectDB()
        images := []Image{}
        db.Raw("SELECT HEX(id) as id, userId, fileId FROM tabletop_images WHERE userId = ?", user.Id).Scan(&images)

        return c.Render("stubs/tabletop/images", fiber.Map{
            "Images": images,
            "User": user,
        })
    })

    app.Post("/tabletop/image", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
        if err != nil {
            c.Response().Header.Set("HX-Redirect", "/sign-in")
            return c.SendStatus(401)
        }
        if user.Id == "" {
            c.Response().Header.Set("HX-Redirect", "/sign-in")
            return c.SendStatus(401)
        }

        file, err := c.FormFile("file")
        if err != nil {
            log.Error("Failed to get file from form: " + err.Error())
            c.Response().Header.Set("HX-Trigger", "{'toast': 'Failed to upload file.'}")
            return c.SendStatus(400)
        }

        src, err := file.Open()
        if err != nil {
            log.Error("Failed to open file: " + err.Error())
            c.Response().Header.Set("HX-Trigger", "{'toast': 'Failed to upload file.'}")
            return c.SendStatus(400)
        }
        defer src.Close()

        fileId := uuid.New().String()
        fileName := file.Filename
        mimeType := file.Header.Get("Content-Type")
        switch mimeType {
            case "image/jpeg":
                break
            case "image/png":
                break
            case "image/jpg":
                break
            default:
                log.Error("Invalid mime type: " + mimeType)
                c.Response().Header.Set("HX-Trigger", "{'toast': 'Failed to upload file.'}")
                return c.SendStatus(400)
        }

        s3Client := CreateSpacesClient()

        object := s3.PutObjectInput{
            Bucket: aws.String("tabletopper"),
            Key:    aws.String("maps/" + user.Id + "/" + fileId),
            Body:   src,
            ACL:    aws.String("public-read"),
            ContentType: aws.String(mimeType),
        }
        _, err = s3Client.PutObject(&object)
        if err != nil {
            log.Error("Failed to upload file: " + err.Error())
            c.Response().Header.Set("HX-Trigger", "{'toast': 'Failed to upload file.'}")
            return c.SendStatus(500)
        }

        id := uuid.New().String()
        id = strings.ReplaceAll(id, "-", "")

        db := helpers.ConnectDB()
        db.Exec("INSERT INTO tabletop_images (id, userId, fileId, name) VALUES (UNHEX(?), ?, ?, ?)", id, user.Id, fileId, fileName)

        images := []Image{}
        db.Raw("SELECT HEX(id) as id, userId, fileId, name FROM tabletop_images WHERE userId = ?", user.Id).Scan(&images)

        return c.Render("stubs/tabletop/images", fiber.Map{
            "Images": images,
            "User": user,
        })
    })

    app.Get("/stub/windows/monsters", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
        if err != nil {
            c.Response().Header.Set("HX-Redirect", "/sign-in")
            return c.SendStatus(401)
        }
        if user.Id == "" {
            c.Response().Header.Set("HX-Redirect", "/sign-in")
            return c.SendStatus(401)
        }

        db := helpers.ConnectDB()
        monsters := []Monster{}
        db.Raw("SELECT * FROM monsters WHERE userId = ? LIMIT 10", user.Id).Scan(&monsters)

        return c.Render("stubs/windows/monsters", fiber.Map{
            "Monsters": monsters,
        })
    })

    app.Get("/stub/tabletop/create-monster", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
        if err != nil {
            c.Response().Header.Set("HX-Redirect", "/sign-in")
            return c.SendStatus(401)
        }
        if user.Id == "" {
            c.Response().Header.Set("HX-Redirect", "/sign-in")
            return c.SendStatus(401)
        }
        return c.Render("stubs/tabletop/create-monster", fiber.Map{
            "User": user,
        })
    })

    app.Post("/monster/image", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
        if err != nil {
            c.Response().Header.Set("HX-Redirect", "/sign-in")
            return c.SendStatus(401)
        }
        if user.Id == "" {
            c.Response().Header.Set("HX-Redirect", "/sign-in")
            return c.SendStatus(401)
        }

        file, err := c.FormFile("file")
        if err != nil {
            log.Error("Failed to get file from form: " + err.Error())
            c.Response().Header.Set("HX-Trigger", "{'toast': 'Failed to upload file.'}")
            return c.SendStatus(400)
        }

        src, err := file.Open()
        if err != nil {
            log.Error("Failed to open file: " + err.Error())
            c.Response().Header.Set("HX-Trigger", "{'toast': 'Failed to upload file.'}")
            return c.SendStatus(400)
        }
        defer src.Close()

        fileId := uuid.New().String()
        fileName := file.Filename
        mimeType := file.Header.Get("Content-Type")
        switch mimeType {
            case "image/jpeg":
                break
            case "image/png":
                break
            case "image/jpg":
                break
            default:
                log.Error("Invalid mime type: " + mimeType)
                c.Response().Header.Set("HX-Trigger", "{'toast': 'Failed to upload file.'}")
                return c.SendStatus(400)
        }

        s3Client := CreateSpacesClient()

        object := s3.PutObjectInput{
            Bucket: aws.String("tabletopper"),
            Key:    aws.String("monsters/" + user.Id + "/" + fileId),
            Body:   src,
            ACL:    aws.String("public-read"),
            ContentType: aws.String(mimeType),
        }
        _, err = s3Client.PutObject(&object)
        if err != nil {
            log.Error("Failed to upload file: " + err.Error())
            c.Response().Header.Set("HX-Trigger", "{'toast': 'Failed to upload file.'}")
            return c.SendStatus(500)
        }

        id := uuid.New().String()
        id = strings.ReplaceAll(id, "-", "")

        db := helpers.ConnectDB()
        db.Exec("INSERT INTO monster_images (id, userId, fileId, name) VALUES (UNHEX(?), ?, ?, ?)", id, user.Id, fileId, fileName)

        return c.Render("stubs/tabletop/monster-image", fiber.Map{
            "Image": "monsters/" + user.Id + "/" + fileId,
            "Name": fileName,
            "Id": id,
        })
    })
    app.Delete("/monster/image/:id", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
        if err != nil {
            c.Response().Header.Set("HX-Redirect", "/sign-in")
            return c.SendStatus(401)
        }
        if user.Id == "" {
            c.Response().Header.Set("HX-Redirect", "/sign-in")
            return c.SendStatus(401)
        }

        id := c.Params("id")

        s3Client := CreateSpacesClient()
        input := &s3.DeleteObjectInput{
            Bucket: aws.String("tabletopper"),
            Key:    aws.String("monsters/" + user.Id + "/" + id),
        }

        _, err = s3Client.DeleteObject(input)
        if err != nil {
            log.Error("Failed to delete file: " + err.Error())
            c.Response().Header.Set("HX-Trigger", "{'toast': 'Failed to delete file.'}")
            return c.SendStatus(500)
        }

        db := helpers.ConnectDB()
        db.Exec("DELETE FROM monster_images WHERE id = UNHEX(?) AND userId = ?", id, user.Id)

        return c.Render("stubs/tabletop/monster-image-upload", fiber.Map{})
    })
}
