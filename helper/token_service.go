package helper

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// GetNewDB is a placeholder for getting a new GORM DB connection.
func GetNewDB() *gorm.DB {
	// Implement GORM DB initialization (e.g., using models.InitDB)
	MYSQL_HOST := os.Getenv("MYSQL_HOST")
	MYSQL_PORT := os.Getenv("MYSQL_PORT")
	MYSQL_USER := os.Getenv("MYSQL_USER")
	MYSQL_PASSWORD := os.Getenv("MYSQL_PASSWORD")
	MYSQL_DATABASE := os.Getenv("MYSQL_DATABASE")
	if MYSQL_HOST == "" {
		MYSQL_HOST = "172.16.238.2"
	}
	if MYSQL_PORT == "" {
		MYSQL_PORT = "31494"
	}
	if MYSQL_USER == "" {
		MYSQL_USER = "root"
	}
	if MYSQL_PASSWORD == "" {
		MYSQL_PASSWORD = "asd123456"
	}
	if MYSQL_DATABASE == "" {
		MYSQL_DATABASE = "nursorv2"
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", MYSQL_USER, MYSQL_PASSWORD, MYSQL_HOST, MYSQL_PORT, MYSQL_DATABASE)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logrus.Fatalf("Failed to initialize database: %v", err)
	}
	return db
}
