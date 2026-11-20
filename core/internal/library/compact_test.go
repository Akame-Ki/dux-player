package library

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func fill(t *testing.T, db *sql.DB, n int) {
	t.Helper()
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS junk (id INTEGER PRIMARY KEY, v TEXT)")
	require.NoError(t, err)
	for i := 0; i < n; i++ {
		_, err = db.Exec("INSERT INTO junk (v) VALUES (hex(randomblob(2000)))")
		require.NoError(t, err)
	}
}

func pragma(t *testing.T, db *sql.DB, name string) int {
	t.Helper()
	var v int
	require.NoError(t, db.QueryRow("PRAGMA "+name).Scan(&v))
	return v
}

func TestCompactReturnsDeletedSpaceToDisk(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "x.db")
	db, err := OpenDB(ctx, path, []string{"CREATE TABLE t (a)"})
	require.NoError(t, err)
	defer db.Close()
	require.Equal(t, 1, pragma(t, db, "synchronous"))
	require.NoError(t, Compact(ctx, db))
	require.Equal(t, 2, pragma(t, db, "auto_vacuum"))

	fill(t, db, 500)
	_, err = db.Exec("DELETE FROM junk")
	require.NoError(t, err)
	require.Greater(t, pragma(t, db, "freelist_count"), 100)
	before, _ := os.Stat(path)

	require.NoError(t, Compact(ctx, db))
	after, _ := os.Stat(path)
	require.Equal(t, 0, pragma(t, db, "freelist_count"))
	require.Less(t, after.Size(), before.Size()/10)
	wal, err := os.Stat(path + "-wal")
	require.NoError(t, err)
	require.Zero(t, wal.Size())
}

func TestOpenSwitchesAnOldDatabaseToIncrementalVacuum(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	// An old build's database: no auto_vacuum, deleted pages still in the file.
	old, err := sql.Open("sqlite", "file:"+path)
	require.NoError(t, err)
	_, err = old.Exec("CREATE TABLE keep (a); INSERT INTO keep VALUES ('x'); PRAGMA user_version = 1")
	require.NoError(t, err)
	fill(t, old, 300)
	_, err = old.Exec("DELETE FROM junk")
	require.NoError(t, err)
	require.Equal(t, 0, pragma(t, old, "auto_vacuum"))
	require.NoError(t, old.Close())
	before, _ := os.Stat(path)

	db, err := OpenDB(ctx, path, []string{"CREATE TABLE keep (a)"})
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, Compact(ctx, db))

	require.Equal(t, 2, pragma(t, db, "auto_vacuum"))
	after, _ := os.Stat(path)
	require.Less(t, after.Size(), before.Size()/10)
	var a string
	require.NoError(t, db.QueryRow("SELECT a FROM keep").Scan(&a))
	require.Equal(t, "x", a)
}
