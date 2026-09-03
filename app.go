package gobay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/shanbay/gobay/observability"
	"github.com/spf13/viper"
)

// A Key represents a key for a Extension.
type Key string

// Extension like db, cache
type Extension interface {
	Object() interface{}
	Application() *Application
	Init(app *Application) error
	Close() error
}

// Application struct
type Application struct {
	rootPath    string
	env         string
	config      *viper.Viper
	extensions  map[Key]Extension
	initialized bool
	closed      bool
	mu          sync.Mutex
	shutdown    func(context.Context) error
}

// Get the extension at the specified key, return nil when the component doesn't exist
func (d *Application) Get(key Key) Extension {
	ext, _ := d.GetOK(key)
	return ext
}

// GetOK the extension at the specified key, return false when the component doesn't exist
func (d *Application) GetOK(key Key) (Extension, bool) {
	ext, ok := d.extensions[key]
	if !ok {
		return nil, ok
	}
	return ext, ok
}

func (d *Application) Env() string {
	return d.env
}

// Config returns the viper config for this application
func (d *Application) Config() *viper.Viper {
	return d.config
}

// Init the application and its extensions with the config.
func (d *Application) Init() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.initialized {
		return nil
	}

	if err := d.initConfig(); err != nil {
		return err
	}
	d.setup()
	if err := d.initExtensions(); err != nil {
		return err
	}
	d.initialized = true
	return nil
}

func (d *Application) initConfig() error {
	configfile := filepath.Join(d.rootPath, "config.yaml")
	originConfig, err := os.ReadFile(configfile)
	if err != nil {
		return err
	}
	renderedConfig := []byte(os.ExpandEnv(string(originConfig)))
	config := viper.New()
	config.SetConfigType("yaml")
	if err := config.ReadConfig(bytes.NewBuffer(renderedConfig)); err != nil {
		return err
	}
	config = config.Sub(d.env)

	// add default config
	config.SetDefault("debug", false)
	config.SetDefault("testing", false)
	config.SetDefault("timezone", "UTC")
	config.SetDefault("grpc_listen_host", "localhost")
	config.SetDefault("grpc_listen_port", 6000)
	config.SetDefault("openapi_listen_host", "localhost")
	config.SetDefault("openapi_listen_port", 3000)

	d.config = config

	return nil
}

func (d *Application) setup() {
	d.shutdown = observability.Initialize()
}

func (d *Application) initExtensions() error {
	var allerr error
	for key, ext := range d.extensions {
		if err := ext.Init(d); err != nil {
			allerr = multierror.Append(allerr, errors.New(string(key)), err)
		}
	}
	return allerr
}

// Close close app when exit
func (d *Application) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return nil
	}

	if err := d.closeExtensions(); err != nil {
		return err
	}
	err := d.shutdown(context.Background())
	if err != nil {
		return err
	}
	d.closed = true
	return nil
}

func (d *Application) closeExtensions() error {
	for _, ext := range d.extensions {
		if err := ext.Close(); err != nil {
			return err
		}
	}
	return nil
}

// CreateApp create an gobay Application
func CreateApp(rootPath string, env string, exts map[Key]Extension) (*Application, error) {
	if rootPath == "" || env == "" {
		return nil, fmt.Errorf("lack of rootPath or env")
	}

	app := &Application{rootPath: rootPath, env: env, extensions: exts}

	if err := app.Init(); err != nil {
		return nil, err
	}
	return app, nil
}

// NormalizeUnderscoreKeys 让带下划线的配置键名也能被 Unmarshal 匹配到。
//
// viper 的 Unmarshal 底层是 mapstructure，它只做大小写折叠、不做下划线归一化：
// 结构体字段 PoolSize 能匹配 poolsize / POOLSIZE，但匹配不上 pool_size。历史上
// <NS>pool_size 这样的写法会被静默忽略，对应字段保持零值，排查起来很费劲。
// 这里在 Unmarshal 之前把带下划线的键补一份去掉下划线的写法。
//
// 两种写法并存时以**不带下划线的**为准。交给 mapstructure 自行处理的话，两个键
// 都会匹配上同一个字段，最终哪个生效取决于 map 的迭代顺序，是不确定的。
//
// 必须在 SetDefault 之前调用，否则 IsSet 会因为默认值而恒为真，感知不到用户实际
// 配的是哪种写法。
func NormalizeUnderscoreKeys(config *viper.Viper) {
	for _, key := range config.AllKeys() {
		flat := strings.ReplaceAll(key, "_", "")
		if flat == key || config.IsSet(flat) {
			continue
		}
		config.Set(flat, config.Get(key))
	}
}

func GetConfigByPrefix(config *viper.Viper, prefix string, trimPrefix bool) *viper.Viper {
	subConfig := viper.New()
	for k, v := range config.AllSettings() {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		key := k
		if trimPrefix {
			key = k[len(prefix):]
		}
		subConfig.Set(key, v)
	}
	return subConfig
}
