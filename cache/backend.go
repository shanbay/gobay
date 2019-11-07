package cache

import (
	"github.com/go-redis/redis"
)

type MemExt struct {
	app    *gobay.Application
	prefix string
}

func (m *MemExt) Init(app *gobay.Application) {
	m.prefix = config.GetString("cache_prefix")
}
