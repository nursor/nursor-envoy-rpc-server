package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"nursor-envoy-rpc/helper"
	httpRecord "nursor-envoy-rpc/models/nursor"

	"github.com/segmentio/kafka-go"
)

var (
	kafkaConsumer *KafkaConsumer
	consumerOnce  sync.Once
)

// KafkaConsumer Kafka消费者结构体
type KafkaConsumer struct {
	reader     *kafka.Reader
	isRunning  bool
	stopChan   chan struct{}
	wg         sync.WaitGroup
	debugMode  bool          // 添加调试模式开关
	workerPool chan struct{} // 工作池，限制并发处理数量
}

// GetKafkaConsumer 获取Kafka消费者单例
func GetKafkaConsumer() *KafkaConsumer {
	consumerOnce.Do(func() {
		kafkaConsumer = &KafkaConsumer{
			stopChan:   make(chan struct{}),
			debugMode:  false,                  // 默认关闭调试模式
			workerPool: make(chan struct{}, 5), // 限制最多5个并发工作协程
		}
	})
	return kafkaConsumer
}

// SetDebugMode 设置调试模式
func (kc *KafkaConsumer) SetDebugMode(enabled bool) {
	kc.debugMode = enabled
}

// debugLog 根据调试模式输出日志
func (kc *KafkaConsumer) debugLog(format string, args ...interface{}) {
	if kc.debugMode {
		log.Printf(format, args...)
	}
}

// Initialize 初始化Kafka消费者
func (kc *KafkaConsumer) Initialize() error {
	// Kafka配置
	brokerAddr := "172.16.238.2:30631"
	topic := kafkaTopic

	// 检查Topic是否存在
	if err := kc.checkTopicExists(brokerAddr, topic); err != nil {
		log.Printf("Warning: Topic check failed: %v", err)
	}

	// 创建Kafka Reader - 使用简单消费者模式，不使用 Consumer Group
	kc.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{brokerAddr},
		Topic:   topic,
		// GroupID:         "http-records-consumer-group", // 注释掉 Group ID，使用简单消费者
		MinBytes:        1,               // 降低最小字节数，避免等待过多数据
		MaxBytes:        10e6,            // 10MB
		MaxWait:         5 * time.Second, // 增加等待时间
		ReadLagInterval: -1,
		// CommitInterval:  1 * time.Second, // 简单消费者不需要提交偏移量
		StartOffset: kafka.LastOffset, // 从最新消息开始消费
		// Logger:      kafka.LoggerFunc(log.Printf), // 注释掉Kafka日志，减少日志输出
	})

	// 注释掉初始化成功的日志，减少日志输出
	// log.Printf("Kafka consumer initialized successfully - Broker: %s, Topic: %s", brokerAddr, topic)
	return nil
}

// Start 启动Kafka消费者
func (kc *KafkaConsumer) Start() error {
	if kc.isRunning {
		log.Println("Kafka consumer is already running")
		return nil
	}

	if kc.reader == nil {
		if err := kc.Initialize(); err != nil {
			return err
		}
	}

	// 初始化PostgreSQL表
	if err := helper.InitHttpRecordsTable(); err != nil {
		log.Printf("Failed to initialize PostgreSQL table: %v", err)
		return err
	}

	kc.isRunning = true
	kc.wg.Add(1)

	go func() {
		defer kc.wg.Done()
		kc.consumeMessages()
	}()

	log.Println("Kafka consumer started successfully")
	return nil
}

// Stop 停止Kafka消费者
func (kc *KafkaConsumer) Stop() error {
	if !kc.isRunning {
		log.Println("Kafka consumer is not running")
		return nil
	}

	log.Println("Stopping Kafka consumer...")
	close(kc.stopChan)
	kc.isRunning = false

	if kc.reader != nil {
		if err := kc.reader.Close(); err != nil {
			log.Printf("Error closing Kafka reader: %v", err)
		}
	}

	kc.wg.Wait()
	log.Println("Kafka consumer stopped successfully")
	return nil
}

