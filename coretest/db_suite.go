package coretest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/urnetwork/connect"
	operator "github.com/urnetwork/operator-core"
)

// Holds db to the Db contract on a live database: scans into standard types and Id, named args, Exec's
// count, batches, temp tables, the access and isolation options, a read before begin, the error a callback
// returns, and a rerun asked for, with retries and without. It creates nothing but temporary tables, which
// drop at commit. A host runs it over the db it grants, as server/core does on the private test stack.
func DbSuite(t testing.TB, db operator.Db) {
	t.Helper()
	ctx := t.Context()
	checks := []struct {
		name  string
		check func(ctx context.Context, db operator.Db) error
	}{
		{name: "scans standard types and Id", check: dbCheckScans},
		{name: "takes named args", check: dbCheckNamedArgs},
		{name: "fills and reads temp tables", check: dbCheckTempTables},
		{name: "sends a batch", check: dbCheckBatch},
		{name: "keeps the access options", check: dbCheckAccess},
		{name: "keeps the isolation options", check: dbCheckIsolation},
		{name: "reads before begin", check: dbCheckReadBeforeBegin},
		{name: "returns a callback's error", check: dbCheckCallbackError},
		{name: "reruns on ErrDbRerun", check: dbCheckRerun},
		{name: "runs once without retries", check: dbCheckNoRetry},
	}
	for _, c := range checks {
		if err := c.check(ctx, db); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

// The one row a query answers, scanned into dest.
func dbQueryOne(ctx context.Context, querier operator.DbQuerier, dest []any, sql string, args ...any) error {
	rows, err := querier.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return errors.New("no row")
	}
	if err := rows.Scan(dest...); err != nil {
		return err
	}
	if rows.Next() {
		return errors.New("a second row")
	}
	return rows.Err()
}

func dbCheckScans(ctx context.Context, db operator.Db) error {
	id := operator.Id(connect.NewId())
	return db.Db(ctx, func(conn operator.DbConn) error {
		var count int64
		var text string
		var scannedId operator.Id
		var presentId *operator.Id
		var nullId *operator.Id
		err := dbQueryOne(ctx, conn, []any{&count, &text, &scannedId, &presentId, &nullId},
			`SELECT $1::bigint, $2::text, $3::uuid, $4::uuid, NULL::uuid`, int64(7), "seven", id, &id)
		if err != nil {
			return err
		}
		if count != 7 || text != "seven" || scannedId != id || presentId == nil || *presentId != id || nullId != nil {
			return fmt.Errorf("scanned %d %q %v %v %v", count, text, scannedId, presentId, nullId)
		}
		return nil
	})
}

func dbCheckNamedArgs(ctx context.Context, db operator.Db) error {
	return db.Db(ctx, func(conn operator.DbConn) error {
		var sum int64
		if err := dbQueryOne(ctx, conn, []any{&sum}, `SELECT @a::bigint + @b::bigint`, operator.DbNamedArgs{"a": int64(4), "b": int64(5)}); err != nil {
			return err
		}
		if sum != 9 {
			return fmt.Errorf("sum %d", sum)
		}
		return nil
	})
}

func dbCheckTempTables(ctx context.Context, db operator.Db) error {
	firstId := operator.Id(connect.NewId())
	secondId := operator.Id(connect.NewId())
	return db.Tx(ctx, func(tx operator.DbTx) error {
		if err := tx.CreateTempTable(ctx, "coretest_ids(id uuid)", firstId, secondId, firstId); err != nil {
			return err
		}
		result, err := tx.Exec(ctx, `DELETE FROM coretest_ids WHERE id = $1`, firstId)
		if err != nil {
			return err
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("a keyed temp table kept a duplicate: deleted %d", result.RowsAffected)
		}
		var kept operator.Id
		if err := dbQueryOne(ctx, tx, []any{&kept}, `SELECT id FROM coretest_ids`); err != nil {
			return err
		}
		if kept != secondId {
			return fmt.Errorf("kept %v, want %v", kept, secondId)
		}
		if err := tx.CreateTempTableAllowDuplicates(ctx, "coretest_repeats(id uuid)", firstId, firstId); err != nil {
			return err
		}
		var repeatCount int64
		if err := dbQueryOne(ctx, tx, []any{&repeatCount}, `SELECT COUNT(*) FROM coretest_repeats`); err != nil {
			return err
		}
		if repeatCount != 2 {
			return fmt.Errorf("an unkeyed temp table holds %d rows, want 2", repeatCount)
		}
		if err := tx.CreateTempJoinTable(ctx, "coretest_join(key uuid -> value text)", map[any]any{firstId: "first"}); err != nil {
			return err
		}
		var value string
		if err := dbQueryOne(ctx, tx, []any{&value}, `SELECT value FROM coretest_join WHERE key = $1`, firstId); err != nil {
			return err
		}
		if value != "first" {
			return fmt.Errorf("joined %q", value)
		}
		return nil
	})
}

func dbCheckBatch(ctx context.Context, db operator.Db) error {
	return db.Tx(ctx, func(tx operator.DbTx) error {
		var batch operator.DbBatch
		batch.Queue(`SELECT $1::bigint`, int64(1))
		batch.Queue(`SELECT $1::bigint`, int64(2))
		values := []int64{}
		err := tx.Batch(ctx, &batch, func(results operator.DbBatchResults) error {
			for range batch.Len() {
				rows, err := results.Query()
				if err != nil {
					return err
				}
				for rows.Next() {
					var value int64
					if err := rows.Scan(&value); err != nil {
						rows.Close()
						return err
					}
					values = append(values, value)
				}
				rows.Close()
				if err := rows.Err(); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !slices.Equal(values, []int64{1, 2}) {
			return fmt.Errorf("batch read %v", values)
		}
		return nil
	})
}

// What SHOW answers for a setting, on querier.
func dbShow(ctx context.Context, querier operator.DbQuerier, setting string) (string, error) {
	var value string
	err := dbQueryOne(ctx, querier, []any{&value}, `SELECT current_setting($1)`, setting)
	return value, err
}

func dbCheckAccess(ctx context.Context, db operator.Db) error {
	// Db's default leaves the session's own default, which the host sets, so only the options are read
	for _, c := range []struct {
		run  func(func(querier operator.DbQuerier) error) error
		want string
	}{
		{
			run: func(read func(querier operator.DbQuerier) error) error {
				return db.Db(ctx, func(conn operator.DbConn) error { return read(conn) }, operator.DbReadWrite)
			},
			want: "off",
		},
		{
			run: func(read func(querier operator.DbQuerier) error) error {
				return db.Tx(ctx, func(tx operator.DbTx) error { return read(tx) })
			},
			want: "off",
		},
		{
			run: func(read func(querier operator.DbQuerier) error) error {
				return db.Tx(ctx, func(tx operator.DbTx) error { return read(tx) }, operator.DbReadOnly)
			},
			want: "on",
		},
	} {
		var readOnly string
		err := c.run(func(querier operator.DbQuerier) error {
			var err error
			readOnly, err = dbShow(ctx, querier, "transaction_read_only")
			return err
		})
		if err != nil {
			return err
		}
		if readOnly != c.want {
			return fmt.Errorf("transaction_read_only %q, want %q", readOnly, c.want)
		}
	}
	return nil
}

func dbCheckIsolation(ctx context.Context, db operator.Db) error {
	for _, c := range []struct {
		options []operator.DbOption
		want    string
	}{
		{options: nil, want: "repeatable read"},
		{options: []operator.DbOption{operator.DbIsolationSerializable}, want: "serializable"},
		{options: []operator.DbOption{operator.DbIsolationReadCommitted}, want: "read committed"},
	} {
		var isolation string
		err := db.Tx(ctx, func(tx operator.DbTx) error {
			var err error
			isolation, err = dbShow(ctx, tx, "transaction_isolation")
			return err
		}, c.options...)
		if err != nil {
			return err
		}
		if isolation != c.want {
			return fmt.Errorf("transaction_isolation %q, want %q", isolation, c.want)
		}
	}
	return nil
}

func dbCheckReadBeforeBegin(ctx context.Context, db operator.Db) error {
	steps := []string{}
	readBeforeBegin := operator.DbReadBeforeBegin(func(ctx context.Context, querier operator.DbQuerier) error {
		var one int64
		if err := dbQueryOne(ctx, querier, []any{&one}, `SELECT 1::bigint`); err != nil {
			return err
		}
		steps = append(steps, "read")
		return nil
	})
	err := db.Tx(ctx, func(tx operator.DbTx) error {
		steps = append(steps, "callback")
		return nil
	}, readBeforeBegin)
	if err != nil {
		return err
	}
	if !slices.Equal(steps, []string{"read", "callback"}) {
		return fmt.Errorf("steps %v", steps)
	}
	return nil
}

func dbCheckCallbackError(ctx context.Context, db operator.Db) error {
	refused := errors.New("refused by the callback")
	if err := db.Db(ctx, func(conn operator.DbConn) error { return refused }); !errors.Is(err, refused) {
		return fmt.Errorf("Db answered %v", err)
	}
	if err := db.Tx(ctx, func(tx operator.DbTx) error { return refused }); !errors.Is(err, refused) {
		return fmt.Errorf("Tx answered %v", err)
	}
	return nil
}

func dbCheckRerun(ctx context.Context, db operator.Db) error {
	attemptCount := 0
	err := db.Tx(ctx, func(tx operator.DbTx) error {
		attemptCount += 1
		if attemptCount == 1 {
			return operator.ErrDbRerun
		}
		return nil
	})
	if err != nil {
		return err
	}
	if attemptCount != 2 {
		return fmt.Errorf("%d attempts, want 2", attemptCount)
	}
	return nil
}

func dbCheckNoRetry(ctx context.Context, db operator.Db) error {
	attemptCount := 0
	err := db.Tx(ctx, func(tx operator.DbTx) error {
		attemptCount += 1
		return operator.ErrDbRerun
	}, operator.DbRetryNone)
	if !errors.Is(err, operator.ErrDbRerun) {
		return fmt.Errorf("answered %v, want ErrDbRerun", err)
	}
	if attemptCount != 1 {
		return fmt.Errorf("%d attempts, want 1", attemptCount)
	}
	return nil
}
