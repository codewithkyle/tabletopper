package controllers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
)

// A database that answers every query with one row of the caller's choosing.
//
// IT SITS A LAYER BELOW recordingDB AND ANSWERS THE OPPOSITE QUESTION.
// recordingDB is a queries.DBTX and reports what statement a handler wrote,
// which is what most of the tests in this package want; what it cannot do is
// hand back a row, because a *sql.Row that scans successfully can only be
// built by database/sql. So this is a driver.Connector instead, and it is for
// the handlers whose behaviour depends on what the row SAYS rather than on
// which statement went out.
//
// One row, whatever the query. Nothing here dispatches on SQL: a test using
// this drives a handler that reads exactly one row, and a handler that read a
// second would be handed the same one, which is a failure the test would see.
type oneRowDB struct {
	columns []string
	values  []driver.Value
}

// db wraps the stub in a *sql.DB ready for queries.New.
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
