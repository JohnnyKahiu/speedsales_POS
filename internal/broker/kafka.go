package broker

import (
	"github.com/segmentio/kafka-go"
)

type Kafka struct {
	Broker     string
	Topic      string
	GroupID    string
	Connection *kafka.Conn
	Key        string
	Payload    []byte
}
