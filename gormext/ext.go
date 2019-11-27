package gormext

import (
	"github.com/jinzhu/gorm"
	"github.com/shanbay/gobay"
)

// GormExt gorm extention
type GormExt struct {
	NS  string
	app *gobay.Application
	db  *gorm.DB
}

// Init implements Extension interface
func (d *GormExt) Init(app *gobay.Application) error {
	d.app = app
	config := app.Config()
	if d.NS != "" {
		config = config.Sub(d.NS)
		config.SetEnvPrefix(d.NS)
	}
	config.AutomaticEnv()
	dbURL := config.GetString("db_url")
	dbDriver := config.GetString("db_driver")
	db, err := gorm.Open(dbDriver, dbURL)
	if err != nil {
		return err
	}
	sqldb := db.DB()
	if config.IsSet("conn_max_lifetime") {
		sqldb.SetConnMaxLifetime(config.GetDuration("conn_max_lifetime"))
	}
	if config.IsSet("max_open_conns") {
		sqldb.SetMaxOpenConns(config.GetInt("max_open_conns"))
	}
	if config.IsSet("max_idle_conns") {
		sqldb.SetMaxIdleConns(config.GetInt("max_idle_conns"))
	}
	d.db = db
	return nil
}

// Close implements Extension interface
func (d *GormExt) Close() error {
	return d.db.Close()
}

// Object implements Extension interface
func (d *GormExt) Object() interface{} {
	return d.db
}

// Application implements Extension interface
func (d *GormExt) Application() *gobay.Application {
	return d.app
}
