package helper

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	httpRecord "nursor-envoy-rpc/models/nursor"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var postgresDB *gorm.DB
var enableSQLLogs bool = false // 控制SQL日志输出
var dbMutex sync.Mutex         // 添加互斥锁保护数据库连接

// GetPostgresDB 获取PostgreSQL数据库连接
func GetPostgresDB() *gorm.DB {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	if postgresDB != nil {
		return postgresDB
	}

	// 从环境变量获取PostgreSQL配置
	POSTGRES_HOST := os.Getenv("POSTGRES_HOST")
	POSTGRES_PORT := os.Getenv("POSTGRES_PORT")
	POSTGRES_USER := os.Getenv("POSTGRES_USER")
	POSTGRES_PASSWORD := os.Getenv("POSTGRES_PASSWORD")
	POSTGRES_DATABASE := os.Getenv("POSTGRES_DATABASE")

	// 设置默认值
	if POSTGRES_HOST == "" {
		POSTGRES_HOST = "172.16.238.2"
	}
	if POSTGRES_PORT == "" {
		POSTGRES_PORT = "31279"
	}
	if POSTGRES_USER == "" {
		POSTGRES_USER = "postgres"
	}
	if POSTGRES_PASSWORD == "" {
		POSTGRES_PASSWORD = "asd123456"
	}
	if POSTGRES_DATABASE == "" {
		POSTGRES_DATABASE = "nursor_http_records"
	}

	// 时区配置
	POSTGRES_TIMEZONE := os.Getenv("POSTGRES_TIMEZONE")
	if POSTGRES_TIMEZONE == "" {
		POSTGRES_TIMEZONE = "UTC" // 默认使用UTC时区
	}

	// 构建DSN
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=%s",
		POSTGRES_HOST, POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DATABASE, POSTGRES_PORT, POSTGRES_TIMEZONE)

	// 配置GORM - 禁用SQL日志输出
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error), // 只显示错误，不显示SQL查询
	}

	// 连接数据库
	db, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	// 设置连接池参数 - 优化连接池配置
	sqlDB.SetMaxIdleConns(5)                   // 减少空闲连接数
	sqlDB.SetMaxOpenConns(20)                  // 减少最大连接数，避免超过PostgreSQL限制
	sqlDB.SetConnMaxLifetime(30 * time.Minute) // 减少连接生命周期

	postgresDB = db
	// 注释掉连接成功日志，减少日志输出
	// log.Println("PostgreSQL connection established successfully")
	return postgresDB
}

// EnableSQLLogs 启用或禁用SQL日志输出
func EnableSQLLogs(enabled bool) {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	enableSQLLogs = enabled
	// 注意：不强制重新连接，避免连接数过多
	// 如果需要重新应用日志设置，建议重启应用
}

// GetDBStats 获取数据库连接池统计信息
func GetDBStats() map[string]interface{} {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	if postgresDB == nil {
		return map[string]interface{}{
			"status": "not_connected",
		}
	}

	sqlDB, err := postgresDB.DB()
	if err != nil {
		return map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
	}

	stats := sqlDB.Stats()
	return map[string]interface{}{
		"status":              "connected",
		"max_open_conns":      stats.MaxOpenConnections,
		"open_conns":          stats.OpenConnections,
		"in_use":              stats.InUse,
		"idle":                stats.Idle,
		"wait_count":          stats.WaitCount,
		"wait_duration":       stats.WaitDuration,
		"max_idle_closed":     stats.MaxIdleClosed,
		"max_lifetime_closed": stats.MaxLifetimeClosed,
	}
}

