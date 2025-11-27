package models

import (
	"time"
)

// MembershipType defines the possible membership types as an enum-like string.
type MembershipType string

const (
	MembershipTypeFree       MembershipType = "Free"
	MembershipTypeTrial      MembershipType = "Trial"
	MembershipTypePremium    MembershipType = "Premium"
	MembershipTypeEnterprise MembershipType = "Enterprise"
	MembershipTypeAnonymous  MembershipType = "Anonymous"
)

// SalesChannel defines the possible sales channels as an enum-like string.
type SalesChannel string

const (
	SalesChannelTaoBao   SalesChannel = "TaoBao"
	SalesChannelWeiShou  SalesChannel = "WeiShou"
	SalesChannelWeiXin   SalesChannel = "WeiXin"
	SalesChannelXianYu   SalesChannel = "XianYu"
	SalesChannelOther    SalesChannel = "Other"
	SalesChannelOfficial SalesChannel = "official"
)

// User represents the user_user table in the database.
type User struct {
	ID             int            `gorm:"primaryKey;column:id" json:"id"`
	IsDispatched   bool           `gorm:"default:false;column:is_dispatched" json:"is_dispatched"`
	IsFree         bool           `gorm:"default:false;column:is_free" json:"is_free"`
	Name           string         `gorm:"type:varchar(255);column:name" json:"name"`
	Email          string         `gorm:"type:varchar(255);unique;column:email" json:"email"`
	InnerToken     string         `gorm:"type:varchar(255);column:inner_token" json:"inner_token"`
	MembershipType MembershipType `gorm:"type:varchar(255);column:membership_type" json:"membership_type"`
	SalesChannel   *SalesChannel  `gorm:"type:varchar(255);column:sales_channel" json:"sales_channel"`
	ClientID       *string        `gorm:"type:varchar(255);column:client_id" json:"client_id"`
	OrderID        *string        `gorm:"type:varchar(255);column:order_id" json:"order_id"`
	ExpiredAt      *time.Time     `gorm:"column:expired_at" json:"expired_at"`
	CreatedAt      time.Time      `gorm:"autoCreateTime;column:created_at" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime;column:updated_at" json:"updated_at"`
	// Django auth required fields
	IsActive    bool       `gorm:"default:true;column:is_active" json:"is_active"`
	IsStaff     bool       `gorm:"default:false;column:is_staff" json:"is_staff"`
	IsSuperuser bool       `gorm:"default:false;column:is_superuser" json:"is_superuser"`
	LastLogin   *time.Time `gorm:"column:last_login" json:"last_login"`
}

// TableName specifies the table name for User.
func (User) TableName() string {
	return "user_user"
}

// String returns a string representation of the User.
func (u User) String() string {
	return u.Name
}
