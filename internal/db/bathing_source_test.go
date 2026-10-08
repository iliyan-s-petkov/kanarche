package db_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"kanarche.eu/internal/db/migrations"
	"kanarche.eu/internal/testsupport"
)

func columnExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, col string) bool {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`,
		table, col).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

// 00017 on a database that already holds classes: existing rows become
// 'discodata', the CHECK holds, and Down removes the columns and the rows
// that only the new source could have written.
func TestMigration00017BathingSourceUpAndDown(t *testing.T) {
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	if err := goose.UpToContext(ctx, sqlDB, ".", 16); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO bathing_site VALUES ('BG0000000000000001', 'а', 'a', 'coastal', 42.5, 27.6, '')`)
	exec(`INSERT INTO bathing_class VALUES ('BG0000000000000001', 2023, 'good')`)
	exec(`INSERT INTO bathing_import VALUES (now(), 1, 1, 0)`)
	if columnExists(t, ctx, pool, "bathing_class", "source") {
		t.Fatal("source exists before 00017")
	}

	if err := goose.UpToContext(ctx, sqlDB, ".", 17); err != nil {
		t.Fatalf("Up: %v", err)
	}
	var src string
	if err := pool.QueryRow(ctx, `SELECT source FROM bathing_class`).Scan(&src); err != nil || src != "discodata" {
		t.Fatalf("existing row source = %q, %v; want discodata", src, err)
	}
	var ed *string
	if err := pool.QueryRow(ctx, `SELECT supplement_edition FROM bathing_import`).Scan(&ed); err != nil || ed != nil {
		t.Fatalf("existing import edition = %v, %v; want NULL", ed, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO bathing_class VALUES ('BG0000000000000001', 2024, 'good', 'other')`); err == nil {
		t.Error("CHECK accepted source 'other'")
	}
	exec(`INSERT INTO bathing_class VALUES ('BG0000000000000001', 2025, 'excellent', 'datahub')`)

	if err := goose.DownToContext(ctx, sqlDB, ".", 16); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if columnExists(t, ctx, pool, "bathing_class", "source") || columnExists(t, ctx, pool, "bathing_import", "supplement_edition") {
		t.Error("Down left a column behind")
	}
	var seasons []int
	rows, err := pool.Query(ctx, `SELECT season FROM bathing_class ORDER BY season`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var s int
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		seasons = append(seasons, s)
	}
	if len(seasons) != 1 || seasons[0] != 2023 {
		t.Errorf("after Down seasons = %v, want only the Discodata 2023", seasons)
	}
	// Up again: the migration is repeatable.
	if err := goose.UpContext(ctx, sqlDB, "."); err != nil {
		t.Fatalf("Up again: %v", err)
	}
}
