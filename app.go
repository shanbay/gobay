package gobay

import (
	"fmt"
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

// newApplication returns a new application.
func newApplication(rootPath, env string, extensions map[Key]Extension) *Application {
	return &Application{
		rootPath:   rootPath,
		env:        env,
		extensions: extensions,
	}
}

// Get the extension at the specified key, return nil when the component doesn't exist
func (d *Application) Get(key Key) Extension {
	ext, _ := d.GetOK(key)
	return ext
}

// GetOK the extension at the specified key, return false when the component doesn't exist
func (d *Application) GetOK(key Key) (Extension, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ext, ok := d.extensions[key]
	if !ok {
		return nil, ok
	}
	return ext, ok
}

// Set the extension at the specified key
func (d *Application) Set(key Key, ext Extension) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.initialized {
		return fmt.Errorf("can't be called after app initialized")
	}

	if d.extensions == nil {
		d.extensions = make(map[Key]Extension)
	}

	d.extensions[key] = ext
	return nil
}

// SetMany set many extensions once. values will be override if same key occurred
func (d *Application) SetMany(exts map[Key]Extension) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.initialized {
		return fmt.Errorf("can't be called after app initialized")
	}

	if d.extensions == nil {
		d.extensions = make(map[Key]Extension, len(exts))
	}

	for k, v := range exts {
		d.extensions[k] = v
	}
	return nil
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
	config := viper.New()
	config.SetConfigFile(configfile)
	if err := config.ReadInConfig(); err != nil {
		return err
	}
	config = config.Sub(d.env)

	// add default config
	config.SetDefault("debug", false)
	config.SetDefault("testing", false)
	config.SetDefault("timezone", "UTC")
	config.SetDefault("grpc_host", "localhost")
	config.SetDefault("grpc_port", 6000)
	config.SetDefault("openapi_host", "localhost")
	config.SetDefault("openapi_port", 3000)

	// read env
	config.AutomaticEnv()

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

type ApplicationProvider interface {
	ProvideExtensions() map[Key]Extension
}

var (
	app *Application
	mu sync.Mutex
)

// CreateApp create an gobay Application.
func CreateApp(rootPath, env string, provider ApplicationProvider) (*Application, error) {
	mu.Lock()
	defer mu.Unlock()

	if app != nil && app.initialized {
		return app, nil
	}

	app = newApplication(rootPath, env, provider.ProvideExtensions())
	if err := app.Init(); err != nil {
		return nil, err
	}

	return app, nil
}

// GetApp return current app
func GetApp() (*Application, error) {
	mu.Lock()
	defer mu.Unlock()

	if app == nil || !app.initialized{
		return nil, fmt.Errorf("app not created")
	}

	return app, nil
}
