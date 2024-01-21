package main

import (
	"encoding/json"
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
	Id     string `gorm:"column:id"`
	UserId string `gorm:"column:userId"`
	FileId string `gorm:"column:fileId"`
	Name   string `gorm:"column:name"`
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
	LairActions      string `gorm:"column:lairActions;type:text"`
	UserId           string
	Image            string
}

type MonsterInfoTable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
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
			"GM": isGM != "",
		}, "layouts/vtt")
	})

	app.Get("/stub/toolbar", func(c *fiber.Ctx) error {
        isGM := c.Query("gm", "0")
        if (isGM == "1") {
            c.Cookie(&fiber.Cookie{
                Name:     "gm",
                Value:    "1",
                Expires:  time.Now().Add(24 * time.Hour),
                Secure:   os.Getenv("ENV") == "production",
                HTTPOnly: true,
                SameSite: "Strict",
            })
        } else {
            c.ClearCookie("gm")
        }
		return c.Render("stubs/toolbar/toolbar", fiber.Map{
			"GM": isGM == "1",
		})
	})
	app.Get("/stub/toolbar/room", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
        if err != nil {
            log.Error("Failed to get session: " + err.Error())
            return c.SendStatus(401)
        }
		isGM := c.Cookies("gm", "")
		return c.Render("stubs/toolbar/room", fiber.Map{
			"GM": isGM != "",
            "User": user,
		})
	})
	app.Get("/stub/toolbar/window", func(c *fiber.Ctx) error {
        isGM := c.Cookies("gm", "")
		return c.Render("stubs/toolbar/window", fiber.Map{
            "isGM": isGM != "",
        })
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
    app.Get("/stub/toolbar/fog", func(c *fiber.Ctx) error {
		return c.Render("stubs/toolbar/fog", fiber.Map{})
	})

    app.Get("/stub/tabletop/spotlight", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
		if err != nil {
			c.Response().Header.Set("HX-Redirect", "/sign-in")
			return c.SendStatus(401)
		}
		if user.Id == "" {
			c.Response().Header.Set("HX-Redirect", "/sign-in")
			return c.SendStatus(401)
		}

        return c.Render("stubs/tabletop/spotlight", fiber.Map{});
    })
    app.Get("/stub/tabletop/spotlight-search", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
		if err != nil {
			c.Response().Header.Set("HX-Redirect", "/sign-in")
			return c.SendStatus(401)
		}
		if user.Id == "" {
			c.Response().Header.Set("HX-Redirect", "/sign-in")
			return c.SendStatus(401)
		}

        search := c.Query("search", "")
        monsters := []Monster{}
        if search != "" {
            db := helpers.ConnectDB()
            db.Raw("SELECT HEX(id) as id, name, hp, ac, size, image FROM monsters WHERE userId = ? AND name LIKE ?", user.Id, "%"+strings.Trim(search, " ")+"%").Scan(&monsters)
        }

        return c.Render("stubs/tabletop/spotlight-search", fiber.Map{
            "Monsters": monsters,
            "User": user,
        });
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
			"User":   user,
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
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to upload file."}`)
			return c.SendStatus(400)
		}

		src, err := file.Open()
		if err != nil {
			log.Error("Failed to open file: " + err.Error())
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to upload file."}`)
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
            case "image/webp":
                break
            case "image/gif":
                break
            case "image/avif":
                break
            default:
                log.Error("Invalid mime type: " + mimeType)
                c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to upload file."}`)
                return c.SendStatus(400)
		}

		s3Client := CreateSpacesClient()

		object := s3.PutObjectInput{
			Bucket:      aws.String("tabletopper"),
			Key:         aws.String("maps/" + user.Id + "/" + fileId),
			Body:        src,
			ACL:         aws.String("public-read"),
			ContentType: aws.String(mimeType),
		}
		_, err = s3Client.PutObject(&object)
		if err != nil {
			log.Error("Failed to upload file: " + err.Error())
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to upload file."}`)
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
			"User":   user,
		})
	})
    app.Delete("/tabletop/image/:id", func(c *fiber.Ctx) error {
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
            Key:    aws.String("maps/" + user.Id + "/" + id),
        }

        _, err = s3Client.DeleteObject(input)
        if err != nil {
            log.Error("Failed to delete file: " + err.Error())
            c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to delete file."}`)
            return c.SendStatus(500)
        }

        db := helpers.ConnectDB()
        db.Exec("DELETE FROM tabletop_images WHERE fileId = ? AND userId = ?", id, user.Id)

        c.Response().Header.Set("HX-Trigger", `{"toast": "Deleted image."}`)
        return c.SendStatus(200)
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
		db.Raw("SELECT HEX(id) as id, name FROM monsters WHERE userId = ? LIMIT 10", user.Id).Scan(&monsters)

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
            "Monster": Monster{},
            "ImageId": "",
            "ImageName": "",
		})
	})
    app.Get("/stub/tabletop/create-monster/:id", func(c *fiber.Ctx) error {
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

        db := helpers.ConnectDB()
        monster := Monster{}
        db.Raw("SELECT HEX(id) as id, name, size, alignment, type, subtype, ac, hp, speed, str, dex, con, int, wis, cha, savingThrows, skills, vulnerabilities, resistances, immunities, senses, languages, cr, xp, userId, image, abilities, actions, reactions, legendaryActions, lairActions FROM monsters WHERE id = UNHEX(?) AND userId = ?", id, user.Id).Scan(&monster)

        image := Image{}
        db.Raw("SELECT HEX(id) as id, userId, fileId, name FROM monster_images WHERE userId = ? AND fileId = ?", user.Id, monster.Image).Scan(&image)

		return c.Render("stubs/tabletop/create-monster", fiber.Map{
			"User": user,
            "Monster": monster,
            "ImageId": image.FileId,
            "ImageName": image.Name,
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
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to upload file."}`)
			return c.SendStatus(400)
		}

		src, err := file.Open()
		if err != nil {
			log.Error("Failed to open file: " + err.Error())
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to upload file."}`)
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
        case "image/webp":
            break
        case "image/gif":
            break
        case "image/avif":
            break
		default:
			log.Error("Invalid mime type: " + mimeType)
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to upload file."}`)
			return c.SendStatus(400)
		}

		s3Client := CreateSpacesClient()

		object := s3.PutObjectInput{
			Bucket:      aws.String("tabletopper"),
			Key:         aws.String("monsters/" + user.Id + "/" + fileId),
			Body:        src,
			ACL:         aws.String("public-read"),
			ContentType: aws.String(mimeType),
		}
		_, err = s3Client.PutObject(&object)
		if err != nil {
			log.Error("Failed to upload file: " + err.Error())
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to upload file."}`)
			return c.SendStatus(500)
		}

		id := uuid.New().String()
		id = strings.ReplaceAll(id, "-", "")

		db := helpers.ConnectDB()
		db.Exec("INSERT INTO monster_images (id, userId, fileId, name) VALUES (UNHEX(?), ?, ?, ?)", id, user.Id, fileId, fileName)

        monsterId := c.FormValue("monsterId", "")
        if monsterId != "" {
            db.Exec("UPDATE monsters SET image = ? WHERE id = UNHEX(?) AND userId = ?", fileId, monsterId, user.Id)
        }

		return c.Render("stubs/tabletop/monster-image", fiber.Map{
			"ImageName":  fileName,
			"ImageId":    fileId,
            "User":  user,
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
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to delete file."}`)
			return c.SendStatus(500)
		}

		db := helpers.ConnectDB()
		db.Exec("DELETE FROM monster_images WHERE fileId = ? AND userId = ?", id, user.Id)
        db.Exec("UPDATE monsters SET image = '' WHERE image = ? AND userId = ?", id, user.Id)

		return c.Render("stubs/tabletop/monster-image-upload", fiber.Map{})
	})
	app.Delete("/monster/create", func(c *fiber.Ctx) error {
		user, err := GetSession(c, rdb)
		if err != nil {
			c.Response().Header.Set("HX-Redirect", "/sign-in")
			return c.SendStatus(401)
		}
		if user.Id == "" {
			c.Response().Header.Set("HX-Redirect", "/sign-in")
			return c.SendStatus(401)
		}

        id := c.FormValue("id", "")
        if id != "" {
            return c.SendStatus(200)
        }
		imageId := c.FormValue("imageId", "")
		if imageId == "" {
			return c.SendStatus(200)
		}

		s3Client := CreateSpacesClient()
		input := &s3.DeleteObjectInput{
			Bucket: aws.String("tabletopper"),
			Key:    aws.String("monsters/" + user.Id + "/" + imageId),
		}

		_, err = s3Client.DeleteObject(input)
		if err != nil {
			log.Error("Failed to delete file: " + err.Error())
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to delete file."}`)
			return c.SendStatus(500)
		}

		db := helpers.ConnectDB()
		db.Exec("DELETE FROM monster_images WHERE fileId = ? AND userId = ?", imageId, user.Id)

		return c.SendStatus(200)
	})
	app.Post("/monster/create", func(c *fiber.Ctx) error {
		user, err := GetSession(c, rdb)
		if err != nil {
			c.Response().Header.Set("HX-Redirect", "/sign-in")
			return c.SendStatus(401)
		}
		if user.Id == "" {
			c.Response().Header.Set("HX-Redirect", "/sign-in")
			return c.SendStatus(401)
		}

		form, err := c.MultipartForm()

		if err != nil {
			log.Error("Failed to get form: " + err.Error())
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to create monster."}`)
			return c.SendStatus(400)
		}

		name := form.Value["name"][0]
		if name == "" {
			name = "Unnamed Monster"
		}
		size := form.Value["size"][0]
		alignment := form.Value["alignment"][0]
		monsterType := form.Value["type"][0]
		subtype := form.Value["subtype"][0]
		ac := form.Value["ac"][0]
		if ac == "" {
			ac = "0"
		}
		hp := form.Value["hp"][0]
		if hp == "" {
			hp = "0"
		}
		speed := form.Value["speed"][0]
		strength := form.Value["str"][0]
		if strength == "" {
			strength = "0"
		}
		dexterity := form.Value["dex"][0]
		if dexterity == "" {
			dexterity = "0"
		}
		constitution := form.Value["con"][0]
		if constitution == "" {
			constitution = "0"
		}
		intelligence := form.Value["int"][0]
		if intelligence == "" {
			intelligence = "0"
		}
		wisdom := form.Value["wis"][0]
		if wisdom == "" {
			wisdom = "0"
		}
		charisma := form.Value["cha"][0]
		if charisma == "" {
			charisma = "0"
		}
		savingThrows := form.Value["savingThrows"][0]
		skills := form.Value["skills"][0]
		vulnerabilities := form.Value["vulnerabilities"][0]
		resistances := form.Value["resistances"][0]
		immunities := form.Value["immunities"][0]
		senses := form.Value["senses"][0]
		languages := form.Value["languages"][0]
		cr := form.Value["cr"][0]
		if cr == "" {
			cr = "0"
		}
		xp := form.Value["xp"][0]
		if xp == "" {
			xp = "0"
		}

		imageId := ""
		if len(form.Value["imageId"]) > 0 {
			imageId = form.Value["imageId"][0]
		}

        id := strings.ReplaceAll(uuid.New().String(), "-", "")
        doUpdate := false
        if len(form.Value["id"]) > 0 {
            id = form.Value["id"][0]
            doUpdate = true
        }

		abilityNames := form.Value["abilities-name"]
		abilityValues := form.Value["abilities-value"]
		abilities := []MonsterInfoTable{}
		if len(abilityNames) > 0 {
			for i, name := range abilityNames {
				value := abilityValues[i]
				abilities = append(abilities, MonsterInfoTable{
					Name:  name,
					Value: value,
				})
			}
		}

		actionNames := form.Value["actions-name"]
		actionValues := form.Value["actions-value"]
		actions := []MonsterInfoTable{}
		if len(actionNames) > 0 {
			for i, name := range actionNames {
				value := actionValues[i]
				actions = append(actions, MonsterInfoTable{
					Name:  name,
					Value: value,
				})
			}
		}

		legendaryActionNames := form.Value["legendaryActions-name"]
		legendaryActionValues := form.Value["legendaryActions-value"]
		legendaryActions := []MonsterInfoTable{}
		if len(legendaryActionNames) > 0 {
			for i, name := range legendaryActionNames {
				value := legendaryActionValues[i]
				legendaryActions = append(legendaryActions, MonsterInfoTable{
					Name:  name,
					Value: value,
				})
			}
		}

		reactionNames := form.Value["reactions-name"]
		reactionValues := form.Value["reactions-value"]
		reactions := []MonsterInfoTable{}
		if len(reactionNames) > 0 {
			for i, name := range reactionNames {
				value := reactionValues[i]
				reactions = append(reactions, MonsterInfoTable{
					Name:  name,
					Value: value,
				})
			}
		}

		lairActionNames := form.Value["lairActions-name"]
		lairActionValues := form.Value["lairActions-value"]
		lairActions := []MonsterInfoTable{}
		if len(lairActionNames) > 0 {
			for i, name := range lairActionNames {
				value := lairActionValues[i]
				lairActions = append(lairActions, MonsterInfoTable{
					Name:  name,
					Value: value,
				})
			}
		}

		monster := Monster{
			Id:               id,
			Name:             name,
			Size:             size,
			Alignment:        alignment,
			Type:             monsterType,
			Subtype:          subtype,
			AC:               helpers.ParseInt(ac),
			HP:               helpers.ParseInt(hp),
			Speed:            speed,
			Strength:         helpers.ParseInt(strength),
			Dexterity:        helpers.ParseInt(dexterity),
			Constitution:     helpers.ParseInt(constitution),
			Intelligence:     helpers.ParseInt(intelligence),
			Wisdom:           helpers.ParseInt(wisdom),
			Charisma:         helpers.ParseInt(charisma),
			SavingThrows:     savingThrows,
			Skills:           skills,
			Vulnerabilities:  vulnerabilities,
			Resistances:      resistances,
			Immunities:       immunities,
			Senses:           senses,
			Languages:        languages,
			CR:               helpers.ParseInt(cr),
			XP:               helpers.ParseInt(xp),
			UserId:           user.Id,
			Image:            imageId,
			Abilities:        helpers.Marshal(abilities),
			Actions:          helpers.Marshal(actions),
			Reactions:        helpers.Marshal(reactions),
			LegendaryActions: helpers.Marshal(legendaryActions),
			LairActions:      helpers.Marshal(lairActions),
		}

		db := helpers.ConnectDB()
        if doUpdate {
            db.Exec(
                "UPDATE monsters SET name = ?, size = ?, alignment = ?, type = ?, subtype = ?, ac = ?, hp = ?, speed = ?, str = ?, dex = ?, con = ?, int = ?, wis = ?, cha = ?, savingThrows = ?, skills = ?, vulnerabilities = ?, resistances = ?, immunities = ?, senses = ?, languages = ?, cr = ?, xp = ?, image = ?, abilities = ?, actions = ?, reactions = ?, legendaryActions = ?, lairActions = ? " +
                "WHERE id = UNHEX(?) AND userId = ?",
                monster.Name,
                monster.Size,
                monster.Alignment,
                monster.Type,
                monster.Subtype,
                monster.AC,
                monster.HP,
                monster.Speed,
                monster.Strength,
                monster.Dexterity,
                monster.Constitution,
                monster.Intelligence,
                monster.Wisdom,
                monster.Charisma,
                monster.SavingThrows,
                monster.Skills,
                monster.Vulnerabilities,
                monster.Resistances,
                monster.Immunities,
                monster.Senses,
                monster.Languages,
                monster.CR,
                monster.XP,
                monster.Image,
                monster.Abilities,
                monster.Actions,
                monster.Reactions,
                monster.LegendaryActions,
                monster.LairActions,
                monster.Id,
                monster.UserId,
            )
        } else {
            db.Exec(
                "INSERT INTO monsters (id, name, size, alignment, type, subtype, ac, hp, speed, str, dex, con, int, wis, cha, savingThrows, skills, vulnerabilities, resistances, immunities, senses, languages, cr, xp, userId, image, abilities, actions, reactions, legendaryActions, lairActions) "+
                    "VALUES (UNHEX(?), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
                monster.Id,
                monster.Name,
                monster.Size,
                monster.Alignment,
                monster.Type,
                monster.Subtype,
                monster.AC,
                monster.HP,
                monster.Speed,
                monster.Strength,
                monster.Dexterity,
                monster.Constitution,
                monster.Intelligence,
                monster.Wisdom,
                monster.Charisma,
                monster.SavingThrows,
                monster.Skills,
                monster.Vulnerabilities,
                monster.Resistances,
                monster.Immunities,
                monster.Senses,
                monster.Languages,
                monster.CR,
                monster.XP,
                monster.UserId,
                monster.Image,
                monster.Abilities,
                monster.Actions,
                monster.Reactions,
                monster.LegendaryActions,
                monster.LairActions,
            )
        }

        if id == "" {
            c.Response().Header.Set("HX-Trigger", `{"toast": "Created `+name+`", "window:monsters:reset": true}`)
        } else {
            c.Response().Header.Set("HX-Trigger", `{"toast": "Updated `+name+`", "window:monsters:reset": true}`)
        }
		return c.SendStatus(200)
	})

    app.Delete("/monster/:id", func(c *fiber.Ctx) error {
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

        db := helpers.ConnectDB()

        monster := Monster{}
        db.Raw("SELECT HEX(id) as id, image, name FROM monsters WHERE id = UNHEX(?) AND userId = ?", id, user.Id).Scan(&monster)

        if monster.Image != "" {
            s3Client := CreateSpacesClient()
            input := &s3.DeleteObjectInput{
                Bucket: aws.String("tabletopper"),
                Key:    aws.String("monsters/" + user.Id + "/" + monster.Image),
            }

            _, err = s3Client.DeleteObject(input)
            if err != nil {
                log.Error("Failed to delete file: " + err.Error())
                c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to delete file."}`)
                return c.SendStatus(500)
            }

            db.Exec("DELETE FROM monster_images WHERE fileId = ? AND userId = ?", monster.Image, user.Id)
        }

        db.Exec("DELETE FROM monsters WHERE id = UNHEX(?) AND userId = ?", id, user.Id)

        c.Response().Header.Set("HX-Trigger", `{"toast": "Deleted `+monster.Name+`"}`)
        return c.SendStatus(200)
    })

    app.Get("/stub/windows/monsters/:id", func(c *fiber.Ctx) error {
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

        db := helpers.ConnectDB()
        monster := Monster{}
        db.Raw("SELECT HEX(id) as id, name, size, alignment, type, subtype, ac, hp, speed, str, dex, con, int, wis, cha, savingThrows, skills, vulnerabilities, resistances, immunities, senses, languages, cr, xp, userId, image, abilities, actions, reactions, legendaryActions, lairActions FROM monsters WHERE id = UNHEX(?) AND userId = ?", id, user.Id).Scan(&monster)

        abilities := []MonsterInfoTable{}
        json.Unmarshal([]byte(monster.Abilities), &abilities)

        actions := []MonsterInfoTable{}
        json.Unmarshal([]byte(monster.Actions), &actions)

        reactions := []MonsterInfoTable{}
        json.Unmarshal([]byte(monster.Reactions), &reactions)

        legendaryActions := []MonsterInfoTable{}
        json.Unmarshal([]byte(monster.LegendaryActions), &legendaryActions)

        lairActions := []MonsterInfoTable{}
        json.Unmarshal([]byte(monster.LairActions), &lairActions)

        return c.Render("stubs/windows/monster", fiber.Map{
            "Monster": monster,
            "Abilities": abilities,
            "Actions": actions,
            "Reactions": reactions,
            "LegendaryActions": legendaryActions,
            "LairActions": lairActions,
        })
    })

    app.Get("/stub/user/menu", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
        if err != nil {
            return c.SendStatus(401)
        }
        if user.Id == "" {
            return c.SendStatus(401)
        }

        db := helpers.ConnectDB()
        images := []Image{}
        db.Raw("SELECT HEX(id) as id, userId, fileId FROM character_images WHERE userId = ?", user.Id).Scan(&images)

        return c.Render("stubs/user/menu", fiber.Map{
            "User": user,
            "Images": images,
        })
    })

    app.Post("/user/image", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
        if err != nil {
            return c.SendStatus(401)
        }
        if user.Id == "" {
            return c.SendStatus(401)
        }

        file, err := c.FormFile("file")
		if err != nil {
			log.Error("Failed to get file from form: " + err.Error())
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to upload file."}`)
			return c.SendStatus(400)
		}

		src, err := file.Open()
		if err != nil {
			log.Error("Failed to open file: " + err.Error())
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to upload file."}`)
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
        case "image/webp":
            break
        case "image/gif":
            break
        case "image/avif":
            break
		default:
			log.Error("Invalid mime type: " + mimeType)
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to upload file."}`)
			return c.SendStatus(400)
		}

		s3Client := CreateSpacesClient()

		object := s3.PutObjectInput{
			Bucket:      aws.String("tabletopper"),
			Key:         aws.String("characters/" + user.Id + "/" + fileId),
			Body:        src,
			ACL:         aws.String("public-read"),
			ContentType: aws.String(mimeType),
		}
		_, err = s3Client.PutObject(&object)
		if err != nil {
			log.Error("Failed to upload file: " + err.Error())
			c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to upload file."}`)
			return c.SendStatus(500)
		}

		id := uuid.New().String()
		id = strings.ReplaceAll(id, "-", "")

		db := helpers.ConnectDB()
		db.Exec("INSERT INTO character_images (id, userId, fileId, name) VALUES (UNHEX(?), ?, ?, ?)", id, user.Id, fileId, fileName)

		images := []Image{}
		db.Raw("SELECT HEX(id) as id, userId, fileId, name FROM character_images WHERE userId = ?", user.Id).Scan(&images)

		return c.Render("stubs/user/menu", fiber.Map{
			"Images": images,
			"User":   user,
		})
    })

    app.Delete("/user/image/:id", func(c *fiber.Ctx) error {
        user, err := GetSession(c, rdb)
        if err != nil {
            return c.SendStatus(401)
        }
        if user.Id == "" {
            return c.SendStatus(401)
        }

        id := c.Params("id")

        s3Client := CreateSpacesClient()
        input := &s3.DeleteObjectInput{
            Bucket: aws.String("tabletopper"),
            Key:    aws.String("characters/" + user.Id + "/" + id),
        }

        _, err = s3Client.DeleteObject(input)
        if err != nil {
            log.Error("Failed to delete file: " + err.Error())
            c.Response().Header.Set("HX-Trigger", `{"toast": "Failed to delete file."}`)
            return c.SendStatus(500)
        }

        db := helpers.ConnectDB()
        db.Exec("DELETE FROM character_images WHERE fileId = ? AND userId = ?", id, user.Id)

        c.Response().Header.Set("HX-Trigger", `{"toast": "Deleted image."}`)
        return c.SendStatus(200)
    })
}
