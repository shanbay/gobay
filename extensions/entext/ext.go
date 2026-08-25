package entext

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/XSAM/otelsql"
	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/observability"
	"go.elastic.co/apm/module/apmsql"
	_ "go.elastic.co/apm/module/apmsql/mysql"
	"go.opentelemetry.io/otel/trace"
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

	IsNotFound          func(error) bool
	IsConstraintFailure func(error) bool
	IsNotSingular       func(error) bool

	drv    *entsql.Driver
	client Client
	app    *gobay.Application
}

func (d *EntExt) Object() interface{} { return d.client }

func (d *EntExt) Application() *gobay.Application { return d.app }

func (d *EntExt) Init(app *gobay.Application) error {
	if d.NS == "" {
		return errors.New("lack of NS")
	}
	d.app = app
	config := gobay.GetConfigByPrefix(app.Config(), d.NS, true)
	config.SetDefault("max_open_conns", defaultMaxOpenConns)
	config.SetDefault("max_idle_conns", defaultMaxIdleConns)
	dbURL := config.GetString("url")
	dbDriver := config.GetString("driver")

	var db *sql.DB
	var err error
	apmEnable := observability.GetApmEnable()
	otelEnable := observability.GetOtelEnable()
	switch {
	case apmEnable && otelEnable:
		db, err = openDualTracedDB(dbDriver, dbURL)
	case apmEnable:
		db, err = apmsql.Open(dbDriver, dbURL)
	case otelEnable:
		db, err = otelsql.Open(dbDriver, dbURL, otelSpanOption())
	default:
		db, err = sql.Open(dbDriver, dbURL)
	}
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(config.GetInt("max_open_conns"))
	db.SetMaxIdleConns(config.GetInt("max_idle_conns"))
	if config.IsSet("conn_max_lifetime") {
		db.SetConnMaxLifetime(config.GetDuration("conn_max_lifetime"))
	}
	drv := entsql.OpenDB(dbDriver, db)
	d.drv = drv
	d.client = d.NewClient(d.Driver(drv))
	return nil
}

// otelSpanOption 是 entext 使用的统一 otelsql 配置。
func otelSpanOption() otelsql.Option {
	return otelsql.WithSpanOptions(otelsql.SpanOptions{
		DisableErrSkip: true,
		SpanFilter: func(ctx context.Context, method otelsql.Method, query string, args []driver.NamedValue) bool {
			sc := trace.SpanContextFromContext(ctx)
			return sc.IsSampled() && sc.IsValid()
		},
	})
}

// openDualTracedDB 在 APM 与 OTel 同时开启时，把两套插桩叠加在同一个 driver 上，
// 而不是二选一（历史实现里 APM 分支会直接短路掉 OTel，导致 OTel 永远看不到 SQL）。
//
// 包装顺序必须是 apmsql 在外、otelsql 在内。database/sql 只对最外层的 driver.Conn
// 做 driver.Validator 断言，而 otelsql 未实现该接口。若把 otelsql 放在外层，会有两个
// 后果：连接复用前的健康检查被整体跳过；以及 keepConnOnRollback 变为 false，
// 事务回滚后连接被销毁而非归还连接池。apmsql 的 conn 静态保证实现 Validator，
// 放在最外层即可保住这两个行为。
func openDualTracedDB(dbDriver, dbURL string) (*sql.DB, error) {
	base, err := sql.Open(dbDriver, dbURL)
	if err != nil {
		return nil, err
	}
	// 只借用底层 driver。sql.Open 是惰性的，此处尚未建立任何真实连接，
	// 关掉它是为了不残留 connectionOpener goroutine。
	raw := base.Driver()
	if err := base.Close(); err != nil {
		return nil, err
	}
	traced := otelsql.WrapDriver(raw, otelSpanOption())
	traced = apmsql.Wrap(traced,
		apmsql.WithDriverName(dbDriver),
		apmsql.WithDSNParser(apmsql.DriverDSNParser(dbDriver)),
	)
	return sql.OpenDB(dsnConnector{dsn: dbURL, drv: traced}), nil
}

// dsnConnector 把 dsn 与已包装的 driver 组合成 driver.Connector。
// database/sql 没有「用给定 driver 实例 + dsn 打开」的公开入口，只能走 sql.OpenDB。
type dsnConnector struct {
	dsn string
	drv driver.Driver
}

func (c dsnConnector) Connect(_ context.Context) (driver.Conn, error) { return c.drv.Open(c.dsn) }

func (c dsnConnector) Driver() driver.Driver { return c.drv }

func (d *EntExt) Close() error { return d.client.Close() }

// DB 获取数据库，ent目前还不够完善，某些场景下还需要执行sql
func (d *EntExt) DB() *sql.DB {
	return d.drv.DB()
}

func (d *EntExt) CheckHealth(ctx context.Context) error {
	return d.DB().PingContext(ctx)
}
