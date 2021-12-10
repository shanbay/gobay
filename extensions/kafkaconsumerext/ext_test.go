package kafkaconsumerext_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/shanbay/gobay/extensions/kafkaconsumerext"

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
func Test_ExampleKafkaConsumerExt_Produce(t *testing.T) {
	setup()
	defer tearDown()

	// setup client
	client := &kafkaconsumerext.KafkaConsumerExt{NS: "questkafkaconsumer_"}
	exts := map[gobay.Key]gobay.Extension{
		"test_kafka_consumer": client,
	}
	if _, err := gobay.CreateApp("../../testdata/kafkaconsumer", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}
	defer client.Close()

	limit := 101

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

	handler := NewSyncConsumerGroupHandler(func(data []byte) error {
		var msg TestKafkaMessage
		err := json.Unmarshal(data, &msg)
		if err != nil {
			return err
		}
		resChan <- msg.Id

		return nil
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

// --------------------------------------- consumer handler

type syncConsumerGroupHandler struct {
	ready chan bool

	cb func([]byte) error
}

func NewSyncConsumerGroupHandler(cb func([]byte) error) ConsumerGroupHandler {
	handler := syncConsumerGroupHandler{
		ready: make(chan bool, 0),
		cb:    cb,
	}
	return &handler
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (h *syncConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(h.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (h *syncConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *syncConsumerGroupHandler) WaitReady() {
	<-h.ready
	return
}

func (h *syncConsumerGroupHandler) Reset() {
	h.ready = make(chan bool, 0)
	return
}

func (h *syncConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {

	// NOTE:
	// Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/master/consumer_group.go#L27-L29
	claimMsgChan := claim.Messages()

	for message := range claimMsgChan {
		if h.cb(message.Value) == nil {
			session.MarkMessage(message, "")
		}
	}

	return nil
}

// -------------------------------- consumer

type ConsumerGroupHandler interface {
	sarama.ConsumerGroupHandler
	WaitReady()
	Reset()
}

type ConsumerGroup struct {
	cg sarama.ConsumerGroup
}

func (c *ConsumerGroup) Close() error {
	return c.cg.Close()
}

type ConsumerSessionMessage struct {
	Session sarama.ConsumerGroupSession
	Message *sarama.ConsumerMessage
}

func decodeMessage(data []byte) error {
	var msg TestKafkaMessage
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return err
	}
	return nil
}
