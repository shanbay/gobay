package entext

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/XSAM/otelsql"
	_ "github.com/go-sql-driver/mysql"
	"go.elastic.co/apm/apmtest"
	"go.elastic.co/apm/module/apmsql"
	_ "go.elastic.co/apm/module/apmsql/mysql"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// 这些测试需要真实 MySQL，通过 ENTEXT_TEST_DSN 提供，未设置时跳过
// （与 apmsql/mysql 自身测试用 MYSQL_HOST 的约定一致）。
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("ENTEXT_TEST_DSN")
	if dsn == "" {
		t.Skip("ENTEXT_TEST_DSN 未设置，跳过")
	}
	return dsn
}

// TestDualTracedDBEmitsBothSpans 验证 APM 与 OTel 双开时，同一条 SQL 在两套链路
// 里各产出一个 span——这是本次改动的核心目的。
func TestDualTracedDBEmitsBothSpans(t *testing.T) {
	dsn := testDSN(t)

	// otelsql 在包装时从全局 TracerProvider 取 tracer，必须先设置。
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	db, err := openDualTracedDB("mysql", dsn)
	if err != nil {
		t.Fatalf("openDualTracedDB 失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, apmSpans, _ := apmtest.WithTransaction(func(ctx context.Context) {
		ctx, span := tp.Tracer("entext-test").Start(ctx, "parent")
		defer span.End()

		var n int
		if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&n); err != nil {
			t.Errorf("查询失败: %v", err)
			return
		}
		if n != 1 {
			t.Errorf("SELECT 1 返回 %d", n)
		}
	})

	var apmDBSpan bool
	for _, s := range apmSpans {
		t.Logf("APM span: name=%q type=%q subtype=%q", s.Name, s.Type, s.Subtype)
		if s.Subtype == "mysql" {
			apmDBSpan = true
		}
	}
	if !apmDBSpan {
		t.Errorf("APM 侧没有产出 mysql span，共 %d 个 span", len(apmSpans))
	}

	var otelDBSpan bool
	for _, s := range exp.GetSpans() {
		t.Logf("OTel span: name=%q kind=%v", s.Name, s.SpanKind)
		if s.Name != "parent" && strings.Contains(strings.ToLower(s.Name), "sql") {
			otelDBSpan = true
		}
	}
	if !otelDBSpan {
		t.Errorf("OTel 侧没有产出 SQL span，共 %d 个 span", len(exp.GetSpans()))
	}
}

// countingDriver 统计真实的物理连接建立次数，放在包装链最内层。
type countingDriver struct {
	driver.Driver
	opens atomic.Int64
}

func (d *countingDriver) Open(name string) (driver.Conn, error) {
	d.opens.Add(1)
	return d.Driver.Open(name)
}

// TestDualTracedDBCtxCancelKeepsConn 锁定包装顺序对连接池的实际影响。
//
// database/sql 的 keepConnOnRollback 取决于最外层 driver.Conn 是否实现
// driver.Validator（sql.go:1904），它只在 (*Tx).awaitDone 里生效（sql.go:2220），
// 即只影响事务 ctx 被取消/超时的路径。若把未实现 Validator 的 otelsql 放到外层，
// 每个被取消的事务都会销毁自己的连接。
//
// 实测：30 次「事务+查询+ctx取消」，apmsql 在外新建 1 条连接，otelsql 在外新建 30 条。
func TestDualTracedDBCtxCancelKeepsConn(t *testing.T) {
	dsn := testDSN(t)
	const rounds = 30

	base, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	counter := &countingDriver{Driver: base.Driver()}
	_ = base.Close()

	// 与 openDualTracedDB 相同的包装顺序，只是最内层多一个计数器。
	traced := otelsql.WrapDriver(counter, otelSpanOption())
	traced = apmsql.Wrap(traced,
		apmsql.WithDriverName("mysql"),
		apmsql.WithDSNParser(apmsql.DriverDSNParser("mysql")),
	)
	db := sql.OpenDB(dsnConnector{dsn: dsn, drv: traced})
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	t.Cleanup(func() { _ = db.Close() })

	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := counter.opens.Load()

	for i := 0; i < rounds; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		var n int
		if err := tx.QueryRowContext(ctx, "SELECT 1").Scan(&n); err != nil {
			cancel()
			t.Fatal(err)
		}
		cancel() // 触发 awaitDone -> rollback(discardConn)
		for j := 0; j < 5000 && db.Stats().InUse > 0; j++ {
			runtime.Gosched()
		}
	}

	opened := counter.opens.Load() - before
	t.Logf("%d 次「事务+查询+ctx取消」后新建连接数 = %d", rounds, opened)
	if opened > rounds/3 {
		t.Errorf("新建连接数 %d 过高（%d 轮）：连接在 ctx 取消后被销毁而非归还连接池，"+
			"说明最外层 conn 未实现 driver.Validator——包装顺序被改动了", opened, rounds)
	}
}
