package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// UserSubscription represents the user's VPN subscription record model.
type UserSubscription struct {
	ID               uint         `gorm:"primaryKey;autoIncrement"`
	UserID           *uint        `gorm:"column:user_id;index;comment:关联用户ID"`             // Foreign key to User model
	User             User         `gorm:"foreignKey:UserID"`                               // GORM will automatically manage the association
	SubscriptionID   *uint        `gorm:"column:subscription_id;index;comment:关联订阅套餐ID"`   // Foreign key to Subscription model
	Subscription     Subscription `gorm:"foreignKey:SubscriptionID"`                       // GORM will automatically manage the association
	TempTokenID      *uint        `gorm:"column:temp_token_id;index;comment:关联临时token ID"` // Foreign key to TempToken model
	TempToken        TempToken    `gorm:"foreignKey:TempTokenID"`                          // GORM will automatically manage the association
	StartDate        time.Time    `gorm:"column:start_date;autoCreateTime;not null;comment:开始时间"`
	EndDate          time.Time    `gorm:"column:end_date;not null;comment:订阅到期时间"`
	Status           string       `gorm:"column:status;type:varchar(20);default:pending;not null;comment:订阅状态"`
	PaymentStatus    string       `gorm:"column:payment_status;type:varchar(20);default:pending;not null;comment:支付状态"`
	PaymentAmount    float64      `gorm:"column:payment_amount;type:decimal(10,2);not null;comment:支付金额"`
	PaymentMethod    string       `gorm:"column:payment_method;type:varchar(50);comment:如：支付宝、微信、银行卡等"`
	PaymentTime      *time.Time   `gorm:"column:payment_time;comment:支付时间"`
	TransactionID    string       `gorm:"column:transaction_id;type:varchar(100);comment:第三方支付平台的交易ID"`
	UsedTraffic      int64        `gorm:"column:used_traffic;default:0;not null;comment:已使用的流量（GB）"`
	TotalTraffic     *int64       `gorm:"column:total_traffic;comment:套餐总流量（GB）"`
	LastUsed         *time.Time   `gorm:"column:last_used;comment:最后使用时间"`
	CursorAskUsage   int          `gorm:"column:cursor_ask_usage;default:0;not null;comment:AI调用次数"`
	CursorTabUsage   int          `gorm:"column:cursor_tab_usage;default:0;not null;comment:AI标签调用次数"`
	CursorTokenUsage int          `gorm:"column:cursor_token_usage;default:0;not null;comment:AI调用token使用量"`
	VPNConfig        JSONMap      `gorm:"column:vpn_config;type:jsonb;default:'{}';comment:VPN连接配置信息"`
	ServerAssigned   string       `gorm:"column:server_assigned;type:varchar(100);comment:分配给用户的服务器"`
	Notes            string       `gorm:"column:notes;type:text;comment:管理员备注信息"`
	CreatedAt        time.Time    `gorm:"column:created_at;autoCreateTime;not null"`
	UpdatedAt        time.Time    `gorm:"column:updated_at;autoUpdateTime;not null"`
}

// TableName sets the custom table name for the UserSubscription model.
func (UserSubscription) TableName() string {
	return "vpn_user_subscription"
}

// JSONMap is a custom type for handling JSON fields where the content is a map (dict).
type JSONMap map[string]interface{}

// Value implements the driver.Valuer interface.
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface.
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed for JSONMap")
	}
	return json.Unmarshal(bytes, j)
}

// GetUserSubscriptionsByUserID retrieves all subscriptions for a given user, ordered by creation time.
func (us *UserSubscription) GetUserSubscriptionsByUserID(db *gorm.DB, userID uint) ([]UserSubscription, error) {
	var subscriptions []UserSubscription
	err := db.Preload("Subscription").Preload("TempToken").Where("user_id = ?", userID).Order("created_at DESC").Find(&subscriptions).Error
	if err != nil {
		return nil, err
	}
	return subscriptions, nil
}

// FindUserSubscriptionsByUserIDAndStatus retrieves all subscriptions for a given user and status,
// ordered by creation time in descending order.
func (us *UserSubscription) FindUserSubscriptionsByUserIDAndStatus(db *gorm.DB, userID uint, status string) ([]UserSubscription, error) {
	var subscriptions []UserSubscription
	err := db.
		Preload("Subscription"). // 预加载关联的 Subscription 对象
		Preload("TempToken").    // 预加载关联的 TempToken 对象
		Where("user_id = ? AND status = ? AND payment_status = ?", userID, status, "paid").
		Order("created_at DESC"). // 按创建时间降序排序
		Find(&subscriptions).Error
	if err != nil {
		return nil, err
	}
	return subscriptions, nil
}

// FindActiveSubscriptionForUser finds an active subscription for a specific user and subscription ID.
func (us *UserSubscription) FindActiveSubscriptionForUser(db *gorm.DB, userID uint, subscriptionID uint) error {
	return db.Where("user_id = ? AND subscription_id = ? AND status = ?", userID, subscriptionID, "active").First(us).Error
}

// UpdateSubscriptionStatus updates the status of a user subscription.
func (us *UserSubscription) UpdateSubscriptionStatus(db *gorm.DB, newStatus string) error {
	us.Status = newStatus
	return db.Save(us).Error
}

// UpdatePaymentStatus updates the payment status of a user subscription.
func (us *UserSubscription) UpdatePaymentStatus(db *gorm.DB, newPaymentStatus string) error {
	us.PaymentStatus = newPaymentStatus
	return db.Save(us).Error
}

// CreateUserSubscription creates a new user subscription record.
func (us *UserSubscription) CreateUserSubscription(db *gorm.DB) error {
	return db.Create(us).Error
}