// HttpRecordModel PostgreSQL中的HTTP记录模型
type HttpRecordModel struct {
	ID              uint              `gorm:"primaryKey;autoIncrement"`
	RequestHeaders  map[string]string `gorm:"type:jsonb"`
	RequestBody     []byte            `gorm:"type:bytea"`
	ResponseHeaders map[string]string `gorm:"type:jsonb"`
	ResponseBody    []byte            `gorm:"type:bytea"`
	URL             string            `gorm:"type:text;index:idx_http_records_url"`
	Method          string            `gorm:"type:varchar(10);index:idx_http_records_method"`
	Host            string            `gorm:"type:varchar(255);index:idx_http_records_host"`
	CreateAt        string            `gorm:"type:varchar(50)"`
	HttpVersion     string            `gorm:"type:varchar(10)"`
	InnerTokenID    string            `gorm:"type:varchar(255);index:idx_http_records_inner_token_id"`
	Status          int               `gorm:"type:integer;index:idx_http_records_status"`
	CreatedAt       time.Time         `gorm:"autoCreateTime;index:idx_http_records_created_at"`
	UpdatedAt       time.Time         `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (HttpRecordModel) TableName() string {
	return "http_records"
}

// SaveHttpRecord 保存HTTP记录到PostgreSQL
func SaveHttpRecord(record *httpRecord.HttpRecord) error {
	db := GetPostgresDB()

	// 创建数据库模型
	httpRecordModel := &HttpRecordModel{
		RequestHeaders:  record.RequestHeaders,
		RequestBody:     record.RequestBody,
		ResponseHeaders: record.ResponseHeaders,
		ResponseBody:    record.ResponseBody,
		URL:             record.Url,
		Method:          record.Method,
		Host:            record.Host,
		CreateAt:        record.CreateAt,
		HttpVersion:     record.HttpVersion,
		InnerTokenID:    record.InnerTokenId,
		Status:          record.Status,
	}

	// 保存到数据库
	if err := db.Create(httpRecordModel).Error; err != nil {
		return fmt.Errorf("failed to save HTTP record to PostgreSQL: %v", err)
	}

	// log.Printf("HTTP record saved to PostgreSQL: ID=%d, URL=%s, Method=%s, Status=%d",
	// httpRecordModel.ID, record.Url, record.Method, record.Status)

	return nil
}

// GetHttpRecordsByToken 根据InnerTokenID获取HTTP记录
func GetHttpRecordsByToken(innerTokenID string, limit int) ([]HttpRecordModel, error) {
	db := GetPostgresDB()
	var records []HttpRecordModel

	if limit <= 0 {
		limit = 100 // 默认限制
	}

	err := db.Where("inner_token_id = ?", innerTokenID).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error

	return records, err
}

// GetHttpRecordsByHost 根据Host获取HTTP记录
func GetHttpRecordsByHost(host string, limit int) ([]HttpRecordModel, error) {
	db := GetPostgresDB()
	var records []HttpRecordModel

	if limit <= 0 {
		limit = 100
	}

	err := db.Where("host = ?", host).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error

	return records, err
}

// GetHttpRecordsByStatus 根据状态码获取HTTP记录
func GetHttpRecordsByStatus(status int, limit int) ([]HttpRecordModel, error) {
	db := GetPostgresDB()
	var records []HttpRecordModel

	if limit <= 0 {
		limit = 100
	}

	err := db.Where("status = ?", status).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error

	return records, err
}

// GetHttpRecordsByMethod 根据HTTP方法获取记录
func GetHttpRecordsByMethod(method string, limit int) ([]HttpRecordModel, error) {
	db := GetPostgresDB()
	var records []HttpRecordModel

	if limit <= 0 {
		limit = 100
	}

	err := db.Where("method = ?", method).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error

	return records, err
}

// GetHttpRecordsByDateRange 根据时间范围获取HTTP记录
func GetHttpRecordsByDateRange(startTime, endTime time.Time, limit int) ([]HttpRecordModel, error) {
	db := GetPostgresDB()
	var records []HttpRecordModel

	if limit <= 0 {
		limit = 100
	}

	err := db.Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error

	return records, err
}

// GetHttpRecordsByHostAndStatus 根据Host和状态码获取记录
func GetHttpRecordsByHostAndStatus(host string, status int, limit int) ([]HttpRecordModel, error) {
	db := GetPostgresDB()
	var records []HttpRecordModel

	if limit <= 0 {
		limit = 100
	}

	err := db.Where("host = ? AND status = ?", host, status).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error

	return records, err
}

// GetHttpRecordsByMethodAndStatus 根据HTTP方法和状态码获取记录
func GetHttpRecordsByMethodAndStatus(method string, status int, limit int) ([]HttpRecordModel, error) {
	db := GetPostgresDB()
	var records []HttpRecordModel

	if limit <= 0 {
		limit = 100
	}

	err := db.Where("method = ? AND status = ?", method, status).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error

	return records, err
}

// SearchHttpRecordsByURL 根据URL模式搜索记录
func SearchHttpRecordsByURL(urlPattern string, limit int) ([]HttpRecordModel, error) {
	db := GetPostgresDB()
	var records []HttpRecordModel

	if limit <= 0 {
		limit = 100
	}

	err := db.Where("url ILIKE ?", "%"+urlPattern+"%").
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error

	return records, err
}

// GetHttpRecordsStats 获取HTTP记录统计信息
func GetHttpRecordsStats() (map[string]interface{}, error) {
	db := GetPostgresDB()
	stats := make(map[string]interface{})

	// 总记录数
	var totalCount int64
	if err := db.Model(&HttpRecordModel{}).Count(&totalCount).Error; err != nil {
		return nil, err
	}
	stats["total_count"] = totalCount

	// 按状态码统计
	var statusStats []struct {
		Status int   `json:"status"`
		Count  int64 `json:"count"`
	}
	if err := db.Model(&HttpRecordModel{}).
		Select("status, count(*) as count").
		Group("status").
		Order("count DESC").
		Find(&statusStats).Error; err != nil {
		return nil, err
	}
	stats["status_stats"] = statusStats

	// 按HTTP方法统计
	var methodStats []struct {
		Method string `json:"method"`
		Count  int64  `json:"count"`
	}
	if err := db.Model(&HttpRecordModel{}).
		Select("method, count(*) as count").
		Group("method").
		Order("count DESC").
		Find(&methodStats).Error; err != nil {
		return nil, err
	}
	stats["method_stats"] = methodStats

	// 按Host统计
	var hostStats []struct {
		Host  string `json:"host"`
		Count int64  `json:"count"`
	}
	if err := db.Model(&HttpRecordModel{}).
		Select("host, count(*) as count").
		Group("host").
		Order("count DESC").
		Limit(10).
		Find(&hostStats).Error; err != nil {
		return nil, err
	}
	stats["host_stats"] = hostStats

	return stats, nil
}

// GetHttpRecordsByHeaderValue 根据请求头值查询记录
func GetHttpRecordsByHeaderValue(headerKey, headerValue string, limit int) ([]HttpRecordModel, error) {
	db := GetPostgresDB()
	var records []HttpRecordModel

	if limit <= 0 {
		limit = 100
	}

	// 使用JSONB操作符查询
	err := db.Where("request_headers->>? = ?", headerKey, headerValue).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error

	return records, err
}

// GetHttpRecordsByResponseHeaderValue 根据响应头值查询记录
func GetHttpRecordsByResponseHeaderValue(headerKey, headerValue string, limit int) ([]HttpRecordModel, error) {
	db := GetPostgresDB()
	var records []HttpRecordModel

	if limit <= 0 {
		limit = 100
	}

	// 使用JSONB操作符查询
	err := db.Where("response_headers->>? = ?", headerKey, headerValue).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error

	return records, err
}

// GetHttpRecordsByUserAgent 根据User-Agent查询记录
func GetHttpRecordsByUserAgent(userAgent string, limit int) ([]HttpRecordModel, error) {
	return GetHttpRecordsByHeaderValue("User-Agent", userAgent, limit)
}

// GetHttpRecordsByContentType 根据Content-Type查询记录
func GetHttpRecordsByContentType(contentType string, limit int) ([]HttpRecordModel, error) {
	return GetHttpRecordsByHeaderValue("Content-Type", contentType, limit)
}

// GetHttpRecordsByAuthorization 根据Authorization头查询记录
func GetHttpRecordsByAuthorization(authValue string, limit int) ([]HttpRecordModel, error) {
	return GetHttpRecordsByHeaderValue("Authorization", authValue, limit)
}

// GetHttpRecordsByHeaderPattern 根据请求头模式查询记录
func GetHttpRecordsByHeaderPattern(headerKey, headerPattern string, limit int) ([]HttpRecordModel, error) {
	db := GetPostgresDB()
	var records []HttpRecordModel

	if limit <= 0 {
		limit = 100
	}

	// 使用JSONB操作符和LIKE查询
	err := db.Where("request_headers->>? ILIKE ?", headerKey, "%"+headerPattern+"%").
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error

	return records, err
}

// GetHttpRecordsStatsByHeaders 获取请求头统计信息
func GetHttpRecordsStatsByHeaders() (map[string]interface{}, error) {
	db := GetPostgresDB()
	stats := make(map[string]interface{})

	// User-Agent统计
	var userAgentStats []struct {
		UserAgent string `json:"user_agent"`
		Count     int64  `json:"count"`
	}
	if err := db.Model(&HttpRecordModel{}).
		Select("request_headers->>'User-Agent' as user_agent, count(*) as count").
		Where("request_headers->>'User-Agent' IS NOT NULL").
		Group("request_headers->>'User-Agent'").
		Order("count DESC").
		Limit(10).
		Find(&userAgentStats).Error; err != nil {
		return nil, err
	}
	stats["user_agent_stats"] = userAgentStats

	// Content-Type统计
	var contentTypeStats []struct {
		ContentType string `json:"content_type"`
		Count       int64  `json:"count"`
	}
	if err := db.Model(&HttpRecordModel{}).
		Select("request_headers->>'Content-Type' as content_type, count(*) as count").
		Where("request_headers->>'Content-Type' IS NOT NULL").
		Group("request_headers->>'Content-Type'").
		Order("count DESC").
		Limit(10).
		Find(&contentTypeStats).Error; err != nil {
		return nil, err
	}
	stats["content_type_stats"] = contentTypeStats

	return stats, nil
}

// InitHttpRecordsTable 初始化HTTP记录表
func InitHttpRecordsTable() error {
	db := GetPostgresDB()

	// 检查表是否存在
	if !db.Migrator().HasTable(&HttpRecordModel{}) {
		// 注释掉表创建日志，减少日志输出
		// log.Println("HTTP records table does not exist, creating...")
	}

	// 自动迁移表结构（如果表不存在会创建，如果存在会更新结构）
	if err := db.AutoMigrate(&HttpRecordModel{}); err != nil {
		return fmt.Errorf("failed to migrate HTTP records table: %v", err)
	}

	// 创建复合索引
	if err := createCompositeIndexes(db); err != nil {
		return fmt.Errorf("failed to create composite indexes: %v", err)
	}

	// 注释掉初始化成功日志，减少日志输出
	// log.Println("HTTP records table initialized successfully")
	return nil
}

// createCompositeIndexes 创建复合索引
func createCompositeIndexes(db *gorm.DB) error {
	// 复合索引列表
	indexes := []struct {
		name string
		sql  string
	}{
		{
			name: "idx_http_records_inner_token_created_at",
			sql:  "CREATE INDEX IF NOT EXISTS idx_http_records_inner_token_created_at ON http_records(inner_token_id, created_at DESC)",
		},
		{
			name: "idx_http_records_host_method",
			sql:  "CREATE INDEX IF NOT EXISTS idx_http_records_host_method ON http_records(host, method)",
		},
		{
			name: "idx_http_records_status_created_at",
			sql:  "CREATE INDEX IF NOT EXISTS idx_http_records_status_created_at ON http_records(status, created_at DESC)",
		},
		{
			name: "idx_http_records_method_status",
			sql:  "CREATE INDEX IF NOT EXISTS idx_http_records_method_status ON http_records(method, status)",
		},
		{
			name: "idx_http_records_host_status_created_at",
			sql:  "CREATE INDEX IF NOT EXISTS idx_http_records_host_status_created_at ON http_records(host, status, created_at DESC)",
		},
		{
			name: "idx_http_records_url_pattern",
			sql:  "CREATE INDEX IF NOT EXISTS idx_http_records_url_pattern ON http_records USING gin(to_tsvector('english', url))",
		},
		{
			name: "idx_http_records_request_headers_gin",
			sql:  "CREATE INDEX IF NOT EXISTS idx_http_records_request_headers_gin ON http_records USING gin(request_headers)",
		},
		{
			name: "idx_http_records_response_headers_gin",
			sql:  "CREATE INDEX IF NOT EXISTS idx_http_records_response_headers_gin ON http_records USING gin(response_headers)",
		},
	}

	// 执行创建索引
	for _, idx := range indexes {
		if err := db.Exec(idx.sql).Error; err != nil {
			// 注释掉索引创建日志，减少日志输出
			// log.Printf("Warning: Failed to create index %s: %v", idx.name, err)
			// 不返回错误，因为索引可能已经存在
		} else {
			// 注释掉索引创建成功日志，减少日志输出
			// log.Printf("Created index: %s", idx.name)
		}
	}

	return nil
}
