package databeat

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type indexByTime string

const (
	Daily   indexByTime = "daily"
	Weekly  indexByTime = "weekly"
	Monthly indexByTime = "monthly"
	Yearly  indexByTime = "yearly"
)
const (
	PREFIX      = "[DATABEAT] "
	MAXFIELDLEN = 10240
)

var indexByChoices = []indexByTime{
	Daily,
	Weekly,
	Monthly,
	Yearly,
}
var reservedWords = []string{
	"beat",
	"input_type",
	"offset",
	"type",
	"id",
	"source",
	"_ts",
}

// Model struct
type DataModel struct {
	name        string
	fields      []string
	group       string
	index_by    indexByTime
	aggs_fields []string
}

var beatLogger *log.Logger

// Singleton beat logger
func GetBeatLogger() *log.Logger {
	if beatLogger == nil {
		beatLogger = log.New(os.Stdout, PREFIX, 0)
	}
	return beatLogger
}

// Log to stdout
func (d *DataModel) Beat(content map[string]interface{}) (string, error) {
	err := d.validate(content)
	if err != nil {
		return "", err
	}
	d.updateContent(content)
	dataBytes, err := json.Marshal(content)
	if err != nil {
		return "", err
	}
	dataStr := string(dataBytes)

	GetBeatLogger().Println(dataStr)
	return dataStr, nil
}

// Validate DataModel and content
func (d *DataModel) validate(content map[string]interface{}) error {
	// validate DataModel, fatal exit
	if d.name == "" {
		log.Fatal("缺少name")
	}
	if d.fields == nil {
		log.Fatal("缺少fields")
	}
	if d.group == "" {
		log.Fatal("缺少group")
	}
	for _, field := range d.fields {
		if field[0] == '@' {
			log.Fatal("不能以@开始")
		}
		for _, reservedWord := range reservedWords {
			if reservedWord == field {
				log.Fatal("包含保留词", reservedWords)
			}
		}
	}
	var flag bool
	for _, aggs_field := range d.aggs_fields {
		flag = false
		for _, field := range d.fields {
			if aggs_field == field {
				flag = true
				break
			}
		}
		if !flag {
			log.Fatal("aggs_field不存在", aggs_field)
		}
	}
	flag = false
	for _, choice := range indexByChoices {
		if d.index_by == choice {
			flag = true
			break
		}
	}
	if !flag {
		log.Fatal("index_by不合法")
	}

	// validate content, return err
	for _, field := range d.fields {
		if _, ok := content[field]; !ok {
			return errors.New("缺少字段: " + field)
		}
	}
	if len(content) != len(d.fields) {
		return errors.New("传入的内容字段不合法")
	}

	return nil
}

// Update map for json.Marshal
func (d *DataModel) updateContent(content map[string]interface{}) {
	for k, v := range content {
		content[k] = cleanData(v)
	}

	// aggs_field
	var agg []string
	for _, field := range d.aggs_fields {
		if s, ok := content[field].(string); ok {
			agg = append(agg, s)
			continue
		}
		if i, ok := content[field].(int); ok {
			agg = append(agg, strconv.Itoa(i))
			continue
		}
	}
	aggStr := strings.Join(agg, "||")
	content["aggs_field"] = aggStr

	// add index_by, @name, @group, _ts, @id
	indexByKey := fmt.Sprintf("@%s", d.index_by)
	content[indexByKey] = true
	content["@name"] = d.name
	content["@group"] = d.group
	content["@id"] = uuid.New().String()
	content["_ts"] = float64(time.Now().UnixNano()) / float64(time.Second)
}

// Clean source content
func cleanData(v interface{}) interface{} {
	if s, ok := v.(string); ok && len(s) > MAXFIELDLEN {
		return s[0:MAXFIELDLEN]
	} else if t, ok := v.(time.Time); ok {
		return t.Format(time.RFC3339)
	}
	return v
}
