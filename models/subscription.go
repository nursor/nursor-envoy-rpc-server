package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Subscription represents the VPN subscription package model.
type Subscription struct {
	ID                 uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name               string     `gorm:"column:name;type:varchar(100);not null;comment:套餐名称, 如：基础套餐、高级套餐等" json:"name"`
	Description        string     `gorm:"column:description;type:text;comment:套餐的详细说明" json:"description"`
	PackageType        string     `gorm:"column:package_type;type:varchar(20);default:basic;not null;comment:套餐类型" json:"package_type"`
	Price              float64    `gorm:"column:price;type:decimal(10,2);not null;comment:套餐价格" json:"price"`
	OriginalPrice      *float64   `gorm:"column:original_price;type:decimal(10,2);comment:促销前的原价" json:"original_price,omitempty"`
	Currency           string     `gorm:"column:currency;type:varchar(3);default:CNY;not null;comment:货币代码，如CNY、USD等" json:"currency"`
	Duration           int        `gorm:"column:duration;not null;comment:套餐时长数值" json:"duration"`
	DurationUnit       string     `gorm:"column:duration_unit;type:varchar(10);default:months;not null;comment:时长单位" json:"duration_unit"`
	TrafficLimit       *int64     `gorm:"column:traffic_limit;comment:流量限制数值（GB）" json:"traffic_limit,omitempty"`
	TrafficUnit        string     `gorm:"column:traffic_unit;type:varchar(10);default:GB;not null;comment:流量单位" json:"traffic_unit"`
	IsUnlimitedTraffic bool       `gorm:"column:is_unlimited_traffic;default:false;not null;comment:是否无流量限制" json:"is_unlimited_traffic"`
	MaxDevices         int        `gorm:"column:max_devices;default:1;not null;comment:同时在线设备数量限制" json:"max_devices"`
	MaxSpeed           string     `gorm:"column:max_speed;type:varchar(20);comment:如：100Mbps、1Gbps等" json:"max_speed"`
	ServerLocations    StringList `gorm:"column:server_locations;type:jsonb;default:'[]';comment:支持的服务器位置列表" json:"server_locations"`
	Status             string     `gorm:"column:status;type:varchar(20);default:active;not null;comment:状态" json:"status"`
	IsFeatured         bool       `gorm:"column:is_featured;default:false;not null;comment:是否在首页推荐" json:"is_featured"`
	IsPopular          bool       `gorm:"column:is_popular;default:false;not null;comment:是否标记为热门" json:"is_popular"`
	IsOnSale           bool       `gorm:"column:is_on_sale;default:false;not null;comment:是否正在促销" json:"is_on_sale"`
	SaleStartDate      *time.Time `gorm:"column:sale_start_date;comment:促销开始时间" json:"sale_start_date,omitempty"`
	SaleEndDate        *time.Time `gorm:"column:sale_end_date;comment:促销结束时间" json:"sale_end_date,omitempty"`
	SaleDiscount       *float64   `gorm:"column:sale_discount;type:decimal(5,2);comment:折扣百分比，如20.00表示8折" json:"sale_discount,omitempty"`
	SortOrder          int        `gorm:"column:sort_order;default:0;not null;comment:数字越小排序越靠前" json:"sort_order"`
	DisplayName        string     `gorm:"column:display_name;type:varchar(100);comment:前台显示的名称，如不填则使用name" json:"display_name,omitempty"`
	CursorAskCount     int        `gorm:"column:cursor_ask_count;default:0;not null;comment:AI调用次数" json:"cursor_ask_count"`
	CursorTabCount     int        `gorm:"column:cursor_tab_count;default:0;not null;comment:AI标签调用次数" json:"cursor_tab_count"`
	CursorTokenUsage   int        `gorm:"column:cursor_token_usage;default:0;not null;comment:AI调用token使用量" json:"cursor_token_usage"`
	CreatedAt          time.Time  `gorm:"column:created_at;autoCreateTime;not null" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;autoUpdateTime;not null" json:"updated_at"`
}

// TableName sets the custom table name for the Subscription model.
func (Subscription) TableName() string {
	return "vpn_subscription"
}

// StringList is a custom type to handle []string as JSONB in PostgreSQL.
type StringList []string

// Value implements the driver.Valuer interface, converting StringList to JSONB.
func (sl StringList) Value() (driver.Value, error) {
	if sl == nil {
		return nil, nil
	}
	return json.Marshal(sl)
}

// Scan implements the sql.Scanner interface, converting JSONB to StringList.
func (sl *StringList) Scan(value interface{}) error {
	if value == nil {
		*sl = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed for StringList")
	}
	return json.Unmarshal(bytes, sl)
}

// GetActiveSubscriptions retrieves all active subscriptions, ordered by sort_order and price.
func (s *Subscription) GetActiveSubscriptions(db *gorm.DB) ([]Subscription, error) {
	var subscriptions []Subscription
	err := db.Where("status = ?", "active").Order("sort_order ASC, price ASC").Find(&subscriptions).Error
	if err != nil {
		return nil, err
	}
	return subscriptions, nil
}

// FindSubscriptionByID finds a subscription by its ID.
func (s *Subscription) FindSubscriptionByID(db *gorm.DB, id uint) error {
	return db.First(s, id).Error
}

// CreateSubscription creates a new subscription record.
func (s *Subscription) CreateSubscription(db *gorm.DB) error {
	return db.Create(s).Error
}

// UpdateSubscription updates an existing subscription record.
func (s *Subscription) UpdateSubscription(db *gorm.DB) error {
	return db.Save(s).Error
}

// DeleteSubscription deletes a subscription record by its ID.
func (s *Subscription) DeleteSubscription(db *gorm.DB, id uint) error {
	return db.Delete(&Subscription{}, id).Error
}
