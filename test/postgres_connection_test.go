package test

import (
	"log"
	"testing"
	"time"

	"nursor-envoy-rpc/helper"
)

// TestPostgreSQLConnectionSimple 测试PostgreSQL连接（简单版本）
func TestPostgreSQLConnectionSimple(t *testing.T) {
	// 测试数据库连接
	db := helper.GetPostgresDB()
	if db == nil {
		t.Fatal("Failed to get PostgreSQL database connection")
	}

	// 测试基本查询
	var result int
	if err := db.Raw("SELECT 1").Scan(&result).Error; err != nil {
		t.Fatalf("Failed to execute basic query: %v", err)
	}

	if result != 1 {
		t.Errorf("Expected result 1, got %d", result)
	}

	log.Println("PostgreSQL connection test passed")
}

// TestTableCreation 测试表创建
func TestTableCreation(t *testing.T) {
	// 初始化表
	if err := helper.InitHttpRecordsTable(); err != nil {
		t.Fatalf("Failed to initialize table: %v", err)
	}

	log.Println("Table creation test passed")
}

// TestBasicCRUD 测试基本的CRUD操作
func TestBasicCRUD(t *testing.T) {
	// 创建测试记录
	testRecord := &helper.HttpRecordModel{
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
		URL:          "https://api.example.com/test",
		Method:       "POST",
		Host:         "api.example.com",
		CreateAt:     time.Now().Format(time.RFC3339),
		HttpVersion:  "HTTP/1.1",
		InnerTokenID: "test-token-crud",
		Status:       200,
	}

	// 保存记录
	db := helper.GetPostgresDB()
	if err := db.Create(testRecord).Error; err != nil {
		t.Fatalf("Failed to create record: %v", err)
	}

	// 查询记录
	var foundRecord helper.HttpRecordModel
	if err := db.Where("inner_token_id = ?", "test-token-crud").First(&foundRecord).Error; err != nil {
		t.Fatalf("Failed to query record: %v", err)
	}

	// 验证记录
	if foundRecord.URL != testRecord.URL {
		t.Errorf("Expected URL %s, got %s", testRecord.URL, foundRecord.URL)
	}

	if foundRecord.Method != testRecord.Method {
		t.Errorf("Expected Method %s, got %s", testRecord.Method, foundRecord.Method)
	}

	// 删除记录
	if err := db.Delete(&foundRecord).Error; err != nil {
		t.Fatalf("Failed to delete record: %v", err)
	}

	log.Println("Basic CRUD test passed")
}
