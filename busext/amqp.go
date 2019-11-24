package busext

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/streadway/amqp"

	log "github.com/sirupsen/logrus"

	"github.com/shanbay/gobay"
)

var (
	errNotConnected  = errors.New("not connected to a server")
	errAlreadyClosed = errors.New("already closed: not connected to the server")
	errShutdown      = errors.New("BusExt is closed")
)

const (
	defaultResendDelay    = "1s"
	defaultReconnectDelay = "2s"
	defaultReinitDelay    = "1s"
	defaultPrefetch       = 100
	defaultPublishRetry   = 3
)

type BusExt struct {
	NS              string
	app             *gobay.Application
	connection      *amqp.Connection
	channel         *amqp.Channel
	done            chan bool
	notifyConnClose chan *amqp.Error
	notifyChanClose chan *amqp.Error
	notifyConfirm   chan amqp.Confirmation
	isReady         bool
	config          *viper.Viper
	consumers       map[string]Handler
	consumeChannels map[string]<-chan amqp.Delivery
	publishRetry    int
	prefetch        int
	resendDelay     time.Duration
	reconnectDelay  time.Duration
	reinitDelay     time.Duration
}

func (b *BusExt) Object() interface{} {
	return b.channel
}

func (b *BusExt) Application() *gobay.Application {
	return b.app
}

func (b *BusExt) Init(app *gobay.Application) error {
	b.app = app
	b.config = app.Config()
	if b.NS != "" {
		b.config = b.config.Sub(b.NS)
	}
	setDefaultConfig(b.config)
	b.consumers = make(map[string]Handler)
	b.consumeChannels = make(map[string]<-chan amqp.Delivery)
	b.prefetch = b.config.GetInt("prefetch")
	b.publishRetry = b.config.GetInt("publish_retry")
	b.resendDelay = b.config.GetDuration("resend_delay")
	b.reconnectDelay = b.config.GetDuration("reconnect_delay")
	b.reinitDelay = b.config.GetDuration("reinit_delay")
	brokerUrl := b.config.GetString("broker_url")
	go b.handleReconnect(brokerUrl)
	for {
		if !b.isReady {
			continue
		} else {
			break
		}
	}
	log.Info("BusExt init done")
	return nil
}

func (b *BusExt) Close() error {
	if !b.isReady {
		return errAlreadyClosed
	}
	if err := b.channel.Close(); err != nil {
		log.Errorf("close channel failed: %v", err)
		return err
	}
	if err := b.connection.Close(); err != nil {
		log.Errorf("close connection failed: %v", err)
		return err
	}
	close(b.done)
	b.isReady = false
	log.Info("BusExt closed")
	return nil
}

func (b *BusExt) Push(exchange, routingKey string, data amqp.Publishing) error {
	log.Debugf("Trying to publish: %+v", data)
	if !b.isReady {
		err := errors.New("BusExt is not ready")
		log.Errorf("Can not publish message: %v", err)
		return err
	}
	for i := 0; i < b.publishRetry; i++ {
		err := b.UnsafePush(exchange, routingKey, data)
		if err != nil {
			log.Errorf("UnsafePush failed: %v", err)
			select {
			case <-b.done:
				log.Error("BusExt closed during publishing message")
				return errShutdown
			case <-time.After(b.resendDelay):
			}
			continue
		}
		select {
		case confirm := <-b.notifyConfirm:
			if confirm.Ack {
				log.Debug("Publish confirmed!")
				return nil
			}
		case <-time.After(b.resendDelay):
		}
		log.Warnf("Publish not confirmed after %f seconds. Retrying...",
			b.resendDelay.Seconds())
	}
	err := fmt.Errorf(
		"publishing message failed after retry %d times", b.publishRetry)
	log.Error(err)
	return err
}

func (b *BusExt) UnsafePush(exchange, routingKey string, data amqp.Publishing) error {
	if !b.isReady {
		return errNotConnected
	}
	return b.channel.Publish(
		exchange,   // Exchange
		routingKey, // Routing key
		false,      // Mandatory
		false,      // Immediate
		data,
	)
}

func (b *BusExt) Register(routingKey string, handler Handler) {
	b.consumers[routingKey] = handler
}

func (b *BusExt) Consume() error {
	if !b.isReady {
		log.Error("can not consume. BusExt is not ready")
		return errNotConnected
	}
	if err := b.channel.Qos(b.prefetch, 0, false); err != nil {
		log.Warnf("set qos failed: %v", err)
	}
	hostName, err := os.Hostname()
	if err != nil {
		log.Warnf("get host name failed: %v", err)
	}
	for _, queue := range b.config.GetStringSlice("queues") {
		ch, err := b.channel.Consume(
			queue,
			hostName,
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			log.Errorf("Consume queue: %v failed: %v", queue, err)
			return err
		}
		b.consumeChannels[queue] = ch
	}
	wg := sync.WaitGroup{}
	for name, ch := range b.consumeChannels {
		chName := name
		channel := ch
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-b.done:
					return
				case delivery := <-channel:
					deliveryAck(delivery)
					log.Debugf("Receive delivery: %+v from queue: %v",
						delivery, chName)
					var handler Handler
					var ok bool
					if delivery.Headers == nil {
						log.Error("Not support v1 celery protocol yet")
					} else if delivery.ContentType != "application/json" {
						log.Error("Only json encoding is allowed")
					} else if delivery.ContentEncoding != "utf-8" {
						log.Error("Unsupported content encoding")
					} else if handler, ok = b.consumers[delivery.RoutingKey]; !ok {
						log.Error("Receive unregistered message")
					} else {
						var payload []json.RawMessage
						if err := json.Unmarshal(delivery.Body, &payload); err != nil {
							log.Errorf("json decode error: %v", err)
						} else if err := handler.ParsePayload(payload[0],
							payload[1]); err != nil {
							log.Errorf("handler parse payload error: %v", err)
						} else if err := handler.Run(); err != nil {
							log.Errorf("handler run task failed: %v", err)
						}
					}
				}
			}
		}()
	}
	wg.Wait()
	return nil
}

