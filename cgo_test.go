// Copyright 2017 The Sqlite Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//+build cgo,cgobench

package sqlite // import "modernc.org/sqlite"

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"modernc.org/libc"
)

const cgoDriver = "sqlite3"
const nativeC = "c"
const nativeGO = "go"

func prepareDatabase() string {
	//if this fails you should probably clean your folders
	for i := 0; ; i++ {
		path := fmt.Sprintf("%dbench.db", i)
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return path
		}
	}
}

var drivers = []string{
	driverName,
	cgoDriver,
}

var inMemory = []bool{
	true,
	false,
}

func makename(inMemory bool, driver string, e int) string {
	name := driver
	if inMemory {
		name += "InMemory"
	} else {
		name += "OnDisk"
	}
	return fmt.Sprintf("%s1e%d", name, e)
}

func benchmarkRead(b *testing.B, drivername, file string, n int) {
	os.Remove(file)
	db, err := sql.Open(drivername, file)
	if err != nil {
		b.Fatal(err)
	}

	defer func() {
		db.Close()
		if err := libc.MemAuditReport(); err != nil {
			b.Error(err)
		}
	}()

	if _, err := db.Exec(`
	create table t(i int);
	begin;
	`); err != nil {
		b.Fatal(err)
	}

	s, err := db.Prepare("insert into t values(?)")
	if err != nil {
		b.Fatal(err)
	}

	defer s.Close()

	for i := 0; i < n; i++ {
		if _, err := s.Exec(int64(i)); err != nil {
			b.Fatal(err)
		}
	}
	if _, err := db.Exec("commit"); err != nil {
		b.Fatal(err)
	}

	dst := 0
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		r, err := db.Query("select * from t")
		if err != nil {
			b.Fatal(err)
		}

		b.StartTimer()
		for i := 0; i < n; i++ {
			if !r.Next() {
				b.Fatal(r.Err())
			}

			err = r.Scan(&dst)
			if err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
		r.Close()
	}
	b.StopTimer()
	if *oRecsPerSec {
		b.SetBytes(1e6 * int64(n))
	}
}

func BenchmarkReading1(b *testing.B) {
	dir := b.TempDir()
	for _, memory := range inMemory {
		filename := "file::memory:"
		if !memory {
			filename = filepath.Join(dir, "test.db")
		}
		for _, driver := range drivers {
			for i, n := range []int{1e1, 1e2, 1e3, 1e4, 1e5, 1e6} {
				b.Run(makename(memory, driver, i+1), func(b *testing.B) {
					benchmarkRead(b, driver, filename, n)
					if !memory {
						err := os.Remove(filename)
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}

func BenchmarkReading1Native(b *testing.B) {
	dir := b.TempDir()
	for _, memory := range inMemory {
		filename := ":memory::"
		if !memory {
			filename = filepath.Join(dir, "test.db")
		}
		for i, n := range []int{1e1, 1e2, 1e3, 1e4, 1e5, 1e6} {
			b.Run(makename(memory, nativeGO, i+1), func(b *testing.B) {
				benchmarkReadNativeGO(b, filename, n)
				if !memory {
					err := os.Remove(filename)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
		for i, n := range []int{1e1, 1e2, 1e3, 1e4, 1e5, 1e6} {
			b.Run(makename(memory, nativeC, i+1), func(b *testing.B) {
				benchmarkReadNativeC(b, filename, n)
				if !memory {
					err := os.Remove(filename)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func benchmarkInsertComparative(b *testing.B, drivername, file string, n int) {
	libc.MemAuditStart()
	os.Remove(file)

	db, err := sql.Open(drivername, file)
	if err != nil {
		b.Fatal(err)
	}

	defer func() {
		db.Close()
		if err := libc.MemAuditReport(); err != nil {
			b.Error(err)
		}
	}()

	if _, err := db.Exec(`
	create table t(i int);
	`); err != nil {
		b.Fatal(err)
	}

	s, err := db.Prepare("insert into t values(?)")
	if err != nil {
		b.Fatal(err)
	}

	defer s.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		if _, err := db.Exec("begin"); err != nil {
			b.Fatal(err)
		}
		if _, err := db.Exec("delete from t"); err != nil {
			b.Fatal(err)
		}

		b.StartTimer()
		for i := 0; i < n; i++ {
			if _, err := s.Exec(int64(i)); err != nil {
				b.Fatal(err)
			}
		}
		if _, err := db.Exec("commit"); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
	}
	b.StopTimer()
	if *oRecsPerSec {
		b.SetBytes(1e6 * int64(n))
	}
}

// https://gitlab.com/cznic/sqlite/-/issues/39
func BenchmarkInsertComparative(b *testing.B) {
	dir := b.TempDir()
	for _, memory := range inMemory {
		filename := ":memory:"
		if !memory {
			filename = filepath.Join(dir, "test.db")
		}
		for _, driver := range drivers {
			for i, n := range []int{1e1, 1e2, 1e3, 1e4, 1e5, 1e6} {
				b.Run(makename(memory, driver, i+1), func(b *testing.B) {
					benchmarkInsertComparative(b, driver, filename, n)
					if !memory {
						err := os.Remove(filename)
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}

func BenchmarkInsertComparativeNative(b *testing.B) {
	dir := b.TempDir()
	for _, memory := range inMemory {
		filename := ":memory:"
		if !memory {
			filename = filepath.Join(dir, "test.db")
		}
		for i, n := range []int{1e1, 1e2, 1e3, 1e4, 1e5, 1e6} {
			b.Run(makename(memory, nativeGO, i+1), func(b *testing.B) {
				benchmarkInsertComparativeNativeGO(b, filename, n)
				if !memory {
					err := os.Remove(filename)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
		for i, n := range []int{1e1, 1e2, 1e3, 1e4, 1e5, 1e6} {
			b.Run(makename(memory, nativeC, i+1), func(b *testing.B) {
				benchmarkInsertComparativeNativeC(b, filename, n)
				if !memory {
					err := os.Remove(filename)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
