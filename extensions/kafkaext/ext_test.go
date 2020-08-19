package kafkaext

import (
	"github.com/segmentio/kafka-go"
	"github.com/shanbay/gobay"
	"log"
	"testing"
)

var (
	app  *gobay.Application
	task KafkaExt
)

func init() {
	app, err := gobay.CreateApp(
		"../../testdata",
		"testing",
		map[gobay.Key]gobay.Extension{
			"kafka": &task,
		},
	)
	if err != nil {
		log.Panic(err)
	}
	if err = app.Init(); err != nil {
		log.Panic(err)
	}
}

func TestWRMsgs(t *testing.T) {
	rawMsg := kafka.Message{Key: []byte("key1"),Value: []byte("one!")}

	task.WriteMessages(rawMsg)
	msg, _ := task.ReadMessage(1e6)

	if string(rawMsg.Key) != string(msg.Key) {
		t.Errorf("rawmsg key:%v received key:%v", rawMsg.Key, msg.Key)
	}
	if string(rawMsg.Value) != string(msg.Value) {
		t.Errorf("rawmsg value:%v received value:%v", rawMsg.Value, msg.Value)
	}
}
