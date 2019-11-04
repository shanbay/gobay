// Copyright (c) 2019 leosocy, leosocy@gmail.com
// Use of this source code is governed by a MIT-style license
// that can be found in the LICENSE file.

package testapp

import (
	_ "github.com/mattn/go-sqlite3"
	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/gormext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type testApplicationProvider struct{
	mock.Mock
}

func (p *testApplicationProvider) ProvideExtensions() map[gobay.Key]gobay.Extension {
	p.MethodCalled("ProvideExtensions")
	return map[gobay.Key]gobay.Extension{
		"db": &gormext.GormExt{},
	}
}

func TestCreateApp(t *testing.T) {
	assert := assert.New(t)
	provider := &testApplicationProvider{}
	provider.On("ProvideExtensions")
	// config file not found
	app, err := gobay.CreateApp("..", "testing", provider)
	assert.Nil(app)
	assert.NotNil(err)
	provider.AssertNumberOfCalls(t, "ProvideExtensions", 1)
	gotApp, err := gobay.GetApp()
	assert.Nil(gotApp)
	assert.NotNil(err)
	// success
	app, err = gobay.CreateApp(".", "testing", provider)
	assert.NotNil(app)
	assert.Nil(err)
	assert.NotNil(app.Get("db"))
	assert.Nil(app.Get("cache"))
	provider.AssertNumberOfCalls(t, "ProvideExtensions", 2)
	gotApp, err = gobay.GetApp()
	assert.Equal(app, gotApp)
	assert.Nil(err)
	// CreateApp again
	newApp, err := gobay.CreateApp(".", "testing", provider)
	assert.Equal(app, newApp)
	provider.AssertNumberOfCalls(t, "ProvideExtensions", 2)
	// Create using another loader
	var loader gobay.ApplicationLoader
	newApp, err = loader.CreateApp(".", "testing", provider)
	assert.NotEqual(app, newApp)
	provider.AssertNumberOfCalls(t, "ProvideExtensions", 3)
}
