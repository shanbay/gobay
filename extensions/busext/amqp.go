package busext

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/streadway/amqp"

	"github.com/shanbay/gobay"
)

var (
	errNotConnected  = errors.New("not connected to a server")
	errAlreadyClosed = errors.New("already closed: not connected to the server")
	errShutdown      = errors.New("BusExt is closed")
	ErrTimeout       = errors.New("Timeout when push bus messages")
	ErrNotReady      = errors.New("BusExt is not ready")
)

const (
	defaultResendDelay    = "1s"
	defaultReconnectDelay = "2s"
	defaultReinitDelay    = "1s"
	defaultPrefetch       = 100
	defaultPublishRetry   = 3
	defaultPushTimeout    = "5s"
)

type customLoggerInterface interface {
	Printf(string, ...interface{})
	Println(...interface{})
}

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
	pushM           sync.Mutex
	ErrorLogger     customLoggerInterface
	mocked          bool
	pushTimeout     time.Duration
	// this two funcs can be mocked for test
	pushFunc        func(context.Context, string, string, amqp.Publishing, chan error)
	publishFunc     func(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	brokerUrl       string
	notifyChanBlock chan error
}

func (b *BusExt) Object() interface{} {
	return b.channel
}

func (b *BusExt) Application() *gobay.Application {
	return b.app
}

func (b *BusExt) Init(app *gobay.Application) error {
	if b.NS == "" {
		return errors.New("lack of NS")
	}
	b.app = app
	config := app.Config()
	b.config = gobay.GetConfigByPrefix(config, b.NS, true)
	setDefaultConfig(b.config)
	b.consumers = make(map[string]Handler)
	b.consumeChannels = make(map[string]<-chan amqp.Delivery)
	b.prefetch = b.config.GetInt("prefetch")
	b.publishRetry = b.config.GetInt("publish_retry")
	b.resendDelay = b.config.GetDuration("resend_delay")
	b.reconnectDelay = b.config.GetDuration("reconnect_delay")
	b.reinitDelay = b.config.GetDuration("reinit_delay")
	b.brokerUrl = b.config.GetString("broker_url")
	b.done = make(chan bool)
	b.pushTimeout = b.config.GetDuration("push_timeout")
	b.pushFunc = b.doPush
	b.notifyChanBlock = make(chan error)

	var tlsConfig *tls.Config
	if b.config.GetBool("tls") {
		if err := b.config.UnmarshalKey("TLSConfig", &tlsConfig); err != nil {
			b.ErrorLogger.Printf("unmarshal TLSConfig failed: %v\n", err)
			return err
		}
	}

	b.mocked = b.config.GetBool("mocked")
	if !b.mocked {
		go b.handleReconnect(b.brokerUrl, tlsConfig)
	} else {
		b.isReady = true
	}

	for {
		if !b.isReady {
			continue
		} else {
			break
		}
	}
	log.Println("BusExt init done")
	return nil
}

func (b *BusExt) Close() error {
	if b.mocked {
		return nil
	}
	if !b.isReady {
		return errAlreadyClosed
	}
	if err := b.channel.Close(); err != nil {
		b.ErrorLogger.Printf("close channel failed: %v\n", err)
		return err
	}
	if err := b.connection.Close(); err != nil {
		b.ErrorLogger.Printf("close connection failed: %v\n", err)
		return err
	}

	close(b.done)
	b.isReady = false

	log.Println("BusExt closed")
	return nil
}

func (b *BusExt) Push(exchange, routingKey string, data amqp.Publishing) error {
	if b.mocked {
		return nil
	}
	b.pushM.Lock()
	defer b.pushM.Unlock()
	if !b.isReady {
		b.ErrorLogger.Printf("Can not publish message: %v\n", ErrNotReady)
		return ErrNotReady
	}
	result := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), b.pushTimeout)
	defer cancel()
	// NOTE: If unsafePush blocked, goroutine may leak here
	go b.pushFunc(ctx, exchange, routingKey, data, result)
	// NOTE END
	select {
	case err := <-result:
		if err != nil {
			b.ErrorLogger.Printf("Push failed: %v\n", err)
			return err
		}
		return nil
	case <-ctx.Done():
		b.isReady = false
		b.notifyChanBlock <- ErrTimeout
		return ErrTimeout
	}
}