// consumeMessages 消费消息的主循环
func (kc *KafkaConsumer) consumeMessages() {
	ctx := context.Background()
	consecutiveErrors := 0
	maxConsecutiveErrors := 5

	for {
		select {
		case <-kc.stopChan:
			log.Println("Received stop signal, exiting consumer loop")
			return
		default:
			// 读取消息
			message, err := kc.reader.ReadMessage(ctx)
			if err != nil {
				consecutiveErrors++
				log.Printf("Error reading message from Kafka (attempt %d/%d): %v", consecutiveErrors, maxConsecutiveErrors, err)

				// 检查是否是 EOF 错误（没有消息）
				if err.Error() == "EOF" {
					// 注释掉 EOF 日志，减少日志输出
					// log.Println("No messages available in topic, waiting...")
					time.Sleep(5 * time.Second) // 等待更长时间
					continue
				}

				// 如果是连续错误过多，尝试重新初始化连接
				if consecutiveErrors >= maxConsecutiveErrors {
					log.Println("Too many consecutive errors, attempting to reinitialize connection...")
					if err := kc.reinitializeConnection(); err != nil {
						log.Printf("Failed to reinitialize connection: %v", err)
						time.Sleep(5 * time.Second)
					} else {
						consecutiveErrors = 0
						log.Println("Connection reinitialized successfully")
					}
				}

				time.Sleep(2 * time.Second) // 增加重试间隔
				continue
			}

			// 成功读取消息，重置错误计数
			consecutiveErrors = 0
			kc.debugLog("Successfully read message from Kafka: offset=%d, partition=%d", message.Offset, message.Partition)

			// 处理消息 - 使用工作池限制并发数
			kc.wg.Add(1)
			go func(msg kafka.Message) {
				defer kc.wg.Done()

				// 获取工作池槽位
				kc.workerPool <- struct{}{}
				defer func() { <-kc.workerPool }()

				kc.processMessage(msg)
			}(message)
		}
	}
}

// processMessage 处理单条消息
func (kc *KafkaConsumer) processMessage(message kafka.Message) {
	kc.debugLog("Processing message: offset=%d, partition=%d", message.Offset, message.Partition)

	// 解析HTTP记录
	var httpRecord httpRecord.HttpRecord
	if err := json.Unmarshal(message.Value, &httpRecord); err != nil {
		log.Printf("Error unmarshaling HTTP record: %v", err)
		return
	}

	// 保存到PostgreSQL
	if err := helper.SaveHttpRecord(&httpRecord); err != nil {
		log.Printf("Error saving HTTP record to PostgreSQL: %v", err)
		return
	}

	kc.debugLog("Successfully processed and saved HTTP record: URL=%s, Method=%s, Status=%d",
		httpRecord.Url, httpRecord.Method, httpRecord.Status)
}

// IsRunning 检查消费者是否正在运行
func (kc *KafkaConsumer) IsRunning() bool {
	return kc.isRunning
}

// checkTopicExists 检查Topic是否存在
func (kc *KafkaConsumer) checkTopicExists(brokerAddr, topic string) error {
	conn, err := kafka.Dial("tcp", brokerAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka broker: %v", err)
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		return fmt.Errorf("failed to read partitions for topic %s: %v", topic, err)
	}

	if len(partitions) == 0 {
		return fmt.Errorf("topic %s does not exist or has no partitions", topic)
	}

	log.Printf("Topic %s exists with %d partitions", topic, len(partitions))
	return nil
}

// reinitializeConnection 重新初始化Kafka连接
func (kc *KafkaConsumer) reinitializeConnection() error {
	log.Println("Reinitializing Kafka connection...")

	// 关闭现有连接
	if kc.reader != nil {
		if err := kc.reader.Close(); err != nil {
			log.Printf("Error closing existing reader: %v", err)
		}
	}

	// 重新初始化
	return kc.Initialize()
}

// GetStats 获取消费者统计信息
func (kc *KafkaConsumer) GetStats() map[string]interface{} {
	if kc.reader == nil {
		return map[string]interface{}{
			"is_running": kc.isRunning,
			"reader":     "not_initialized",
		}
	}

	stats := kc.reader.Stats()
	return map[string]interface{}{
		"is_running":       kc.isRunning,
		"messages_read":    stats.Messages,
		"bytes_read":       stats.Bytes,
		"errors":           stats.Errors,
		"rebalance_errors": stats.Rebalances,
		"timeouts":         stats.Timeouts,
		"last_offset":      stats.Offset,
		"last_partition":   stats.Partition,
	}
}
