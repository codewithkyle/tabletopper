package controllers
import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
)
type oneRowDB struct {
	columns []string
	values  []driver.Value
}
func (s oneRowDB) db() *sql.DB { return sql.OpenDB(s) }
func (s oneRowDB) Connect(context.Context) (driver.Conn, error) { return oneRowConn{s}, nil }
func (s oneRowDB) Driver() driver.Driver { return nil }
type oneRowConn struct{ stub oneRowDB }
func (c oneRowConn) Prepare(string) (driver.Stmt, error) { return oneRowStmt{c.stub}, nil }
func (c oneRowConn) Close() error                        { return nil }
func (c oneRowConn) Begin() (driver.Tx, error)           { return nil, io.ErrUnexpectedEOF }
type oneRowStmt struct{ stub oneRowDB }
func (s oneRowStmt) Close() error  { return nil }
func (s oneRowStmt) NumInput() int { return -1 }
func (s oneRowStmt) Exec([]driver.Value) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}
func (s oneRowStmt) Query([]driver.Value) (driver.Rows, error) {
	return &oneRowRows{stub: s.stub}, nil
}
type oneRowRows struct {
	stub oneRowDB
	done bool
}
func (r *oneRowRows) Columns() []string { return r.stub.columns }
func (r *oneRowRows) Close() error      { return nil }
func (r *oneRowRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	copy(dest, r.stub.values)
	return nil
}
