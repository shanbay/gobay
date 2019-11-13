package busext

import (
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/shanbay/gobay"
)

var (
	app    *gobay.Application
	bus    BusExt
	result []*OCPaid
)

func init() {
	bus = BusExt{NS: "bus"}

	app, _ = gobay.CreateApp(
		"../testdata",
		"testing",
		map[gobay.Key]gobay.Extension{
			"bus": &bus,
		},
	)

	if err := app.Init(); err != nil {
		log.Println(err)
	}
}

func TestPushPushConsume(t *testing.T) {
	// publish
	routingKey := "buses.oc.post_order_paid"
	for i := 0; i < 100; i++ {
		msg, _ := BuildMsg(
			routingKey,
			[]interface{}{},
			map[string]interface{}{
				"user_id":       i,
				"order_id":      i,
				"department_id": i,
				"items": []map[string]interface{}{
					{
						"created_at":    time.Now(),
						"updated_at":    time.Now(),
						"user_id":       i,
						"item_quantity": i,
						"item_price":    i,
						"service_id":    i,
						"order_id":      i,
						"item_id":       i,
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

	//consume
	bus.Register("buses.oc.post_order_paid", &OCPaid{})
	go bus.Consume()
	time.Sleep(2 * time.Second)
	if len(result) != 100 {
		t.Error("consume length doesn't match publish'")
	}
}

type OCPaid struct {
	UserID       int64 `json:"user_id"`
	OrderID      int64 `json:"order_id"`
	DepartmentID int   `json:"department_id"`
	Items        []Item
}

type Item struct {
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	UserID       int64     `json:"user_id"`
	ItemQuantity int       `json:"item_quantity"`
	ItemPrice    int       `json:"item_price"`
	ServiceID    int       `json:"service_id"`
	OrderID      int64     `json:"order_id"`
	ItemID       int       `json:"item_id"`
}

func (o *OCPaid) ParsePayload(args []byte, kwargs []byte) (err error) {
	if err := json.Unmarshal(kwargs, o); err != nil {
		return err
	}
	return nil
}

func (o *OCPaid) Run() error {
	result = append(result, o)
	return nil
}
