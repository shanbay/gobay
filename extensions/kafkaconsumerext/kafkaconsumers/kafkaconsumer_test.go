package kafkaconsumers_test

import (
	"context"
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/shanbay/gobay/extensions/kafkaconsumerext"
	"github.com/shanbay/gobay/extensions/kafkaconsumerext/kafkaconsumers"

	"github.com/Shopify/sarama"
	"github.com/shanbay/gobay"
	"github.com/stretchr/testify/assert"
)

var (
	kafkaTopic  = "test-topic"
	kafkaBroker = "127.0.0.1:9092"
)

func setup() {
	// create clean topic
	clusterAdmin, _ := sarama.NewClusterAdmin([]string{kafkaBroker}, sarama.NewConfig())
	clusterAdmin.CreateTopic(kafkaTopic, &sarama.TopicDetail{}, false)
}

func tearDown() {
	// delete topic to clear queue
	clusterAdmin, _ := sarama.NewClusterAdmin([]string{kafkaBroker}, sarama.NewConfig())
	clusterAdmin.DeleteTopic(kafkaTopic)
}

// Test_ExampleKafkaProducerExt_Produce -
func Test_BatchConsumerGroup(t *testing.T) {
	setup()
	defer tearDown()

	// setup client
	client := &kafkaconsumerext.KafkaConsumerExt{NS: "questkafkaconsumer_"}
	exts := map[gobay.Key]gobay.Extension{
		"test_kafka_consumer": client,
	}
	if _, err := gobay.CreateApp("../../../testdata/kafkaconsumer", "testing", exts); err != nil {
		log.Fatalf("test failed to createapp")
		return
	}
	defer client.Close()

	limit := 25

	produceDone := testProduce(kafkaTopic, limit)

	var consumeMsgMap = make(map[int]struct{})
	var resChan = make(chan int)
	go func() {
		for r := range resChan {
			consumeMsgMap[r] = struct{}{}
		}
		if len(consumeMsgMap) >= limit {
			assert.Equal(t, limit, consumeMsgMap)
		}
	}()

	handler := kafkaconsumers.NewBatchConsumerGroupHandler(&kafkaconsumers.BatchConsumerConfig{
		MaxBufSize:            10,
		TickerIntervalSeconds: 1,
		Callback: func(messages []*kafkaconsumers.ConsumerSessionMessage) error {
			for i := range messages {

				var msg TestKafkaMessage
				err := json.Unmarshal(messages[i].Message.Value, &msg)
				if err != nil {
					return err
				}
				resChan <- msg.Id
				log.Printf("checked message %v", msg.Id)

				messages[i].Session.MarkMessage(messages[i].Message, "")
			}
			return nil
		},
	})

	//
	ctx := context.Background()

	go func() {
		for {
			err := (*client.Client()).Consume(ctx, []string{kafkaTopic}, handler)
			if err != nil {
				if err == sarama.ErrClosedConsumerGroup {
					break
				} else {
					panic(err)
				}
			}
			if ctx.Err() != nil {
				return
			}
			handler.Reset()
		}

	}()

	handler.WaitReady() // Await till the consumer has been set up
	//

	<-produceDone

	// ------------------------- consume to check result ------------------------
	time.Sleep(3 * time.Second) // wait for kafka message to be consumed

	// Note: this test will cause DATA RACE error, but it's a good test
	// assert.Equal(t, limit, len(consumeMsgMap))
}

// ----------------------------------- helper producer functions -------------------------------------
func testProduce(topic string, limit int) <-chan struct{} {
	var produceDone = make(chan struct{})

	p, err := sarama.NewAsyncProducer([]string{kafkaBroker}, sarama.NewConfig())
	if err != nil {
		return nil
	}

	go func() {
		defer close(produceDone)

		for i := 0; i < limit; i++ {
			msg := TestKafkaMessage{i}
			msgBytes, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			select {
			case p.Input() <- &sarama.ProducerMessage{
				Topic: topic,
				Value: sarama.ByteEncoder(msgBytes),
			}:
			case err := <-p.Errors():
				log.Fatalf("Failed to send message to kafka, err: %s, msg: %s\n", err, msgBytes)
			}
		}
		log.Printf("produced %v messages", limit)
	}()

	return produceDone
}

type Producer struct {
	p sarama.AsyncProducer
}

func NewProducer(broker string) (*Producer, error) {
	producer, err := sarama.NewAsyncProducer([]string{broker}, sarama.NewConfig())
	if err != nil {
		return nil, err
	}
	return &Producer{
		p: producer,
	}, nil
}

type TestKafkaMessage struct {
	Id int `json:"id"`
}

func (p *Producer) Close() error {
	if p != nil {
		return p.p.Close()
	}
	return nil
}
