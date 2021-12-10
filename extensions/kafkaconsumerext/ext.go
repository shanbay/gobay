package kafkaconsumerext

import (
	"errors"
	"log"
	"os"
	"strings"

	"github.com/Shopify/sarama"
	"github.com/shanbay/gobay"
)

type KafkaConsumerExt struct {
	app    *gobay.Application
	NS     string
	client *sarama.ConsumerGroup
	Config *sarama.Config
}

var _ gobay.Extension = (*KafkaConsumerExt)(nil)

// Init -
func (e *KafkaConsumerExt) Init(app *gobay.Application) error {
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

	log.Println("Starting a new Sarama consumer")

	sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)

	versionStr := gobayConfig.GetString("version")
	assignor := gobayConfig.GetString("assignor") // default to range
	brokers := gobayConfig.GetString("brokers")   // comma delimited
	group := gobayConfig.GetString("group")
	fromBeginning := gobayConfig.GetBool("from_beginning")

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

	switch assignor {
	case "sticky":
		e.Config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategySticky
	case "roundrobin":
		e.Config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	case "range":
		e.Config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange
	default:
		log.Panicf("Unrecognized consumer group partition assignor: %s", assignor)
	}

	if fromBeginning {
		e.Config.Consumer.Offsets.Initial = sarama.OffsetOldest
	}

	client, err := sarama.NewConsumerGroup(strings.Split(brokers, ","), group, e.Config)
	if err != nil {
		log.Panicf("Error creating consumer group client: %v", err)
	}

	e.client = &client

	return nil
}

func (e *KafkaConsumerExt) Object() interface{} {
	return e
}

func (e *KafkaConsumerExt) Client() *sarama.ConsumerGroup {
	return e.client
}

// Close close client
func (e *KafkaConsumerExt) Close() error {
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
func (e *KafkaConsumerExt) Application() *gobay.Application {
	return e.app
}
