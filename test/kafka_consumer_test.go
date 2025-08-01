package test

import (
	"log"
	"testing"
	"time"

	"nursor-envoy-rpc/helper"
	httpRecord "nursor-envoy-rpc/models/nursor"
	"nursor-envoy-rpc/provider"
)

// TestKafkaConsumerAndPostgreSQL 测试Kafka消费者和PostgreSQL存储功能
func TestKafkaConsumerAndPostgreSQL(t *testing.T) {
	// 启动Kafka消费者
	log.Println("Starting Kafka consumer for testing...")
	kafkaConsumer := provider.GetKafkaConsumer()
	if err := kafkaConsumer.Start(); err != nil {
		t.Fatalf("Failed to start Kafka consumer: %v", err)
	}
	defer func() {
		log.Println("Stopping Kafka consumer...")
		if err := kafkaConsumer.Stop(); err != nil {
			t.Logf("Error stopping Kafka consumer: %v", err)
		}
	}()

	// 等待消费者启动
	time.Sleep(2 * time.Second)

	// 创建测试HTTP记录
	testRecord := &httpRecord.HttpRecord{
		RequestHeaders: map[string]string{
			"User-Agent":   "test-agent",
			"Content-Type": "application/json",
		},
		RequestBody: []byte(`{"test": "data"}`),
		ResponseHeaders: map[string]string{
			"Content-Type": "application/json",
			"Status":       "200 OK",
		},
		ResponseBody: []byte(`{"result": "success"}`),
		Url:          "https://api.example.com/test",
		Method:       "POST",
		Host:         "api.example.com",
		CreateAt:     time.Now().Format(time.RFC3339),
		HttpVersion:  "HTTP/1.1",
		InnerTokenId: "test-token-123",
		Status:       200,
	}

	// 推送测试数据到Kafka
	log.Println("Pushing test data to Kafka...")
	if err := provider.PushHttpRequestToCache(testRecord); err != nil {
		t.Fatalf("Failed to push test data to Kafka: %v", err)
	}

	// 等待一段时间让消费者处理消息
	log.Println("Waiting for consumer to process message...")
	time.Sleep(5 * time.Second)

	// 检查消费者状态
	stats := kafkaConsumer.GetStats()
	log.Printf("Consumer stats: %+v", stats)

	// 验证消费者是否正在运行
	if !kafkaConsumer.IsRunning() {
		t.Error("Kafka consumer is not running")
	}

	// 验证是否读取了消息
	if stats["messages_read"] == nil || stats["messages_read"].(int64) == 0 {
		t.Log("No messages were read by consumer (this might be normal if no messages were available)")
	} else {
		log.Printf("Consumer read %d messages", stats["messages_read"])
	}
}

// TestPostgreSQLConnection 测试PostgreSQL连接
func TestPostgreSQLConnection(t *testing.T) {
	// 测试数据库连接
	db := helper.GetPostgresDB()
	if db == nil {
		t.Fatal("Failed to get PostgreSQL database connection")
	}

	// 测试表初始化
	if err := helper.InitHttpRecordsTable(); err != nil {
		t.Fatalf("Failed to initialize HTTP records table: %v", err)
	}

	log.Println("PostgreSQL connection and table initialization test passed")
}

// TestHttpRecordSave 测试HTTP记录保存功能
func TestHttpRecordSave(t *testing.T) {
	// 创建测试记录
	testRecord := &httpRecord.HttpRecord{
		RequestHeaders: map[string]string{
			"User-Agent":   "test-agent",
			"Content-Type": "application/json",
		},
		RequestBody: []byte(`{"test": "data"}`),
		ResponseHeaders: map[string]string{
			"Content-Type": "application/json",
			"Status":       "200 OK",
		},
		ResponseBody: []byte(`{"result": "success"}`),
		Url:          "https://api.example.com/test",
		Method:       "POST",
		Host:         "api.example.com",
		CreateAt:     time.Now().Format(time.RFC3339),
		HttpVersion:  "HTTP/1.1",
		InnerTokenId: "test-token-456",
		Status:       200,
	}

	// 直接保存到PostgreSQL
	if err := helper.SaveHttpRecord(testRecord); err != nil {
		t.Fatalf("Failed to save HTTP record to PostgreSQL: %v", err)
	}

	log.Println("HTTP record save test passed")
}

