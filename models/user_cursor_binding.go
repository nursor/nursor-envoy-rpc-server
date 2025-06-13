package models

import (
	"time"

	"gorm.io/gorm"
)

// UserCursorTokenBinding 定义了 user_cursor_token_binding 表的 GORM 模型
type UserCursorTokenBinding struct {
	ID          uint      `gorm:"primaryKey;autoIncrement"`
	UserID      uint      `gorm:"column:user_id;index"` // 外键，关联 User 表
	User        User      `gorm:"foreignKey:UserID"`    // 关联 User 模型
	CursorToken string    `gorm:"column:cursor_token;type:varchar(1000)"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 自定义表名
func (UserCursorTokenBinding) TableName() string {
	return "user_cursor_token_binding"
}

// FindUserByCursorToken 根据 cursor_token 查找最新的 User 记录
func (m *UserCursorTokenBinding) FindUserByCursorToken(db *gorm.DB, cursorToken string) (*User, error) {
	var binding UserCursorTokenBinding
	err := db.Preload("User").
		Where("cursor_token = ?", cursorToken).
		Order("updated_at DESC").
		First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding.User, nil
}
