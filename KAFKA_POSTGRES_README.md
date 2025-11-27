# Kafka消费者和PostgreSQL存储功能

## 功能概述

本项目新增了Kafka消费者功能，用于监听Kafka队列中的HTTP记录数据，并将其自动转存到PostgreSQL数据库中。

## 架构说明

```
HTTP请求 → Envoy代理 → 本服务 → Kafka队列 → Kafka消费者 → PostgreSQL数据库
```

1. **数据流**：
   - HTTP请求经过Envoy代理处理
   - 本服务将HTTP记录推送到Kafka队列
   - Kafka消费者持续监听队列
   - 消费者将数据保存到PostgreSQL数据库

2. **组件**：
   - `provider/persistent_http.go`: Kafka生产者，推送数据到队列
   - `provider/kafka_consumer.go`: Kafka消费者，监听队列并处理数据
   - `helper/postgres_service.go`: PostgreSQL数据库连接和存储服务

## 配置说明

### 环境变量

#### PostgreSQL配置
```bash
# PostgreSQL连接配置
POSTGRES_HOST=172.16.238.2      # PostgreSQL主机地址
POSTGRES_PORT=31279             # PostgreSQL端口
POSTGRES_USER=postgres          # 数据库用户名
POSTGRES_PASSWORD=asd123456     # 数据库密码
POSTGRES_DATABASE=nursor_http_records  # 数据库名称
POSTGRES_TIMEZONE=UTC           # 时区设置（可选，默认为UTC）
```

#### Kafka配置
```bash
# Kafka配置（在代码中硬编码，可根据需要修改）
KAFKA_BROKER=172.16.238.2:30631  # Kafka代理地址
KAFKA_TOPIC=http-records         # Kafka主题名称
```

### 数据库表结构

PostgreSQL中的`http_records`表结构（已优化）：

```sql
CREATE TABLE http_records (
    id BIGSERIAL PRIMARY KEY,
    request_headers JSONB,
    request_body BYTEA,
    response_headers JSONB,
    response_body BYTEA,
    url TEXT,
    method VARCHAR(10),
    host VARCHAR(255),
    create_at VARCHAR(50),
    http_version VARCHAR(10),
    inner_token_id VARCHAR(255),
    status INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 单列索引
CREATE INDEX idx_http_records_url ON http_records(url);
CREATE INDEX idx_http_records_method ON http_records(method);
CREATE INDEX idx_http_records_host ON http_records(host);
CREATE INDEX idx_http_records_inner_token_id ON http_records(inner_token_id);
CREATE INDEX idx_http_records_status ON http_records(status);
CREATE INDEX idx_http_records_created_at ON http_records(created_at);

-- 复合索引
CREATE INDEX idx_http_records_inner_token_created_at ON http_records(inner_token_id, created_at DESC);
CREATE INDEX idx_http_records_host_method ON http_records(host, method);
CREATE INDEX idx_http_records_status_created_at ON http_records(status, created_at DESC);
CREATE INDEX idx_http_records_method_status ON http_records(method, status);
CREATE INDEX idx_http_records_host_status_created_at ON http_records(host, status, created_at DESC);

-- 全文搜索索引
CREATE INDEX idx_http_records_url_pattern ON http_records USING gin(to_tsvector('english', url));

-- JSONB索引
CREATE INDEX idx_http_records_request_headers_gin ON http_records USING gin(request_headers);
CREATE INDEX idx_http_records_response_headers_gin ON http_records USING gin(response_headers);
```

### Headers存储优化

**使用map[string]string存储Headers的优势：**

1. **类型安全**：直接使用Go的map类型，无需JSON序列化/反序列化
2. **查询便利**：可以使用PostgreSQL的JSONB操作符进行高效查询
3. **存储效率**：JSONB格式比普通JSON更紧凑，查询更快
4. **索引支持**：可以创建GIN索引支持复杂查询

**示例查询：**
```sql
-- 根据User-Agent查询
SELECT * FROM http_records WHERE request_headers->>'User-Agent' = 'Mozilla/5.0...';

-- 根据Content-Type查询
SELECT * FROM http_records WHERE request_headers->>'Content-Type' LIKE '%json%';

-- 统计User-Agent分布
SELECT request_headers->>'User-Agent' as user_agent, count(*) 
FROM http_records 
WHERE request_headers->>'User-Agent' IS NOT NULL 
GROUP BY request_headers->>'User-Agent';
```

**潜在缺点：**
1. **存储空间**：JSONB格式可能比普通字符串稍大
2. **查询复杂度**：需要学习PostgreSQL的JSONB操作符
3. **版本兼容性**：需要PostgreSQL 9.4+版本

### 时区配置

PostgreSQL连接默认使用UTC时区，确保跨时区的一致性。可以通过环境变量`POSTGRES_TIMEZONE`来配置时区：

