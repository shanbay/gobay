package kafkaproducerext_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/shanbay/gobay/extensions/kafkaproducerext"

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

// ExampleKafkaProducerExt_Produce -
func Test_ExampleKafkaProducerExt_Produce(t *testing.T) {
	setup()
	defer tearDown()

	// setup client
	client := &kafkaproducerext.KafkaProducerExt{NS: "questkafkaproducer_"}
	exts := map[gobay.Key]gobay.Extension{
		"test_kafka_producer": client,
	}
	if _, err := gobay.CreateApp("../../testdata/kafkaproducer", "testing", exts); err != nil {
		fmt.Println(err)
		return
	}
	defer client.Close()

	// test produce message
	randomStr := fmt.Sprintf("asdfasdf%v", rand.Intn(1000))
	message := &sarama.ProducerMessage{Topic: kafkaTopic, Value: sarama.StringEncoder(randomStr)}
	(*client.Client()).Input() <- message

	// ------------------------- consume to check result ------------------------

	var consumeMsgMap = make(map[int]struct{})
	var resChan = make(chan int)
	go func() {
		for r := range resChan {
			consumeMsgMap[r] = struct{}{}
		}
	}()

	handler := NewSyncConsumerGroupHandler(func(data []byte) error {
		// log.Fatalf("got data %v", string(data))
		assert.Equal(t, randomStr, string(data))
		return nil
	})
	consumer, err := NewConsumerGroup(kafkaBroker, []string{kafkaTopic}, "sync-consumer-"+fmt.Sprintf("%d", time.Now().Unix()), handler)
	if err != nil {
		return
	}
	defer consumer.Close()
}

/* -------------------------------------------- helpers -------------------------------------------- */

type ConsumerGroupHandler interface {
	sarama.ConsumerGroupHandler
	WaitReady()
	Reset()
}

type ConsumerGroup struct {
	cg sarama.ConsumerGroup
}

func NewConsumerGroup(broker string, topics []string, group string, handler ConsumerGroupHandler) (*ConsumerGroup, error) {
	ctx := context.Background()
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V0_10_2_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	client, err := sarama.NewConsumerGroup([]string{broker}, group, cfg)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			err := client.Consume(ctx, topics, handler)
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

	return &ConsumerGroup{
		cg: client,
	}, nil
}

func (c *ConsumerGroup) Close() error {
	return c.cg.Close()
}

type ConsumerSessionMessage struct {
	Session sarama.ConsumerGroupSession
	Message *sarama.ConsumerMessage
}

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
