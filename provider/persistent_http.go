package provider

import (
	"context"
	"encoding/json"
	"log"
	httpRecord "nursor-envoy-rpc/models/nursor"

	"github.com/segmentio/kafka-go"
)

var kafkaTopic = "http-records"

var kafkaWriter *kafka.Writer

func PushHttpRequestToDB(req *httpRecord.HttpRecord) error {

	writer := GetKafkaWriter()

	recordJson, err := json.Marshal(req)
	if err != nil {
		return err
	}
	ctx := context.Background()
	err = writer.WriteMessages(ctx,
		kafka.Message{Value: recordJson},
	)
	if err != nil {
		log.Println("Error writing message to Kafka:", err)
		return err
	}
	log.Println("Message sent to Kafka:")
	return nil

}

func GetKafkaWriter() *kafka.Writer {
	if kafkaWriter != nil {
		return kafkaWriter
	}

	brokerAddr := "172.16.238.2:30631"
	topic := kafkaTopic
	kafkaWriter = kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{brokerAddr},
		Topic:   topic,
	})
	return kafkaWriter

}
