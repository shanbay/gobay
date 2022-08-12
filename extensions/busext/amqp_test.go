package busext

import (
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/extensions/sentryext/custom_logger"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

var (
	app    *gobay.Application
	bus    BusExt
	result []*TestHandler
)

func init() {
	bus = BusExt{NS: "bus_"}

	bus.ErrorLogger = custom_logger.NewSentryErrorLogger()
	app, _ = gobay.CreateApp(
		"../../testdata",
		"testing",
		map[gobay.Key]gobay.Extension{
			"bus": &bus,
		},
	)

	if err := app.Init(); err != nil {
		log.Println(err)
	}
}

func TestPushConsume(t *testing.T) {
	// publish
	routingKey := "gobay.buses.test"
	for i := 0; i < 100; i++ {
		msg, _ := BuildMsg(
			routingKey,
			[]interface{}{},
			map[string]interface{}{
				"user_id": i,
				"items": []map[string]interface{}{
					{
						"created_at": time.Now(),
						"updated_at": time.Now(),
					},
				},
			},
		)
		if err := bus.Push(
			"sbay-exchange",
			routingKey,
			*msg,
		); err != nil {
			log.Println(err)
		}
	}

	// consume
	bus.Register(routingKey, &TestHandler{})
	go func() {
		err := bus.Consume()
		if err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(2 * time.Second)
	assert.Len(t, result, 100)

	// check config works
	assert.NotEqual(t, defaultPublishRetry, bus.publishRetry)
	assert.Equal(t, 5, bus.publishRetry)
	assert.NotEqual(t, defaultPushTimeout, bus.pushTimeout)
	assert.Equal(t, time.Second * 3, bus.pushTimeout)

	// mock amqp 的 publish, 使其 sleep 一个远比设置的 pushTimeout 长的时间, 模拟其卡死的情况
	bus.publishFunc = func(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
		dur := 100 * bus.pushTimeout
		time.Sleep(dur)
		return nil
	}

	msg, _ := BuildMsg(
		routingKey,
		[]interface{}{},
		map[string]interface{}{
			"user_id": 1,
			"items": []map[string]interface{}{
				{
					"created_at": time.Now(),
					"updated_at": time.Now(),
				},
			},
		},
	)
	// case-1: 超时后会结束本次 push 并返回 errTimeout error, 并且尝试重连
	err := bus.Push("sbay-exchange", routingKey, *msg)
	assert.NotNil(t, err)
	assert.Equal(t, ErrTimeout, err)

	// case-2: 紧接着 push 一次, 会返回 errNotReady, 因为此时还没有重新连接好
	err = bus.Push("sbay-exchange", routingKey, *msg)
	assert.NotNil(t, err)
	assert.Equal(t, ErrNotReady, err)

	// case-3: 等带几秒, 重连后再次 push, 可以成功
	time.Sleep(3 * time.Second)
	err = bus.Push("sbay-exchange", routingKey, *msg)
	assert.Nil(t, err)
	assert.Len(t, result, 101)

	assert.Nil(t, app.Close())
}

type TestHandler struct {
	UserID int64 `json:"user_id"`
	Items  []Item
}

type Item struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (o *TestHandler) ParsePayload(args []byte, kwargs []byte) (err error) {
	if err := json.Unmarshal(kwargs, o); err != nil {
		return err
	}
	return nil
}

func (o *TestHandler) Run() error {
	result = append(result, o)
	return nil
}
