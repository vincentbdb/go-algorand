// Copyright (C) 2019 Algorand, Inc.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package db

import (
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/vincentbdb/go-algorand/crypto"
)

func TestInMemoryDisposal(t *testing.T) {
	acc, err := MakeAccessor("fn.db", false, true)
	require.NoError(t, err)
	err = acc.Atomic(func(tx *sql.Tx) error {
		_, err := tx.Exec("create table Service (data blob)")
		return err
	})
	require.NoError(t, err)

	err = acc.Atomic(func(tx *sql.Tx) error {
		raw := []byte{0, 1, 2}
		_, err := tx.Exec("insert or replace into Service (rowid, data) values (1, ?)", raw)
		return err
	})
	require.NoError(t, err)

	anotherAcc, err := MakeAccessor("fn.db", false, true)
	require.NoError(t, err)
	err = anotherAcc.Atomic(func(tx *sql.Tx) error {
		var nrows int
		row := tx.QueryRow("select count(*) from Service")
		err := row.Scan(&nrows)
		return err
	})
	require.NoError(t, err)
	anotherAcc.Close()

	acc.Close()

	acc, err = MakeAccessor("fn.db", false, true)
	require.NoError(t, err)
	err = acc.Atomic(func(tx *sql.Tx) error {
		var nrows int
		row := tx.QueryRow("select count(*) from Service")
		err := row.Scan(&nrows)
		if err == nil {
			return errors.New("table `Service` presents while it should not")
		}
		return nil
	})
	require.NoError(t, err)

	acc.Close()
}

func TestInMemoryUniqueDB(t *testing.T) {
	acc, err := MakeAccessor("fn.db", false, true)
	require.NoError(t, err)
	defer acc.Close()
	err = acc.Atomic(func(tx *sql.Tx) error {
		_, err := tx.Exec("create table Service (data blob)")
		return err
	})
	require.NoError(t, err)

	err = acc.Atomic(func(tx *sql.Tx) error {
		raw := []byte{0, 1, 2}
		_, err := tx.Exec("insert or replace into Service (rowid, data) values (1, ?)", raw)
		return err
	})
	require.NoError(t, err)

	anotherAcc, err := MakeAccessor("fn2.db", false, true)
	require.NoError(t, err)
	defer anotherAcc.Close()
	err = anotherAcc.Atomic(func(tx *sql.Tx) error {
		var nrows int
		row := tx.QueryRow("select count(*) from Service")
		err := row.Scan(&nrows)
		if err == nil {
			return errors.New("table `Service` presents while it should not")
		}
		return nil
	})
	require.NoError(t, err)
}

func TestDBConcurrency(t *testing.T) {
	fn := fmt.Sprintf("/tmp/%s.%d.sqlite3", t.Name(), crypto.RandUint64())
	defer cleanupSqliteDb(t, fn)

	acc, err := MakeAccessor(fn, false, false)
	require.NoError(t, err)
	defer acc.Close()

	acc2, err := MakeAccessor(fn, true, false)
	require.NoError(t, err)
	defer acc2.Close()

	err = acc.Atomic(func(tx *sql.Tx) error {
		_, err := tx.Exec("CREATE TABLE foo (a INTEGER, b INTEGER)")
		return err
	})
	require.NoError(t, err)

	err = acc.Atomic(func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO foo (a, b) VALUES (?, ?)", 1, 1)
		return err
	})
	require.NoError(t, err)

	c1 := make(chan struct{})
	c2 := make(chan struct{})
	go func() {
		err := acc.Atomic(func(tx *sql.Tx) error {
			<-c2

			_, err := tx.Exec("INSERT INTO foo (a, b) VALUES (?, ?)", 2, 2)
			if err != nil {
				return err
			}

			c1 <- struct{}{}
			<-c2

			_, err = tx.Exec("INSERT INTO foo (a, b) VALUES (?, ?)", 3, 3)
			if err != nil {
				return err
			}

			return nil
		})

		require.NoError(t, err)
		c1 <- struct{}{}
	}()

	err = acc2.Atomic(func(tx *sql.Tx) error {
		var nrows int64
		err := tx.QueryRow("SELECT COUNT(*) FROM foo").Scan(&nrows)
		if err != nil {
			return err
		}

		if nrows != 1 {
			return fmt.Errorf("row count mismatch: %d != 1", nrows)
		}

		return nil
	})
	require.NoError(t, err)

	c2 <- struct{}{}
	<-c1

	err = acc2.Atomic(func(tx *sql.Tx) error {
		var nrows int64
		err := tx.QueryRow("SELECT COUNT(*) FROM foo").Scan(&nrows)
		if err != nil {
			return err
		}

		if nrows != 1 {
			return fmt.Errorf("row count mismatch: %d != 1", nrows)
		}

		return nil
	})
	require.NoError(t, err)

	c2 <- struct{}{}
	<-c1

	err = acc2.Atomic(func(tx *sql.Tx) error {
		var nrows int64
		err := tx.QueryRow("SELECT COUNT(*) FROM foo").Scan(&nrows)
		if err != nil {
			return err
		}

		if nrows != 3 {
			return fmt.Errorf("row count mismatch: %d != 3", nrows)
		}

		return nil
	})
	require.NoError(t, err)
}

