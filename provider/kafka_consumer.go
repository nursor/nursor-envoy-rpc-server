package provider

import (
	"context"
	"encoding/json"
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
	reader    *kafka.Reader
	isRunning bool
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// GetKafkaConsumer 获取Kafka消费者单例
func GetKafkaConsumer() *KafkaConsumer {
	consumerOnce.Do(func() {
		kafkaConsumer = &KafkaConsumer{
			stopChan: make(chan struct{}),
		}
	})
	return kafkaConsumer
}

// Initialize 初始化Kafka消费者
func (kc *KafkaConsumer) Initialize() error {
	// Kafka配置
	brokerAddr := "172.16.238.2:30631"
	topic := kafkaTopic

	// 创建Kafka Reader
	kc.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:         []string{brokerAddr},
		Topic:           topic,
		GroupID:         "http-records-consumer-group",
		MinBytes:        10e3, // 10KB
		MaxBytes:        10e6, // 10MB
		MaxWait:         1 * time.Second,
		ReadLagInterval: -1,
		CommitInterval:  1 * time.Second,
	})

	log.Println("Kafka consumer initialized successfully")
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

	for {
		select {
		case <-kc.stopChan:
			log.Println("Received stop signal, exiting consumer loop")
			return
		default:
			// 读取消息
			message, err := kc.reader.ReadMessage(ctx)
			if err != nil {
				log.Printf("Error reading message from Kafka: %v", err)
				time.Sleep(1 * time.Second) // 避免无限循环
				continue
			}

			// 处理消息
			kc.wg.Add(1)
			go func(msg kafka.Message) {
				defer kc.wg.Done()
				kc.processMessage(msg)
			}(message)
		}
	}
}

// processMessage 处理单条消息
func (kc *KafkaConsumer) processMessage(message kafka.Message) {
	log.Printf("Processing message: offset=%d, partition=%d", message.Offset, message.Partition)

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

	log.Printf("Successfully processed and saved HTTP record: URL=%s, Method=%s, Status=%d",
		httpRecord.Url, httpRecord.Method, httpRecord.Status)
}

// IsRunning 检查消费者是否正在运行
func (kc *KafkaConsumer) IsRunning() bool {
	return kc.isRunning
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
