package models

import (
	"time"

	"gorm.io/gorm"
)

// TempToken represents the temporary token model.
type TempToken struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    *uint     `gorm:"column:user_id;index;comment:关联用户ID" json:"user_id,omitempty"` // Foreign key to User model
	Token     string    `gorm:"column:token;type:varchar(255);unique;not null;comment:临时token" json:"token"`
	Status    int       `gorm:"column:status;default:0;not null;comment:状态：0: 未使用，1: 已使用，2: 已过期" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime;not null" json:"updated_at"`
}

// TableName sets the custom table name for the TempToken model.
func (TempToken) TableName() string {
	return "user_temp_token"
}

// FindByToken retrieves a TempToken by its token string.
func (t *TempToken) FindByToken(db *gorm.DB, token string) error {
	return db.Where("token = ?", token).First(t).Error
}

// MarkAsUsed updates the status of the token to "used".
func (t *TempToken) MarkAsUsed(db *gorm.DB) error {
	t.Status = 1 // 1 represents '已使用'
	return db.Save(t).Error
}

// MarkAsExpired updates the status of the token to "expired".
func (t *TempToken) MarkAsExpired(db *gorm.DB) error {
	t.Status = 2 // 2 represents '已过期'
	return db.Save(t).Error
}

// CreateTempToken creates a new TempToken record.
func (t *TempToken) CreateTempToken(db *gorm.DB) error {
	return db.Create(t).Error
}
