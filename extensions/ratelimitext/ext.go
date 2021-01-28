package ratelimitext

import (
	"context"
	"errors"
	"sync"

	"github.com/shanbay/gobay"
	"github.com/spf13/viper"
)

type void struct{}

// RatelimitExt
type RatelimitExt struct {
	NS             string
	app            *gobay.Application
	backend        RatelimitBackend
	prefix         string
	initialized    bool
	cachedFuncName map[string]void
}

var (
	_          gobay.Extension = (*RatelimitExt)(nil)
	backendMap                 = map[string](func() RatelimitBackend){}
	mu         sync.Mutex
)

// RatelimitBackend
type RatelimitBackend interface {
	Init(*viper.Viper) error
	Allow(ctx context.Context, key string, limitAmount, limitBaseSeconds int) (allowed int, remaining int, err error) // if record not exist, return (nil, nil)
}

// Init init a cache extension
func (c *RatelimitExt) Init(app *gobay.Application) error {
	if c.NS == "" {
		return errors.New("cachext: lack of NS")
	}
	mu.Lock()
	defer mu.Unlock()

	if c.initialized {
		return nil
	}
	c.app = app
	c.cachedFuncName = make(map[string]void)
	config := app.Config()
	config = gobay.GetConfigByPrefix(config, c.NS, true)
	c.prefix = config.GetString("prefix")
	backendConfig := config.GetString("backend")
	if backendFunc, exist := backendMap[backendConfig]; exist {
		c.backend = backendFunc()
		if err := c.backend.Init(config); err != nil {
			return err
		}
	} else {
		return errors.New("No backend found for ratelimit_backend:" + backendConfig)
	}

	c.initialized = true
	return nil
}

// CheckHealth - Check if extension is healthy
func (c *RatelimitExt) CheckHealth(ctx context.Context) error {
	return nil
}

// RegisteBackend if you want a new backend, use this func to registe your backend
// then load it by config
func RegisteBackend(configBackend string, backendFunc func() RatelimitBackend) error {
	mu.Lock()
	defer mu.Unlock()
	if _, exist := backendMap[configBackend]; !exist {
		backendMap[configBackend] = backendFunc
		return nil
	} else {
		return errors.New("Backend already registered")
	}
}

// Close
func (c *RatelimitExt) Close() error {
	return nil
}

// Object
func (c *RatelimitExt) Object() interface{} {
	return c
}

// Application
func (c *RatelimitExt) Application() *gobay.Application {
	return c.app
}

// Allow - check if ratelimit allows
func (c *RatelimitExt) Allow(ctx context.Context, pathKey, ip string, limitAmount, limitBaseSeconds int) (bool, error) {
	ratelimitKey := c.prefix + ":" + pathKey + ":" + ip
	allow, _, err := c.backend.Allow(ctx, ratelimitKey, limitAmount, limitBaseSeconds)
	isAllow := allow == 1
	return isAllow, err
}
