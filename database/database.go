package database

import (
	"github.com/mrAboalfazl/dnstt-manager/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dbPath string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return err
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(1)

	return DB.AutoMigrate(&models.User{})
}

func Close() {
	if DB != nil {
		sqlDB, _ := DB.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}
}
