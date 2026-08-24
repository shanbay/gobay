/*
# Partition table (ECP + BVA) — step 01-asynctask-bus-metrics (busext side)
#
# 参数/状态           | 等价类                    | 类型              | 代表值/场景                                              | 期望输出                                                                  | 对应契约条目
# monitorEnabled      | 关闭（默认/未设置）       | 有效              | busmonoff_ 在 "testing" env 下未设置 monitor_enable      | Consume() 正常处理消息不受影响，不 Observe                                | 数据层 API 锚点 4
# monitorEnabled      | 开启                      | 有效              | busmon_monitor_enable: true                              | monitorEnabled == true                                                    | 数据层 API 锚点 5/6/7/8
# dispatch status     | success                   | 有效（正常路径）  | 已注册 handler，ParsePayload/Run 都成功                  | status="success" 计数增加                                                 | 数据层 API 锚点 5
# dispatch status     | failure                   | 有效（错误路径）  | handler.Run() 返回 error                                 | status="failure" 计数增加                                                 | 数据层 API 锚点 6
# dispatch status     | invalid_message           | 无效（错误路径）  | 消息路由到已绑定队列，但 routing key 未注册消费者        | status="invalid_message" 计数增加                                         | 数据层 API 锚点 7
# dispatch status     | parse_error               | 无效（错误路径）  | handler.ParsePayload() 返回 error                        | status="parse_error" 计数增加                                             | 数据层 API 锚点 8
#
# 说明：busext 侧的独立参数只有 2 个（monitorEnabled ×2、dispatch status ×4），
# 未达到 pairwise 触发阈值（≥3 参数 × 每个 ≥2 取值），因此未调用 pairwise 脚本，
# 改为对 dispatch status 的 4 个等价类逐一穷举（success/failure/invalid_message/
# parse_error 各一个断言），并单独覆盖 monitorEnabled=false 的等价类代表。
*/
package busext

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	// health check
	err := bus.HealthCheck()
	assert.Nil(t, err)

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
	err = bus.Push("sbay-exchange", routingKey, *msg)
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

// monitorSuccessHandler / monitorFailureHandler / monitorParseErrorHandler are
// used exclusively by the busext monitor tests below, each representing one
// equivalence class of the dispatch() status partition (success / failure /
// parse_error). The "invalid_message" class doesn't need a handler at all —
// it's triggered by deliberately leaving a bound routing key unregistered.
type monitorSuccessHandler struct{}

func (h *monitorSuccessHandler) ParsePayload(args []byte, kwargs []byte) error { return nil }
func (h *monitorSuccessHandler) Run() error                                    { return nil }

type monitorFailureHandler struct{}

func (h *monitorFailureHandler) ParsePayload(args []byte, kwargs []byte) error { return nil }
func (h *monitorFailureHandler) Run() error {
	return errors.New("intentional failure for busext monitor test")
}

type monitorParseErrorHandler struct{}

func (h *monitorParseErrorHandler) ParsePayload(args []byte, kwargs []byte) error {
	return errors.New("intentional parse error for busext monitor test")
}
func (h *monitorParseErrorHandler) Run() error { return nil }

var busMetricsOnce sync.Once

const busMetricsAddr = "127.0.0.1:2114"

// startBusMetricsServer exposes prometheus.DefaultRegisterer via
// promhttp.Handler(), mirroring extensions/cachext's TestCacheExt_Cached_Monitor
// pattern (config-gated instrumentation + no self-built /metrics server in
// production code, only in the test harness).
func startBusMetricsServer() {
	busMetricsOnce.Do(func() {
		go func() {
			http.Handle("/metrics", promhttp.Handler())
			if err := http.ListenAndServe(busMetricsAddr, nil); err != nil {
				log.Fatalf("error when start prometheus server: %v\n", err)
			}
		}()
		time.Sleep(200 * time.Millisecond)
	})
}

