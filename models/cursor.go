package models

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Cursor represents the cursor table in the database.
type Cursor struct {
	ID              uint            `gorm:"primaryKey;column:id"`
	Usage           *int            `gorm:"default:0;column:usage"`
	Name            *string         `gorm:"type:varchar(255);column:name"`
	Password        *string         `gorm:"type:varchar(255);column:password"`
	CursorID        *string         `gorm:"type:varchar(255);column:cursor_id"`
	FirstName       *string         `gorm:"type:varchar(255);column:first_name"`
	LastName        *string         `gorm:"type:varchar(255);column:last_name"`
	Email           *string         `gorm:"type:varchar(255);column:email"`
	AccessToken     *string         `gorm:"type:text;column:access_token"`
	RefreshToken    *string         `gorm:"type:text;column:refresh_token"`
	MembershipType  *MembershipType `gorm:"type:varchar(255);column:membership_type"`
	CacheEmail      bool            `gorm:"default:false;column:cache_email"`
	UniqueCppUserID *string         `gorm:"type:varchar(255);column:unique_cpp_user_id"`
	DispatchOrder   *int            `gorm:"default:0;column:dispatch_order"`
	Description     *string         `gorm:"type:text;column:description"`
	Status          string          `gorm:"type:varchar(255);column:status"`
	ExpiresAt       *time.Time      `gorm:"column:expires_at"`
	CreatedAt       time.Time       `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt       time.Time       `gorm:"autoUpdateTime;column:updated_at"`
	ClientKey       *string         `gorm:"type:varchar(255);column:client_key"`
}

// TableName specifies the table name for Cursor.
func (Cursor) TableName() string {
	return "cursor_cursor"
}

// String returns a string representation of the Cursor.
func (c Cursor) String() string {
	if c.Name != nil {
		return *c.Name
	}
	return fmt.Sprintf("Cursor %d", c.ID)
}

// GetAvailableCursors retrieves all active cursors that have not expired.
func (c Cursor) GetAvailableCursors(db *gorm.DB) ([]Cursor, error) {
	var cursors []Cursor
	currentTime := time.Now()
	err := db.Where("status = ? AND expires_at > ?", "active", currentTime).
		Order("usage ASC").
		Find(&cursors).Error
	if err != nil {
		return nil, err
	}
	return cursors, nil
}

// GetCursorByToken retrieves a cursor by its access token.
func (c Cursor) GetCursorByToken(db *gorm.DB, token string) (*Cursor, error) {
	var cursor Cursor
	err := db.Where("access_token = ?", token).First(&cursor).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cursor, nil
}

// UserCursorAccountBind represents the cursor_usercursoraccountbind table.
type UserCursorAccountBind struct {
	ID         uint      `gorm:"primaryKey;column:id"`
	UserID     int       `gorm:"column:user_id"`
	CursorID   uint      `gorm:"column:cursor_id"`
	AskCount   *int      `gorm:"default:0;column:ask_count"`
	TokenUsage *int      `gorm:"default:0;column:token_usage"`
	VipLevel   *int      `gorm:"default:0;column:vip_level"`
	Status     bool      `gorm:"default:true;column:status"`
	CreatedAt  time.Time `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime;column:updated_at"`
	User       User      `gorm:"foreignKey:UserID"`
	Cursor     Cursor    `gorm:"foreignKey:CursorID"`
}

// TableName specifies the table name for UserCursorAccountBind.
func (UserCursorAccountBind) TableName() string {
	return "cursor_usercursoraccountbind"
}

// String returns a string representation of the UserCursorAccountBind.
func (u UserCursorAccountBind) String() string {
	return fmt.Sprintf("%d - %d", u.UserID, u.CursorID)
}

// CursorUrlQueryRecord represents the cursor_cursorurlqueryrecord table.
type CursorUrlQueryRecord struct {
	ID         uint       `gorm:"primaryKey;column:id"`
	CursorID   uint       `gorm:"column:cursor_id"`
	UserID     int        `gorm:"column:user_id"`
	URL        *string    `gorm:"type:varchar(255);column:url"`
	TokenUsage *int       `gorm:"default:0;column:token_usage"`
	Count      *int       `gorm:"default:0;column:count"`
	Date       *time.Time `gorm:"type:date;column:date"`
	CreatedAt  time.Time  `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime;column:updated_at"`
	Cursor     Cursor     `gorm:"foreignKey:CursorID"`
	User       User       `gorm:"foreignKey:UserID"`
}

// TableName specifies the table name for CursorUrlQueryRecord.
func (CursorUrlQueryRecord) TableName() string {
	return "cursor_cursorurlqueryrecord"
}

// UserCursorModelUsage represents the cursor_usercursormodelusage table.
type UserCursorModelUsage struct {
	ID        uint      `gorm:"primaryKey;column:id"`
	CursorID  uint      `gorm:"column:cursor_id"`
	UserID    int       `gorm:"column:user_id"`
	AskCount  *int      `gorm:"default:0;column:ask_count"`
	ModelName *string   `gorm:"type:varchar(255);default:'gpt-4o';column:model_name"`
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime;column:updated_at"`
	Cursor    Cursor    `gorm:"foreignKey:CursorID"`
	User      User      `gorm:"foreignKey:UserID"`
}

type CursorUnavaliableReason struct {
	CursorID   string    `gorm:"primaryKey;column:cursor_id"`
	Cursor     Cursor    `gorm:"foreignKey:CursorID"`
	Reason     string    `gorm:"type:text;column:reason"`
	ReasonType string    `gorm:"type:varchar(255);column:reason_type"`
	CreatedAt  time.Time `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime;column:updated_at"`
}

// TableName specifies the table name for UserCursorModelUsage.
func (UserCursorModelUsage) TableName() string {
	return "cursor_usercursormodelusage"
}

// InitDB initializes the database connection using environment variables.
func InitDB() (*gorm.DB, error) {
	mysqlHost := os.Getenv("MYSQL_HOST")
	if mysqlHost == "" {
		mysqlHost = "172.16.238.2"
	}
	mysqlPort := os.Getenv("MYSQL_PORT")
	if mysqlPort == "" {
		mysqlPort = "30409"
	}
	mysqlUser := os.Getenv("MYSQL_USER")
	if mysqlUser == "" {
		mysqlUser = "root"
	}
	mysqlPassword := os.Getenv("MYSQL_PASSWORD")
	if mysqlPassword == "" {
		mysqlPassword = "asd123456"
	}
	mysqlDB := os.Getenv("MYSQL_DB")
	if mysqlDB == "" {
		mysqlDB = "nursor-test"
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		mysqlUser, mysqlPassword, mysqlHost, mysqlPort, mysqlDB)
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN: dsn,
	}), &gorm.Config{})
	if err != nil {
		logrus.Errorf("Failed to connect to database: %v", err)
		return nil, err
	}

	// Auto-migrate the schemas
	err = db.AutoMigrate(&Cursor{}, &UserCursorAccountBind{}, &CursorUrlQueryRecord{}, &UserCursorModelUsage{}, &User{})
	if err != nil {
		logrus.Errorf("Failed to auto-migrate schemas: %v", err)
		return nil, err
	}

	logrus.Info("Database connection initialized successfully")
	return db, nil
}
