package broker

import (
	"context"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
)

type Kafka struct {
	Broker     string
	Topic      string
	Connection *kafka.Conn
	Key        string
	Payload    []byte
}

// NewConn makes a new connection to kafka broker
// returns an error if it fails
func (b *Kafka) NewConn(ctx context.Context) error {
	conn, err := kafka.DialLeader(ctx, "tcp", b.Broker, b.Topic, 0)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
		return err
	}
	defer conn.Close()

	fmt.Println("\t connected to Kafka")
	return nil
}

// Write produces a new topic
func (b *Kafka) Produce(ctx context.Context) error {
	// define and create a new Kafka writer
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(b.Broker),
		Topic:                  b.Topic,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}
	defer writer.Close()

	err := writer.WriteMessages(ctx,
		kafka.Message{
			Key:   []byte(b.Key),
			Value: b.Payload,
		},
	)
	if err != nil {
		log.Printf("write error: %v", err)
		return err
	}

	fmt.Printf("\n\t\t published  id= %s  \n", b.Key)
	return nil
}

func (b *Kafka) Consume(ctx context.Context) error {
	return nil
}
