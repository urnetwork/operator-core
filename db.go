// Env.Db: the database a process grants (§4.5; owner decision 251), server's Postgres in server. It mirrors
// server's helpers (server/db.go) as methods on an instance, in this package's own types, so an extension
// names no pgx type: callback-style Db and Tx whose implementation owns retries, the isolation and
// read-before-begin options, Query, Exec and Batch, Rows that scan into standard types and Id, named args
// and temp tables.
//
// Each process grants it only where its role allows: evs and gkm get none (owner decisions 2, 19, 21), so a
// planted Postgres touch from either still fails. An extension's SQL touches only its own tables;
// server-owned data reaches it through domain interfaces, not through this.
//
// Server's locking rules hold here: never take a second connection while a callback holds one, and never
// nest transactions, savepoints included. A callback that needs the database again uses the conn or tx it
// was handed.

package operator

import (
	"context"
	"errors"
	"time"
)

var (
	// What a callback returns to ask for a rerun of the whole call, as server's PgRetry does. It reruns
	// only under the retry option's rules and budget, and ends the call wrapped once those are spent.
	ErrDbRerun = errors.New("db rerun requested")
	// A statement's wait for a lock passed the call's bound (Postgres's 55P03). The call rolled back and
	// was not rerun, since a rerun would queue again while holding a pooled server: the caller's own retry
	// decides. The implementation wraps the database's error with this.
	ErrDbLockTimeout = errors.New("db lock timeout")
)

// The database. Instances are safe for concurrent use.
type Db interface {
	// Runs callback on one connection. Without DbReadWrite the session keeps the database's default
	// access, which a host may hold read-only; DbReadWrite makes it read-write. The implementation owns
	// retries: a connection failure before callback wrote anything, and a transient failure (a
	// serialization failure, a deadlock, ErrDbRerun) rerun callback whole after a backoff and within the
	// implementation's budget, so callback must be safe to run again and publish nothing outside the
	// database before it returns. Any other error callback returns ends the call with that error. A
	// cancelled ctx ends the call with an error that errors.Is reads as ctx's.
	Db(ctx context.Context, callback func(conn DbConn) error, options ...DbOption) error
	// Runs callback in one transaction, read-write at repeatable read unless options say otherwise,
	// committing when callback returns nil and rolling back otherwise. It retries as Db does, a commit
	// failure included where a rerun can resolve it. A statement's lock wait is bounded by the host's own
	// bound unless DbLockTimeout replaces it, and a wait past it ends the call with ErrDbLockTimeout.
	Tx(ctx context.Context, callback func(tx DbTx) error, options ...DbOption) error
}

// What a connection and a transaction both run. Every statement's args may be standard types, Id and *Id
// (a nil *Id is SQL NULL), or one DbNamedArgs alone, which names them as @name in sql.
type DbQuerier interface {
	// The rows sql selects. The caller closes them, and reads Err once Next is false.
	Query(ctx context.Context, sql string, args ...any) (DbRows, error)
	// Runs sql and says how many rows it touched.
	Exec(ctx context.Context, sql string, args ...any) (DbResult, error)
	// Sends batch's statements in one round trip and hands their results, in queue order, to read, which
	// may be nil. The results close once read returns, and the first error, read's or a statement's, ends
	// the call.
	Batch(ctx context.Context, batch *DbBatch, read func(results DbBatchResults) error) error
}

// One connection, inside a Db callback. Valid only during the callback.
type DbConn interface {
	DbQuerier
}

// One transaction, inside a Tx callback. Valid only during the callback.
type DbTx interface {
	DbQuerier
	// Creates a temporary table that drops at commit, keyed by every column, and inserts values, skipping
	// a duplicate. spec is table_name(column type[, ...]). A value with several columns is a
	// DbComplexValue.
	CreateTempTable(ctx context.Context, spec string, values ...any) error
	// CreateTempTable with no key, so duplicates stay.
	CreateTempTableAllowDuplicates(ctx context.Context, spec string, values ...any) error
	// A many-to-one join table: spec is table_name(key_column type[, ...] -> value_column type[ NULL][,
	// ...]), keyed by the key columns, and keyValues fills it.
	CreateTempJoinTable(ctx context.Context, spec string, keyValues map[any]any) error
}

