package helpers

import (
    "gorm.io/driver/mysql"
	"gorm.io/gorm"
    "os"
    "log"
)

func ConnectDB() *gorm.DB {
    db, err := gorm.Open(mysql.Open(os.Getenv("DSN")), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		log.Fatalf("failed to connect to PlanetScale: %v", err)
	}
    return db;
}