func (b *BusExt) doPush(ctx context.Context, exchange, routingKey string, data amqp.Publishing, result chan error) {
	log.Printf("Trying to publish: %s\n", data.Headers["id"])
	for i := 0; i <= b.publishRetry; i++ {
		// clear staled confirmnation
		// 有可能在超时之后才收到 confirm，会堵塞 channel，最终造成死锁
		select {
		case <-b.notifyConfirm:
		default:
		}
		err := b.unsafePush(exchange, routingKey, data)
		if err != nil {
			b.ErrorLogger.Printf("UnsafePush msg %s failed : %v\n", data.Headers["id"], err)
			select {
			case <-b.done:
				b.ErrorLogger.Println("BusExt closed during publishing message")
				result <- errShutdown
				return
			case <-time.After(b.resendDelay):
			}
			continue
		}
		select {
		case confirm := <-b.notifyConfirm:
			if confirm.Ack {
				log.Printf("Publish %s confirmed!", data.Headers["id"])
				result <- nil
				return
			}
		case <-time.After(b.resendDelay):
		}
		log.Printf("Publish %s not confirmed after %f seconds. Retrying...\n",
			data.Headers["id"], b.resendDelay.Seconds())
	}
	err := fmt.Errorf(
		"publishing message %s failed after retry %d times", data.Headers["id"], b.publishRetry)
	b.ErrorLogger.Println(err)
	result <- err
}

func (b *BusExt) unsafePush(exchange, routingKey string, data amqp.Publishing) error {
	if !b.isReady {
		return errNotConnected
	}
	return b.publishFunc(
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
		b.ErrorLogger.Println("can not consume. BusExt is not ready")
		return errNotConnected
	}
	if err := b.channel.Qos(b.prefetch, 0, false); err != nil {
		b.ErrorLogger.Printf("set qos failed: %v\n", err)
	}
	hostName, err := os.Hostname()
	if err != nil {
		b.ErrorLogger.Printf("get host name failed: %v\n", err)
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
			b.ErrorLogger.Printf("StartWorker queue: %v failed: %v\n", queue, err)
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
					b.deliveryAck(delivery)
					log.Printf("Receive delivery: %s from queue: %v\n",
						delivery.Headers["id"], chName)
					var handler Handler
					var ok bool
					if delivery.Headers == nil {
						b.ErrorLogger.Println("Not support v1 celery protocol yet")
					} else if delivery.ContentType != "application/json" {
						b.ErrorLogger.Println("Only json encoding is allowed")
					} else if delivery.ContentEncoding != "utf-8" {
						b.ErrorLogger.Println("Unsupported content encoding")
					} else if handler, ok = b.consumers[delivery.RoutingKey]; !ok {
						b.ErrorLogger.Println("Receive unregistered message")
					} else {
						var payload []json.RawMessage
						if err := json.Unmarshal(delivery.Body, &payload); err != nil {
							b.ErrorLogger.Printf("json decode error: %v\n", err)
						} else if err := handler.ParsePayload(payload[0],
							payload[1]); err != nil {
							b.ErrorLogger.Printf("handler parse payload error: %v\n", err)
						} else if err := handler.Run(); err != nil {
							b.ErrorLogger.Printf("handler run task failed: %v\n", err)
						}
					}
				}
			}
		}()
	}
	wg.Wait()
	return nil
}

