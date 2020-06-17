package busext

import (
	"encoding/json"

	uuid "github.com/satori/go.uuid"
	"github.com/streadway/amqp"
)

type Handler interface {
	ParsePayload(args []byte, kwargs []byte) error

	Run() error
}

type Body struct {
	Args   []interface{}          `json:"args"`
	Kwargs map[string]interface{} `json:"kwargs"`
	Embed  Embed
}

type Embed struct {
	Callbacks []string `json:"callbacks"`
	Errbacks  []string `json:"errbacks"`
	Chain     []string `json:"chain"`
	Chord     string   `json:"chord"`
}

func BuildMsg(routingKey string, args []interface{}, kwargs map[string]interface{}) (*amqp.Publishing, error) {
	taskID := uuid.NewV4().String()
	headers := amqp.Table{
		"lang":       "go",
		"task":       routingKey,
		"id":         taskID,
		"root_id":    taskID,
		"parent_id":  taskID,
		"group":      nil,
		"eta":        nil,
		"expires":    nil,
		"retries":    0,
		"timelimit":  []interface{}{nil, nil},
		"argsrepr":   nil,
		"kwargsrepr": nil,
	}
	body, err := json.Marshal([]interface{}{args, kwargs, Embed{}})
	if err != nil {
		return nil, err
	}
	return &amqp.Publishing{
		ContentType:     "application/json",
		ContentEncoding: "utf-8",
		Body:            body,
		DeliveryMode:    amqp.Persistent,
		Headers:         headers,
	}, nil
}
