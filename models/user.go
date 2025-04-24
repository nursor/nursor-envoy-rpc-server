package models

import (
	"time"
)

// MembershipType defines the possible membership types as an enum-like string.
type MembershipType string

const (
	MembershipTypeFree       MembershipType = "Free"
	MembershipTypePremium    MembershipType = "Premium"
	MembershipTypeEnterprise MembershipType = "Enterprise"
)

// SalesChannel defines the possible sales channels as an enum-like string.
type SalesChannel string

const (
	SalesChannelTaoBao  SalesChannel = "TaoBao"
	SalesChannelWeiShou SalesChannel = "WeiShou"
	SalesChannelWeiXin  SalesChannel = "WeiXin"
	SalesChannelXianYu  SalesChannel = "XianYu"
)

// User represents the user_user table in the database.
type User struct {
	ID             int            `gorm:"primaryKey;column:id"`
	IsDispatched   bool           `gorm:"default:false;column:is_dispatched"`
	IsFree         bool           `gorm:"default:false;column:is_free"`
	Name           string         `gorm:"type:varchar(255);column:name"`
	Email          string         `gorm:"type:varchar(255);unique;column:email"`
	Password       string         `gorm:"type:varchar(255);column:password"`
	AccessToken    string         `gorm:"type:varchar(255);column:access_token"`
	InnerToken     string         `gorm:"type:varchar(255);column:inner_token"`
	MembershipType MembershipType `gorm:"type:varchar(255);column:membership_type"`
	Limit          int            `gorm:"default:10;column:limit"`
	SalesChannel   *SalesChannel  `gorm:"type:varchar(255);column:sales_channel"`
	ClientID       *string        `gorm:"type:varchar(255);column:client_id"`
	OrderID        *string        `gorm:"type:varchar(255);column:order_id"`
	Usage          int            `gorm:"default:0;column:usage"`
	ExpiredAt      *time.Time     `gorm:"column:expired_at"`
	CreatedAt      time.Time      `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime;column:updated_at"`
}

// TableName specifies the table name for User.
func (User) TableName() string {
	return "user_user"
}

// String returns a string representation of the User.
func (u User) String() string {
	return u.Name
}

// ModelDumpJSON serializes the User struct into a JSON-compatible map.
func (u User) ModelDumpJSON() map[string]interface{} {
	expiredAt := ""
	if u.ExpiredAt != nil {
		expiredAt = u.ExpiredAt.Format(time.RFC3339)
	}
	createdAt := u.CreatedAt.Format(time.RFC3339)
	updatedAt := u.UpdatedAt.Format(time.RFC3339)

	return map[string]interface{}{
		"id":              u.ID,
		"name":            u.Name,
		"email":           u.Email,
		"password":        u.Password,
		"access_token":    u.AccessToken,
		"refresh_token":   u.InnerToken,
		"membership_type": u.MembershipType,
		"limit":           u.Limit,
		"sales_channel":   u.SalesChannel,
		"client_id":       u.ClientID,
		"order_id":        u.OrderID,
		"usage":           u.Usage,
		"expired_at":      expiredAt,
		"created_at":      createdAt,
		"updated_at":      updatedAt,
	}
}
