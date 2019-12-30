package entext

import (
	"database/sql"
	"github.com/facebookincubator/ent/dialect"
	entsql "github.com/facebookincubator/ent/dialect/sql"
	"github.com/shanbay/gobay"
	"go.elastic.co/apm/module/apmsql"
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

	drv    *entsql.Driver
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
	config.SetDefault("db_max_open_conns", defaultMaxOpenConns)
	config.SetDefault("db_max_idle_conns", defaultMaxIdleConns)
	dbURL := config.GetString("db_url")
	dbDriver := config.GetString("db_driver")

	var db *sql.DB
	var err error
	if app.Config().GetBool("elastic_apm_enable") {
		db, err = apmsql.Open(dbDriver, dbURL)
	} else {
		db, err = sql.Open(dbDriver, dbURL)
	}
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(config.GetInt("db_max_open_conns"))
	db.SetMaxIdleConns(config.GetInt("db_max_idle_conns"))
	if config.IsSet("db_conn_max_lifetime") {
		db.SetConnMaxLifetime(config.GetDuration("db_conn_max_lifetime"))
	}
	drv := entsql.OpenDB(dbDriver, db)
	d.drv = drv
	d.client = d.NewClient(d.Driver(drv))
	return nil
}

func (d *EntExt) Close() error { return d.client.Close() }
