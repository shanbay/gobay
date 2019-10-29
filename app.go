package gobay

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// A Key represents a key for a Extention.
type Key string

// Extention like db, cache
type Extention interface {
	Object() interface{}
	Application() *Application
	Init(app *Application) error
	Close() error
}

// Application struct
type Application struct {
	RootPath    string
	Env         string
	config      *viper.Viper
	extentions  map[Key]Extention
	initialized bool
	mu          sync.Mutex
}

// Get the extention at the specified key, return nil when the component doesn't exist
func (d *Application) Get(key Key) Extention {
	ext, _ := d.GetOK(key)
	return ext
}

// GetOK the extention at the specified key, return false when the component doesn't exist
func (d *Application) GetOK(key Key) (Extention, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ext, ok := d.extentions[key]
	if !ok {
		return nil, ok
	}
	return ext, ok
}

// Set the extention at the specified key
func (d *Application) Set(key Key, ext Extention) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.initialized {
		return fmt.Errorf("can't be called after app initialized")
	}

	if d.extentions == nil {
		d.extentions = make(map[Key]Extention)
	}

	d.extentions[key] = ext
	return nil
}

// SetMany set many extentions once. values will be override if same key occurred
func (d *Application) SetMany(exts map[Key]Extention) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.initialized {
		return fmt.Errorf("can't be called after app initialized")
	}

	if d.extentions == nil {
		d.extentions = make(map[Key]Extention, len(exts))
	}

	for k, v := range exts {
		d.extentions[k] = v
	}
	return nil
}

// Config returns the viper config for this application
func (d *Application) Config() *viper.Viper {
	return d.config
}

// Init the application and its extentions with the config.
func (d *Application) Init() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.initialized {
		return nil
	}

	if err := d.initConfig(); err != nil {
		return err
	}
	if err := d.initExtentions(); err != nil {
		return err
	}
	d.initialized = true
	return nil
}

func (d *Application) initConfig() error {
	d.config = viper.New()

	// add default config
	d.config.SetDefault("DEBUG", false)
	d.config.SetDefault("TESTING", false)
	d.config.SetDefault("TIMEZONE", "UTC")

	originyaml := filepath.Join(d.RootPath, "config/config.yaml")
	configfile := filepath.Join(d.RootPath, ".config.yaml")

	var configmap map[string]interface{}
	instream, err := ioutil.ReadFile(originyaml)
	if err != nil {
		return err
	}
	yaml.Unmarshal(instream, &configmap)
	outstream, err := yaml.Marshal(configmap[d.Env])
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(configfile, outstream, 0644); err != nil {
		return err
	}

	d.config.SetConfigFile(configfile)
	if err := d.config.ReadInConfig(); err != nil {
		return err
	}
	d.config.AutomaticEnv()

	return nil
}

func (d *Application) initExtentions() error {
	for _, ext := range d.extentions {
		if err := ext.Init(d); err != nil {
			return err
		}
	}
	return nil
}