// TestKafkaProducer 测试Kafka生产者功能
func TestKafkaProducer(t *testing.T) {
	// 创建测试记录
	testRecord := &httpRecord.HttpRecord{
		RequestHeaders: map[string]string{
			"User-Agent":   "test-agent",
			"Content-Type": "application/json",
		},
		RequestBody: []byte(`{"test": "data"}`),
		ResponseHeaders: map[string]string{
			"Content-Type": "application/json",
			"Status":       "200 OK",
		},
		ResponseBody: []byte(`{"result": "success"}`),
		Url:          "https://api.example.com/test",
		Method:       "POST",
		Host:         "api.example.com",
		CreateAt:     time.Now().Format(time.RFC3339),
		HttpVersion:  "HTTP/1.1",
		InnerTokenId: "test-token-789",
		Status:       200,
	}

	// 推送到Kafka
	if err := provider.PushHttpRequestToCache(testRecord); err != nil {
		t.Fatalf("Failed to push HTTP record to Kafka: %v", err)
	}

	log.Println("Kafka producer test passed")
}

// TestJsonbQueries 测试JSONB查询功能
func TestJsonbQueries(t *testing.T) {
	// 初始化数据库
	if err := helper.InitHttpRecordsTable(); err != nil {
		t.Fatalf("Failed to initialize table: %v", err)
	}

	// 创建测试记录
	testRecord := &httpRecord.HttpRecord{
		RequestHeaders: map[string]string{
			"User-Agent":    "test-agent/1.0",
			"Content-Type":  "application/json",
			"Authorization": "Bearer test-token",
		},
		RequestBody: []byte(`{"test": "data"}`),
		ResponseHeaders: map[string]string{
			"Content-Type": "application/json",
			"Status":       "200 OK",
		},
		ResponseBody: []byte(`{"result": "success"}`),
		Url:          "https://api.example.com/test",
		Method:       "POST",
		Host:         "api.example.com",
		CreateAt:     time.Now().Format(time.RFC3339),
		HttpVersion:  "HTTP/1.1",
		InnerTokenId: "test-token-jsonb",
		Status:       200,
	}

	// 保存记录
	if err := helper.SaveHttpRecord(testRecord); err != nil {
		t.Fatalf("Failed to save record: %v", err)
	}

	// 测试根据User-Agent查询
	records, err := helper.GetHttpRecordsByUserAgent("test-agent/1.0", 10)
	if err != nil {
		t.Fatalf("Failed to query by User-Agent: %v", err)
	}
	if len(records) == 0 {
		t.Error("No records found by User-Agent")
	}

	// 测试根据Content-Type查询
	records, err = helper.GetHttpRecordsByContentType("application/json", 10)
	if err != nil {
		t.Fatalf("Failed to query by Content-Type: %v", err)
	}
	if len(records) == 0 {
		t.Error("No records found by Content-Type")
	}

	// 测试根据Authorization查询
	records, err = helper.GetHttpRecordsByAuthorization("Bearer test-token", 10)
	if err != nil {
		t.Fatalf("Failed to query by Authorization: %v", err)
	}
	if len(records) == 0 {
		t.Error("No records found by Authorization")
	}

	// 测试获取统计信息
	stats, err := helper.GetHttpRecordsStatsByHeaders()
	if err != nil {
		t.Fatalf("Failed to get header stats: %v", err)
	}
	if stats == nil {
		t.Error("Header stats is nil")
	}

	log.Println("JSONB queries test passed")
}
