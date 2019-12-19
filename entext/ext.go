package entext

import (
	"github.com/facebookincubator/ent/dialect"
	"github.com/facebookincubator/ent/dialect/sql"
	"github.com/shanbay/gobay"
)

const (
	defaultMaxOpenConns = 15
	defaultMaxIdleConns = 5
)

type Client interface {
	Close() error
}

type EntExt struct {
	NS        string
	NewClient func(interface{}) Client
	Driver    func(dialect.Driver) interface{}

	drv    *sql.Driver
	client Client
	app    *gobay.Application
}

func (d *EntExt) Object() interface{} { return d.client }

func (d *EntExt) Application() *gobay.Application { return d.app }

func (d *EntExt) Init(app *gobay.Application) error {
	d.app = app
	config := app.Config()
	if d.NS != "" {
		config = config.Sub(d.NS)
	}
	config.SetDefault("max_open_conns", defaultMaxOpenConns)
	config.SetDefault("max_idle_conns", defaultMaxIdleConns)
	dbURL := config.GetString("db_url")
	dbDriver := config.GetString("db_driver")
	drv, err := sql.Open(dbDriver, dbURL)
	if err != nil {
		return err
	}
	db := drv.DB()
	if config.IsSet("conn_max_lifetime") {
		db.SetConnMaxLifetime(config.GetDuration("conn_max_lifetime"))
	}
	if config.IsSet("max_open_conns") {
		db.SetMaxOpenConns(config.GetInt("max_open_conns"))
	}
	if config.IsSet("max_idle_conns") {
		db.SetMaxIdleConns(config.GetInt("max_idle_conns"))
	}
	d.client = d.NewClient(d.Driver(drv))
	return nil
}

func (d *EntExt) Close() error { return d.client.Close() }