func (b *BusExt) handleReconnect(brokerUrl string) {
	for {
		b.isReady = false
		log.Infof("Attempting to connect to %v", brokerUrl)

		conn, err := b.connect(brokerUrl)

		if err != nil {
			log.Errorf("Failed to connect: %v. Retrying...", err)
			select {
			case <-b.done:
				return
			case <-time.After(b.reconnectDelay):
			}
			continue
		}

		if done := b.handleReInit(conn); done {
			break
		}
	}
}

func (b *BusExt) connect(brokerUrl string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(brokerUrl)

	if err != nil {
		return nil, err
	}

	b.changeConnection(conn)
	log.Info("Connected!")
	return conn, nil
}

func (b *BusExt) handleReInit(conn *amqp.Connection) bool {
	for {
		b.isReady = false

		err := b.init(conn)

		if err != nil {
			log.Errorf("Failed to initialize channel: %v. Retrying...", err)

			select {
			case <-b.done:
				return true
			case <-time.After(b.reinitDelay):
			}
			continue
		}

		select {
		case <-b.done:
			return true
		case <-b.notifyConnClose:
			log.Info("Connection closed. Reconnecting...")
			return false
		case <-b.notifyChanClose:
			log.Info("channel closed. Rerunning init...")
		}
	}
}

func (b *BusExt) init(conn *amqp.Connection) error {
	ch, err := conn.Channel()

	if err != nil {
		log.Errorf("create channel failed: %v", err)
		return err
	}

	err = ch.Confirm(false)

	if err != nil {
		log.Errorf("change to confirm mod failed: %v", err)
		return err
	}

	for _, exchange := range b.config.GetStringSlice("exhanges") {
		err = ch.ExchangeDeclare(
			exchange,
			amqp.ExchangeTopic,
			true,
			false,
			false,
			false,
			nil)

		if err != nil {
			log.Errorf("declare exchange: %v failed: %v", exchange, err)
			return err
		}
		log.Infof("declare exchange: %v succeeded", exchange)
	}

	for _, queue := range b.config.GetStringSlice("queues") {
		_, err = ch.QueueDeclare(
			queue,
			true,  // Durable
			false, // Delete when unused
			false, // Exclusive
			false, // No-wait
			nil,   // Arguments
		)

		if err != nil {
			log.Errorf("declare queue: %v failed: %v", queue, err)
			return err
		}
		log.Infof("declare queue: %v succeeded", queue)
	}

	var bs []map[string]string
	if err := b.config.UnmarshalKey("bindings", &bs); err != nil {
		log.Errorf("unmarshal bindings failed: %v", err)
		return err
	}
	for _, binding := range bs {
		if err := ch.QueueBind(
			binding["queue"],
			binding["binding_key"],
			binding["exchange"],
			false,
			nil); err != nil {
			log.Errorf("declare binding: %v failed: %v", binding, err)
			return err
		}
		log.Infof("declare binding: %v succeeded", binding)
	}

	b.changeChannel(ch)
	b.isReady = true
	if len(b.consumers) > 0 {
		b.consumeChannels = make(map[string]<-chan amqp.Delivery)
		go func() {
			err := b.Consume()
			if err != nil {
				log.Errorf("errors occur when consume: %v", err)
			}
		}()
	}
	log.Info("init finished")

	return nil
}

func (b *BusExt) changeConnection(connection *amqp.Connection) {
	b.connection = connection
	b.notifyConnClose = make(chan *amqp.Error)
	b.connection.NotifyClose(b.notifyConnClose)
	log.Info("connection changed")

}

func (b *BusExt) changeChannel(channel *amqp.Channel) {
	b.channel = channel
	b.notifyChanClose = make(chan *amqp.Error)
	b.notifyConfirm = make(chan amqp.Confirmation, 1)
	b.channel.NotifyClose(b.notifyChanClose)
	b.channel.NotifyPublish(b.notifyConfirm)
	log.Info("channel changed")
}

func deliveryAck(delivery amqp.Delivery) {
	var err error
	for retryCount := 3; retryCount > 0; retryCount-- {
		if err = delivery.Ack(false); err == nil {
			break
		}
	}
	if err != nil {
		log.Errorf("failed to ack delivery: %+v"+
			": %+v",
			delivery.MessageId, err)
	}
}

func setDefaultConfig(v *viper.Viper) {
	v.SetDefault("prefetch", defaultPrefetch)
	v.SetDefault("publish_retry", defaultPublishRetry)
	v.SetDefault("resend_delay", defaultResendDelay)
	v.SetDefault("reconnect_delay", defaultReconnectDelay)
	v.SetDefault("reinit_delay", defaultReinitDelay)
}