```bash
# 使用UTC时区（默认）
POSTGRES_TIMEZONE=UTC

# 使用其他时区（需要PostgreSQL支持）
POSTGRES_TIMEZONE=America/New_York
POSTGRES_TIMEZONE=Europe/London
```

**注意事项：**
1. 确保PostgreSQL服务器支持指定的时区
2. 如果时区不存在，连接会失败
3. 建议使用UTC时区以确保一致性

**在查询时转换时区**：
```sql
SELECT created_at AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Shanghai' as local_time 
FROM http_records;
```

## 使用方法

### 1. 启动服务

服务启动时会自动：
- 初始化PostgreSQL数据库连接
- 创建HTTP记录表（如果不存在）
- 启动Kafka消费者
- 开始监听Kafka队列

```bash
go run main.go
```

### 2. 查看日志

启动后可以看到类似以下日志：

```
Starting Kafka consumer...
PostgreSQL connection established successfully
HTTP records table initialized successfully
Kafka consumer initialized successfully
Kafka consumer started successfully
Starting ext_proc gRPC server on :8080...
```

### 3. 数据流验证

当有HTTP请求经过时，会看到类似以下日志：

```
Processing message: offset=123, partition=0
Successfully processed and saved HTTP record: URL=https://api.example.com/test, Method=POST, Status=200
HTTP record saved to PostgreSQL: ID=1, URL=https://api.example.com/test, Method=POST, Status=200
```

## 测试

### 运行测试

```bash
# 测试PostgreSQL连接
go test -v ./test -run TestPostgreSQLConnection

# 测试HTTP记录保存
go test -v ./test -run TestHttpRecordSave

# 测试Kafka生产者
go test -v ./test -run TestKafkaProducer

# 测试完整流程
go test -v ./test -run TestKafkaConsumerAndPostgreSQL
```

### 手动测试

1. **推送测试数据到Kafka**：
```go
testRecord := &httpRecord.HttpRecord{
    RequestHeaders: map[string]string{"User-Agent": "test"},
    RequestBody: []byte(`{"test": "data"}`),
    ResponseHeaders: map[string]string{"Content-Type": "application/json"},
    ResponseBody: []byte(`{"result": "success"}`),
    Url: "https://api.example.com/test",
    Method: "POST",
    Host: "api.example.com",
    CreateAt: time.Now().Format(time.RFC3339),
    HttpVersion: "HTTP/1.1",
    InnerTokenId: "test-token-123",
    Status: 200,
}

err := provider.PushHttpRequestToCache(testRecord)
```

2. **检查PostgreSQL数据**：
```sql
SELECT * FROM http_records ORDER BY created_at DESC LIMIT 10;
```

## 监控和调试

### 消费者状态

可以通过以下方式获取消费者状态：

```go
kafkaConsumer := provider.GetKafkaConsumer()
stats := kafkaConsumer.GetStats()
fmt.Printf("Consumer stats: %+v\n", stats)
```

### 统计信息

消费者统计信息包括：
- `is_running`: 是否正在运行
- `messages_read`: 已读取消息数量
- `bytes_read`: 已读取字节数
- `errors`: 错误数量
- `rebalance_errors`: 重平衡错误数量
- `timeouts`: 超时次数
- `last_offset`: 最后读取的偏移量
- `last_partition`: 最后读取的分区

### 日志级别

可以通过修改GORM配置来调整数据库日志级别：

```go
config := &gorm.Config{
    Logger: logger.Default.LogMode(logger.Info), // 或 logger.Error, logger.Warn
}
```

## 故障排除

### 常见问题

1. **PostgreSQL连接失败**：
   - 检查环境变量配置
   - 确认PostgreSQL服务正在运行
   - 验证网络连接

2. **Kafka连接失败**：
   - 检查Kafka服务状态
   - 验证代理地址和端口
   - 确认主题存在

3. **表创建失败**：
   - 检查数据库权限
   - 确认数据库名称正确
   - 查看详细错误日志

### 优雅关闭

服务支持优雅关闭，会：
- 停止接收新的Kafka消息
- 等待正在处理的消息完成
- 关闭数据库连接
- 关闭Kafka连接

## 性能优化

### 配置建议

1. **Kafka消费者配置**：
   - `MinBytes`: 10KB（减少网络往返）
   - `MaxBytes`: 10MB（限制内存使用）
   - `MaxWait`: 1秒（平衡延迟和吞吐量）

2. **PostgreSQL连接池**：
   - `MaxIdleConns`: 10
   - `MaxOpenConns`: 100
   - `ConnMaxLifetime`: 1小时

3. **并发处理**：
   - 每个消息在独立的goroutine中处理
   - 使用WaitGroup确保所有消息处理完成

### 扩展性

- 支持多个消费者实例（通过消费者组）
- 可以水平扩展PostgreSQL（读写分离）
- Kafka分区支持并行处理 