// A row value that fills several columns of a temp table, in the spec's order.
type DbComplexValue interface {
	Values() []any
}

// Rows, read in order. Not safe for concurrent use.
type DbRows interface {
	// Moves to the next row, false once there is none or a read failed.
	Next() bool
	// The current row's columns into dest: pointers to standard types, Id and *Id. A NULL column needs a
	// pointer to a pointer.
	Scan(dest ...any) error
	// What ended the rows early, nil when they ran out.
	Err() error
	// Releases the rows. Safe more than once.
	Close()
}

// What a statement did.
type DbResult struct {
	// The rows it inserted, updated, deleted or selected, as the database reports them.
	RowsAffected int64
}

// The results of a sent batch, one per queued statement, read in queue order. Not safe for concurrent use.
type DbBatchResults interface {
	// The next statement's outcome, as Exec.
	Exec() (DbResult, error)
	// The next statement's rows, as Query. The caller closes them before reading the next.
	Query() (DbRows, error)
}

// Statements queued to send in one round trip. The zero value is an empty batch.
type DbBatch struct {
	statements []dbBatchStatement
}

// One queued statement.
type dbBatchStatement struct {
	sql  string
	args []any
}

// Adds a statement, with args as DbQuerier takes them.
func (self *DbBatch) Queue(sql string, args ...any) {
	self.statements = append(self.statements, dbBatchStatement{sql: sql, args: args})
}

// How many statements are queued.
func (self *DbBatch) Len() int {
	return len(self.statements)
}

// The statement at index, in queue order, as Queue took it: what an implementation sends.
func (self *DbBatch) Statement(index int) (sql string, args []any) {
	statement := self.statements[index]
	return statement.sql, statement.args
}

// A statement's args by name, written @name in its sql. Passed as a statement's one arg.
type DbNamedArgs map[string]any

// One option of a Db or Tx call: one of the kinds below, and no other. Of two options of one kind, the
// later one stands.
type DbOption interface {
	dbOption()
}

// A transaction's isolation. Tx's default is repeatable read.
type DbIsolation int

const (
	DbIsolationReadCommitted  DbIsolation = 1
	DbIsolationRepeatableRead DbIsolation = 2
	DbIsolationSerializable   DbIsolation = 3
)

func (self DbIsolation) dbOption() {}

// Read-only or read-write. Db's default leaves the session's own default, as server's helpers do, and
// Tx's default is read-write; a process the host holds read-only stays read-only whatever this says.
type DbAccess int

const (
	DbReadOnly  DbAccess = 1
	DbReadWrite DbAccess = 2
)

func (self DbAccess) dbOption() {}

// Whether the call reruns callback. The default reruns what the implementation classifies transient.
type DbRetry int

const (
	// Reruns transient failures, within the implementation's backoff and budget.
	DbRetryTransient DbRetry = 1
	// Runs callback once, whatever fails.
	DbRetryNone DbRetry = 2
)

func (self DbRetry) dbOption() {}

// An authoritative read on the call's own connection before the transaction takes its first snapshot,
// as server's TxReadBeforeBegin: it runs for every attempt, a rerun included, and must not write or
// publish anything. Its error ends the call.
type DbReadBeforeBegin func(ctx context.Context, querier DbQuerier) error

func (self DbReadBeforeBegin) dbOption() {}

// How long each of the transaction's statements may wait for a lock, in place of the host's own bound.
// Not positive leaves lock waits to the database's default, unbounded.
type DbLockTimeout time.Duration

func (self DbLockTimeout) dbOption() {}
