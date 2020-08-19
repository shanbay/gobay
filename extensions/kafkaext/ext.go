package kafkaext

import (
	"context"
	"github.com/shanbay/gobay"
	"github.com/segmentio/kafka-go"
	"log"
	"time"
)

type KafkaExt struct {
	app         *gobay.Application
	topic		string
	broker		string
	conn		*kafka.Conn
}

func (t *KafkaExt) Object() interface{} {
	return t
}

func (t *KafkaExt) Application() *gobay.Application {
	return t.app
}

func (t *KafkaExt)Init(app *gobay.Application) error {
	t.app = app
	config := app.Config()
	t.topic = config.GetString("kafka_topic")
	t.broker = config.GetString("kafka_broker")
	var err error
	t.conn, err = kafka.DialLeader(context.Background(), "tcp", "localhost:9092", t.topic, 0)
	if err != nil {
		return err
	}
	return nil
}

func (t *KafkaExt)Close() error {
	err := t.conn.Close()
	if err != nil {
		return err
	}
	return nil
}

func (t *KafkaExt)WriteMessages(mesgs ...kafka.Message) {
	err := t.conn.SetWriteDeadline(time.Now().Add(10*time.Second))
	if err != nil {
		log.Fatal("SetWriteDeadline failed")
	}
	_, err = t.conn.WriteMessages(mesgs...)
	if err != nil {
		log.Fatal("WriteMessages failed")
	}
}

func (t *KafkaExt)ReadMessage(maxBytes int) (kafka.Message, error) {
	err := t.conn.SetReadDeadline(time.Now().Add(10*time.Second))
	if err != nil {
		log.Fatal("SetReadDeadline failed")
	}
	msg, err := t.conn.ReadMessage(maxBytes)
	if err != nil {
		log.Fatal("ReadMessage failed")
	}
	return msg, err
}