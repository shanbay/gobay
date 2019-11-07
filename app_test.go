package gobay

import (
	"errors"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type testExtension struct {
	mock.Mock
}

func (e *testExtension) Init(app *Application) error {
	args := e.Called("Init")
	return args.Error(0)
}

func (e *testExtension) Close() error {
	args := e.Called("Close")
	return args.Error(0)
}

func (e *testExtension) Application() *Application {
	return nil
}

func (e *testExtension) Object() interface{} {
	return nil
}

func TestCreateApp(t *testing.T) {
	assert := assert.New(t)
	testExt := new(testExtension)
	exts := map[Key]Extension{
		"test": testExt,
	}
	// config file not found
	call := testExt.On("Init", mock.Anything)
	call.Return(nil)
	app, err := CreateApp("..", "testing", exts)
	assert.Nil(app)
	assert.NotNil(err)
	testExt.AssertNotCalled(t, "Init")
	// success
	app, err = CreateApp(".", "testing", exts)
	assert.NotNil(app)
	assert.Nil(err)
	assert.NotNil(app.Get("test"))
	assert.Nil(app.Get("cache"))
	testExt.AssertNumberOfCalls(t, "Init", 1)
	// call extension.Init failed
	initErr := errors.New("init failed")
	call.Return(initErr)
	app, err = CreateApp(".", "testing", exts)
	assert.Nil(app)
	assert.Equal(initErr, err)
}

func TestApplicationClose(t *testing.T) {
	assert := assert.New(t)
	testExt := new(testExtension)
	exts := map[Key]Extension{
		"test": testExt,
	}
	testExt.On("Init", mock.Anything).Return(nil)
	app, _ := CreateApp(".", "testing", exts)
	// call extension.Close failed
	closeErr := errors.New("close failed")
	call := testExt.On("Close", mock.Anything)
	call.Return(closeErr)
	err := app.Close()
	assert.Equal(err, closeErr)
	testExt.AssertNumberOfCalls(t, "Close", 1)
	// success
	call.Return(nil)
	err = app.Close()
	assert.Nil(err)
	testExt.AssertNumberOfCalls(t, "Close", 2)
	// close again
	err = app.Close()
	assert.Nil(err)
	testExt.AssertNumberOfCalls(t, "Close", 2)
}
