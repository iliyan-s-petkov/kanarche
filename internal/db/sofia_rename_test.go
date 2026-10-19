package db_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"kanarche.eu/internal/db/migrations"
	"kanarche.eu/internal/testsupport"
)

// 00020 moves the retired sofiya slugs to sofia with their sensor links, so the
// deploy-time boundaries import does not leave a second overlapping Sofia.
func TestMigration00020RenamesSofiaSlugs(t *testing.T) {
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	if err := goose.UpToContext(ctx, sqlDB, ".", 19); err != nil {
		t.Fatal(err)
	}
	exec := func(q string) {
		t.Helper()
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	poly := `ST_Multi(ST_GeomFromText('POLYGON((0 0,0 1,1 1,0 0))', 4326))::geography`
	exec(`INSERT INTO area (slug, kind, name_bg, name_en, geom) VALUES
	      ('sofiya', 'city', 'София', 'Sofia', ` + poly + `),
	      ('sofiya-grad-oblast', 'oblast', 'София-град', 'Sofia City', ` + poly + `),
	      ('varna', 'city', 'Варна', 'Varna', ` + poly + `)`)
	exec(`INSERT INTO sensor (sensor_id, sensor_type, location) VALUES (1, 'x', 'POINT(0.2 0.5)'), (2, 'x', 'POINT(0.2 0.5)')`)
	exec(`INSERT INTO area_sensor VALUES ('sofiya', 1), ('sofiya-grad-oblast', 1), ('varna', 2)`)

	if err := goose.UpToContext(ctx, sqlDB, ".", 20); err != nil {
		t.Fatalf("Up: %v", err)
	}
	var slugs string
	if err := pool.QueryRow(ctx, `SELECT string_agg(slug, ',' ORDER BY slug) FROM area`).Scan(&slugs); err != nil {
		t.Fatal(err)
	}
	if want := "sofia,sofia-grad-oblast,varna"; slugs != want {
		t.Errorf("area slugs = %q, want %q", slugs, want)
	}
	var links string
	if err := pool.QueryRow(ctx, `SELECT string_agg(area_slug || ':' || sensor_id, ',' ORDER BY area_slug) FROM area_sensor`).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if want := "sofia:1,sofia-grad-oblast:1,varna:2"; links != want {
		t.Errorf("area_sensor = %q, want %q", links, want)
	}
}
