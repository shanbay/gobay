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

// Init implements Extention interface
func (d *GormExt) Init(app *gobay.Application) error {
	d.app = app
	config := app.Config()
	if d.NS != "" {
		config = config.Sub(d.NS)
	}
	dbURL := config.GetString("db_url")
	dbDriver := config.GetString("db_driver")
	db, err := gorm.Open(dbDriver, dbURL)
	if err != nil {
		return err
	}
	d.db = db
	return nil
}

// Close implements Extention interface
func (d *GormExt) Close() error {
	return d.db.Close()
}

// Object implements Extention interface
func (d *GormExt) Object() interface{} {
	return d.db
}

// Application implements Extention interface
func (d *GormExt) Application() *gobay.Application {
	return d.app
}
