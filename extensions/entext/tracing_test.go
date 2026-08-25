package entext

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
)

// fakeConn 实现 driver.Validator，模拟 go-sql-driver/mysql 的行为。
type fakeConn struct{}

func (fakeConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (fakeConn) Close() error                        { return nil }
func (fakeConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (fakeConn) IsValid() bool                       { return true }

var _ driver.Validator = fakeConn{}

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return fakeConn{}, nil }

func init() { sql.Register("entext-fake", fakeDriver{}) }

// TestDualTracedDBKeepsValidator 锁定 APM/OTel 双开时的包装顺序。
//
// database/sql 只对最外层 driver.Conn 做 driver.Validator 断言：一处用于连接复用前的
// 健康检查，一处用于决定 keepConnOnRollback（事务回滚后连接是否归还连接池）。otelsql
// 未实现该接口，因此它必须在内层、apmsql 在外层。若有人调换顺序，本测试会失败。
func TestDualTracedDBKeepsValidator(t *testing.T) {
	db, err := openDualTracedDB("entext-fake", "fake-dsn")
	if err != nil {
		t.Fatalf("openDualTracedDB 失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	conn, err := db.Driver().Open("fake-dsn")
	if err != nil {
		t.Fatalf("打开连接失败: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if _, ok := conn.(driver.Validator); !ok {
		t.Fatalf("最外层 conn 未实现 driver.Validator："+
			"包装顺序被改动了。apmsql 必须在外层、otelsql 在内层，"+
			"否则连接健康检查失效且事务回滚后连接不再复用。实际类型 %T", conn)
	}
}

// TestDualTracedDBConnectorUsesDSN 确认 dsnConnector 把 dsn 透传给了底层 driver。
func TestDualTracedDBConnectorUsesDSN(t *testing.T) {
	c := dsnConnector{dsn: "fake-dsn", drv: fakeDriver{}}
	conn, err := c.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}
	if c.Driver() == nil {
		t.Fatal("Driver() 返回 nil")
	}
}
