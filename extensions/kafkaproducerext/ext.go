package kafkaproducerext

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Shopify/sarama"
	"github.com/getsentry/sentry-go"
	"github.com/shanbay/gobay"
)

type KafkaProducerExt struct {
	app    *gobay.Application
	NS     string
	client *sarama.AsyncProducer
	Config *sarama.Config
}

var _ gobay.Extension = (*KafkaProducerExt)(nil)

// Init -
func (e *KafkaProducerExt) Init(app *gobay.Application) error {
	var err error
	if e.NS == "" {
		return errors.New("lack of NS")
	}
	e.app = app
	gobayConfig := gobay.GetConfigByPrefix(app.Config(), e.NS, true)

	enabled := gobayConfig.GetString("enabled")
	if enabled != "true" {
		return nil
	}

	log.Println("Starting a new Sarama producer")

	sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)

	versionStr := gobayConfig.GetString("version")
	brokers := gobayConfig.GetString("brokers") // comma delimited

	version, err := sarama.ParseKafkaVersion(versionStr)
	if err != nil {
		log.Panicf("Error parsing Kafka version: %v", err)
	}

	/**
	 * Construct a new Sarama configuration.
	 * The Kafka cluster version has to be defined before the consumer/producer is initialized.
	 */
	e.Config = sarama.NewConfig()
	e.Config.Version = version
	// 配置回调信息
	e.Config.Producer.Return.Errors = true
	e.Config.Producer.Return.Successes = true
	// 配置 producer 端防丢失策略（重试没有单独配，用默认的暂时没有问题）
	e.Config.Producer.RequiredAcks = sarama.WaitForAll
	e.Config.Producer.Partitioner = sarama.NewRandomPartitioner

	client, err := sarama.NewAsyncProducer(strings.Split(brokers, ","), e.Config)
	if err != nil {
		log.Panicf("Error creating producer client: %v", err)
	}

	e.client = &client

	go func() {
		for {
			select {
			case <-(*e.client).Successes():
				// log.Printf("topic: %s - key: %s - offset: %d, success published!", s.Topic, s.Key, s.Offset)
			case e := <-(*e.client).Errors():
				if e != nil {
					errorMsg := fmt.Errorf("topic: %s publish failed: %s", e.Msg.Topic, e.Error())
					sentry.CaptureException(errorMsg)
				}
			}
		}
	}()

	return nil
}

func (e *KafkaProducerExt) Object() interface{} {
	return e
}

func (e *KafkaProducerExt) Client() *sarama.AsyncProducer {
	return e.client
}

// Close close client
func (e *KafkaProducerExt) Close() error {
	if e.client == nil {
		return nil
	}
	if err := (*e.client).Close(); err != nil {
		log.Panicf("Error closing client: %v", err)
		return err
	}
	return nil
}

// Application
func (e *KafkaProducerExt) Application() *gobay.Application {
	return e.app
}