func (b *BusExt) handleReconnect(brokerUrl string, tlsConfig *tls.Config) {
	for {
		b.isReady = false
		log.Printf("Attempting to connect to %v tlsConfig: %v\n", brokerUrl, tlsConfig)

		conn, err := b.connect(brokerUrl, tlsConfig)

		if err != nil {
			b.ErrorLogger.Printf("Failed to connect: %v. Retrying...\n", err)
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

func (b *BusExt) connect(brokerUrl string, tlsConfig *tls.Config) (*amqp.Connection, error) {
	var conn *amqp.Connection
	var err error
	if tlsConfig != nil {
		conn, err = amqp.DialTLS(brokerUrl, tlsConfig)
	} else {
		conn, err = amqp.Dial(brokerUrl)
	}

	if err != nil {
		return nil, err
	}

	b.changeConnection(conn)
	log.Println("Connected!")
	return conn, nil
}

func (b *BusExt) handleReInit(conn *amqp.Connection) bool {
	for {
		b.isReady = false

		err := b.init(conn)

		if err != nil {
			b.ErrorLogger.Printf("Failed to initialize channel: %v. Retrying...\n", err)

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
			log.Println("Connection closed. Reconnecting...")
			return false
		case <-b.notifyChanClose:
			log.Println("channel closed. Retrying init...")
		case <-b.notifyChanBlock:
			log.Println("Channel blocked. Retrying init...")
		}
	}
}

func (b *BusExt) init(conn *amqp.Connection) error {
	ch, err := conn.Channel()

	if err != nil {
		b.ErrorLogger.Printf("create channel failed: %v\n", err)
		return err
	}

	err = ch.Confirm(false)

	if err != nil {
		b.ErrorLogger.Printf("change to confirm mod failed: %v\n", err)
		return err
	}

	for _, exchange := range b.config.GetStringSlice("exchanges") {
		err = ch.ExchangeDeclare(
			exchange,
			amqp.ExchangeTopic,
			true,
			false,
			false,
			false,
			nil)

		if err != nil {
			b.ErrorLogger.Printf("declare exchange: %v failed: %v\n", exchange, err)
			return err
		}
		log.Printf("declare exchange: %v succeeded\n", exchange)
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
			b.ErrorLogger.Printf("declare queue: %v failed: %v\n", queue, err)
			return err
		}
		log.Printf("declare queue: %v succeeded\n", queue)
	}

	var bs []map[string]string
	if err := b.config.UnmarshalKey("bindings", &bs); err != nil {
		b.ErrorLogger.Printf("unmarshal bindings failed: %v\n", err)
		return err
	}
	for _, binding := range bs {
		if err := ch.QueueBind(
			binding["queue"],
			binding["binding_key"],
			binding["exchange"],
			false,
			nil); err != nil {
			b.ErrorLogger.Printf("declare binding: %v failed: %v\n", binding, err)
			return err
		}
		log.Printf("declare binding: %v succeeded\n", binding)
	}

	b.changeChannel(ch)
	b.isReady = true
	if len(b.consumers) > 0 {
		b.consumeChannels = make(map[string]<-chan amqp.Delivery)
		go func() {
			err := b.Consume()
			if err != nil {
				b.ErrorLogger.Printf("errors occur when consume: %v\n", err)
			}
		}()
	}
	b.publishFunc = b.channel.Publish
	log.Println("init finished")

	return nil
}

func (b *BusExt) changeConnection(connection *amqp.Connection) {
	b.connection = connection
	b.notifyConnClose = make(chan *amqp.Error)
	b.connection.NotifyClose(b.notifyConnClose)
	log.Println("connection changed")

}

func (b *BusExt) changeChannel(channel *amqp.Channel) {
	b.channel = channel
	b.notifyChanClose = make(chan *amqp.Error)
	b.notifyConfirm = make(chan amqp.Confirmation, 5)
	b.channel.NotifyClose(b.notifyChanClose)
	b.channel.NotifyPublish(b.notifyConfirm)
	log.Println("channel changed")
}

func (b *BusExt) deliveryAck(delivery amqp.Delivery) {
	var err error
	for retryCount := 3; retryCount > 0; retryCount-- {
		if err = delivery.Ack(false); err == nil {
			break
		}
	}
	if err != nil {
		b.ErrorLogger.Printf("failed to ack delivery: %+v"+
			": %+v\n",
			delivery.MessageId, err)
	}
}

func setDefaultConfig(v *viper.Viper) {
	v.SetDefault("prefetch", defaultPrefetch)
	v.SetDefault("publish_retry", defaultPublishRetry)
	v.SetDefault("resend_delay", defaultResendDelay)
	v.SetDefault("reconnect_delay", defaultReconnectDelay)
	v.SetDefault("reinit_delay", defaultReinitDelay)
	v.SetDefault("push_timeout", defaultPushTimeout)
}