func fetchBusMetrics(t *testing.T) string {
	resp, err := http.Get("http://" + busMetricsAddr + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestBusExt_Monitor_Disabled 覆盖锚点 4：monitor_enable 默认关闭时，
// monitorEnabled 保持 false，Consume() 正常处理消息不受影响，且不产生任何
// bus_task_duration_seconds 记录。
func TestBusExt_Monitor_Disabled(t *testing.T) {
	startBusMetricsServer()

	busMonOff := BusExt{NS: "busmonoff_"}
	busMonOff.ErrorLogger = custom_logger.NewSentryErrorLogger()
	offApp, err := gobay.CreateApp(
		"../../testdata",
		"testing",
		map[gobay.Key]gobay.Extension{"busmonoff": &busMonOff},
	)
	assert.Nil(t, err)
	assert.Nil(t, offApp.Init())
	assert.False(t, busMonOff.monitorEnabled)

	routingKey := "gobay.buses.busmonoff"
	busMonOff.Register(routingKey, &monitorSuccessHandler{})
	go func() {
		if err := busMonOff.Consume(); err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(500 * time.Millisecond)

	msg, err := BuildMsg(routingKey, []interface{}{}, map[string]interface{}{"n": 1})
	assert.Nil(t, err)
	assert.Nil(t, busMonOff.Push("sbay-exchange", routingKey, *msg))
	time.Sleep(1 * time.Second)

	data := fetchBusMetrics(t)
	assert.NotContains(t, data,
		`bus_task_duration_seconds_count{queue="`+routingKey+`",status="success",task_name="`+routingKey+`"}`)
}

// TestBusExt_Monitor covers 锚点 5/6/7/8：monitor_enable=true 下，
// dispatch() 的四种状态（success/failure/invalid_message/parse_error）
// 都会被各自记录到 bus_task_duration_seconds。
func TestBusExt_Monitor(t *testing.T) {
	startBusMetricsServer()

	busMon := BusExt{NS: "busmon_"}
	busMon.ErrorLogger = custom_logger.NewSentryErrorLogger()
	monApp, err := gobay.CreateApp(
		"../../testdata",
		"busmonitored",
		map[gobay.Key]gobay.Extension{"busmon": &busMon},
	)
	assert.Nil(t, err)
	assert.Nil(t, monApp.Init())
	assert.True(t, busMon.monitorEnabled)

	busMon.Register("gobay.buses.busmon", &monitorSuccessHandler{})
	busMon.Register("gobay.buses.busmon.failure", &monitorFailureHandler{})
	busMon.Register("gobay.buses.busmon.parseerror", &monitorParseErrorHandler{})
	// gobay.buses.busmon.unregistered 故意不注册消费者，用于验证 invalid_message 分支

	go func() {
		if err := busMon.Consume(); err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(500 * time.Millisecond)

	push := func(t *testing.T, routingKey string) {
		msg, err := BuildMsg(routingKey, []interface{}{}, map[string]interface{}{"n": 1})
		assert.Nil(t, err)
		assert.Nil(t, busMon.Push("sbay-exchange", routingKey, *msg))
	}

	t.Run("5: 成功处理的消息 -> status=success 计数增加", func(t *testing.T) {
		routingKey := "gobay.buses.busmon"
		push(t, routingKey)
		time.Sleep(1 * time.Second)

		data := fetchBusMetrics(t)
		labels := `queue="` + routingKey + `",status="success",task_name="` + routingKey + `"`
		assert.Contains(t, data, `bus_task_duration_seconds_count{`+labels+`} 1`)
	})

	t.Run("6: handler.Run() 返回 error -> status=failure", func(t *testing.T) {
		routingKey := "gobay.buses.busmon.failure"
		push(t, routingKey)
		time.Sleep(1 * time.Second)

		data := fetchBusMetrics(t)
		labels := `queue="` + routingKey + `",status="failure",task_name="` + routingKey + `"`
		assert.Contains(t, data, `bus_task_duration_seconds_count{`+labels+`} 1`)
	})

	t.Run("7: 未注册 routing key 的消息 -> status=invalid_message", func(t *testing.T) {
		routingKey := "gobay.buses.busmon.unregistered"
		push(t, routingKey)
		time.Sleep(1 * time.Second)

		data := fetchBusMetrics(t)
		labels := `queue="` + routingKey + `",status="invalid_message",task_name="` + routingKey + `"`
		assert.Contains(t, data, `bus_task_duration_seconds_count{`+labels+`} 1`)
	})

	t.Run("8: payload 无法解析的消息 -> status=parse_error", func(t *testing.T) {
		routingKey := "gobay.buses.busmon.parseerror"
		push(t, routingKey)
		time.Sleep(1 * time.Second)

		data := fetchBusMetrics(t)
		labels := `queue="` + routingKey + `",status="parse_error",task_name="` + routingKey + `"`
		assert.Contains(t, data, `bus_task_duration_seconds_count{`+labels+`} 1`)
	})
}