func cleanupSqliteDb(t *testing.T, path string) {
	parts, err := filepath.Glob(path + "*")
	if err != nil {
		t.Errorf("%s*: could not glob, %s", path, err)
		return
	}
	for _, part := range parts {
		err = os.Remove(part)
		if err != nil {
			t.Errorf("%s: error cleaning up, %s", part, err)
		}
	}
}

func TestDBConcurrencyRW(t *testing.T) {
	dbFolder := "/dev/shm"
	os := runtime.GOOS
	if os == "darwin" {
		var err error
		dbFolder, err = ioutil.TempDir("", "TestDBConcurrencyRW")
		if err != nil {
			panic(err)
		}
	}

	fn := fmt.Sprintf("/%s.%d.sqlite3", t.Name(), crypto.RandUint64())
	fn = filepath.Join(dbFolder, fn)
	defer cleanupSqliteDb(t, fn)

	acc, err := MakeAccessor(fn, false, false)
	require.NoError(t, err)
	defer acc.Close()

	acc2, err := MakeAccessor(fn, true, false)
	require.NoError(t, err)
	defer acc2.Close()

	err = acc.Atomic(func(tx *sql.Tx) error {
		_, err := tx.Exec("CREATE TABLE t (a INTEGER PRIMARY KEY)")
		return err
	})
	require.NoError(t, err)

	started := make(chan struct{})
	var lastInsert int64
	testRoutineComplete := make(chan struct{}, 3)
	targetTestDurationTimer := time.After(15 * time.Second)
	go func() {
		defer func() {
			atomic.StoreInt64(&lastInsert, -1)
			testRoutineComplete <- struct{}{}
		}()
		var errw error
		for i, timedLoop := int64(1), true; timedLoop && errw == nil; i++ {
			errw = acc.Atomic(func(tx *sql.Tx) error {
				_, err := tx.Exec("INSERT INTO t (a) VALUES (?)", i)
				return err
			})
			if errw == nil {
				atomic.StoreInt64(&lastInsert, i)
			}
			if i == 1 {
				close(started)
			}
			select {
			case <-targetTestDurationTimer:
				// abort the for loop.
				timedLoop = false
			default:
				// keep going.
			}
		}
		require.NoError(t, errw)
	}()

	for i := 0; i < 2; i++ {
		go func() {
			defer func() {
				testRoutineComplete <- struct{}{}
			}()
			select {
			case <-started:
			case <-time.After(10 * time.Second):
				t.Error("timeout")
				return
			}
			for {
				id := atomic.LoadInt64(&lastInsert)
				if id == 0 {
					// we have yet to complete the first item insertion, yet,
					// the "started" channel is closed. This happen only in case of
					// an error during the insert ( which is reported above)
					break
				} else if id < 0 {
					break
				}
				var x int64
				errsel := acc2.Atomic(func(tx *sql.Tx) error {
					return tx.QueryRow("SELECT a FROM t WHERE a=?", id).Scan(&x)
				})
				if errsel != nil {
					t.Errorf("selecting %d: %v", id, errsel)
				}
				require.Equal(t, x, id)
			}
		}()
	}

	testTimeout := time.After(3 * time.Minute)
	for i := 0; i < 3; i++ {
		select {
		case <-testRoutineComplete:
			// good. keep going.
		case <-testTimeout:
			// the test has timed out. we want to abort now with a failuire since we might be stuck in one of the goroutines above.
			lastID := atomic.LoadInt64(&lastInsert)
			t.Errorf("Test has timed out. Last id is %d.\n", lastID)
		}
	}

}
