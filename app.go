package gobay

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

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
	if err := d.initExtensions(); err != nil {
		return err
	}
	d.initialized = true
	return nil
}

func (d *Application) initConfig() error {
	configfile := filepath.Join(d.rootPath, "config.yaml")
	originConfig, err := ioutil.ReadFile(configfile)
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
	config.SetDefault("openapi_host", "localhost")
	config.SetDefault("openapi_port", 3000)

	d.config = config

	return nil
}

func (d *Application) initExtensions() error {
	for _, ext := range d.extensions {
		if err := ext.Init(d); err != nil {
			return err
		}
	}
	return nil
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
