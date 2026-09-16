package observability

// MapCarrier 把 map[string]interface{} 形态的消息头（machinery 的 tasks.Headers、
// amqp.Table）适配成 OpenTelemetry 的 TextMapCarrier，供 traceparent 注入/提取。
// 非 string 值在 Get 时视为不存在。
type MapCarrier map[string]interface{}

func (c MapCarrier) Get(key string) string {
	if v, ok := c[key].(string); ok {
		return v
	}
	return ""
}

func (c MapCarrier) Set(key, value string) {
	c[key] = value
}

func (c MapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
