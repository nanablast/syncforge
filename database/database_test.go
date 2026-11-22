package database

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
)

func strPtr(s string) *string {
	return &s
}

func TestBuildDSNSpecialChars(t *testing.T) {
	specialPass := "p@ss/w:d?1#2%3= &"

	t.Run("MySQL", func(t *testing.T) {
		driver, dsn, err := buildDSN(ConnectionConfig{
			Type:     MySQL,
			Host:     "db.internal",
			Port:     3307,
			User:     "admin@prod",
			Password: specialPass,
			Database: "app_db",
		})
		if err != nil || driver != "mysql" {
			t.Fatalf("unexpected buildDSN result: driver=%q err=%v", driver, err)
		}
		parsed, err := mysqldrv.ParseDSN(dsn)
		if err != nil {
			t.Fatalf("ParseDSN failed: %v (dsn=%q)", err, dsn)
		}
		if parsed.User != "admin@prod" || parsed.Passwd != specialPass || parsed.DBName != "app_db" {
			t.Fatalf("parsed MySQL DSN mismatch: user=%q pass=%q db=%q", parsed.User, parsed.Passwd, parsed.DBName)
		}
	})

	t.Run("PostgreSQL", func(t *testing.T) {
		driver, dsn, err := buildDSN(ConnectionConfig{
			Type:     PostgreSQL,
			Host:     "pg.internal",
			Port:     5432,
			User:     "pguser",
			Password: specialPass,
			Database: "main",
		})
		if err != nil || driver != "postgres" {
			t.Fatalf("unexpected buildDSN result: driver=%q err=%v", driver, err)
		}
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatalf("url.Parse failed: %v", err)
		}
		pw, ok := u.User.Password()
		if !ok || u.User.Username() != "pguser" || pw != specialPass {
			t.Fatalf("parsed PG credentials mismatch: user=%q ok=%v", u.User.Username(), ok)
		}
		if got := u.Query().Get("sslmode"); got != "prefer" {
			t.Fatalf("expected sslmode=prefer, got %q", got)
		}
	})

	t.Run("SQLServer", func(t *testing.T) {
		driver, dsn, err := buildDSN(ConnectionConfig{
			Type:     SQLServer,
			Host:     "mssql.internal",
			Port:     1433,
			User:     "sa",
			Password: specialPass,
			Database: "reporting",
		})
		if err != nil || driver != "sqlserver" {
			t.Fatalf("unexpected buildDSN result: driver=%q err=%v", driver, err)
		}
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatalf("url.Parse failed: %v", err)
		}
		pw, _ := u.User.Password()
		if pw != specialPass || u.Query().Get("database") != "reporting" {
			t.Fatalf("parsed SQLServer DSN mismatch: db=%q", u.Query().Get("database"))
		}
	})

	t.Run("SQLiteMissingPath", func(t *testing.T) {
		if _, _, err := buildDSN(ConnectionConfig{Type: SQLite}); err == nil {
			t.Fatal("expected error for SQLite without FilePath")
		}
	})
}

func TestCompareSchemasWithDialect(t *testing.T) {
	source := &SchemaInfo{
		Database: "src",
		Tables: map[string]TableInfo{
			"new_tbl": {
				Name:      "new_tbl",
				CreateSQL: "CREATE TABLE \"new_tbl\" (\n  \"id\" int\n);",
			},
			"users": {
				Name: "users",
				Columns: []ColumnInfo{
					{Name: "id", Type: "int", Nullable: "NO", Position: 1},
					{Name: "email", Type: "varchar(255)", Nullable: "NO", Default: strPtr("none'"), Position: 2},
					{Name: "age", Type: "bigint", Nullable: "NO", Default: strPtr("18"), Position: 3},
				},
				Indexes: []IndexInfo{
					{Name: "idx_uniq_email_age", NonUnique: 0, Column: "age", SeqInIdx: 2},
					{Name: "idx_uniq_email_age", NonUnique: 0, Column: "email", SeqInIdx: 1},
				},
			},
		},
	}

	target := &SchemaInfo{
		Database: "dst",
		Tables: map[string]TableInfo{
			"old_tbl": {
				Name: "old_tbl",
			},
			"users": {
				Name: "users",
				Columns: []ColumnInfo{
					{Name: "id", Type: "int", Nullable: "NO", Position: 1},
					{Name: "age", Type: "int", Nullable: "YES", Position: 2},
					{Name: "legacy", Type: "text", Nullable: "YES", Position: 3},
				},
			},
		},
	}

	for _, dialect := range []DBType{MySQL, PostgreSQL, SQLite, SQLServer} {
		t.Run(string(dialect), func(t *testing.T) {
			diffs := CompareSchemasWithDialect(source, target, dialect)
			if len(diffs) == 0 {
				t.Fatal("expected non-empty diffs")
			}

			var sawNewTable, sawDropTable, sawAddCol, sawAddIdx, sawModCol bool
			for _, d := range diffs {
				if strings.Contains(d.SQL, ";;") {
					t.Fatalf("double semicolon generated for %s: %s", dialect, d.SQL)
				}
				switch {
				case d.Type == "added" && d.TableName == "new_tbl":
					sawNewTable = true
				case d.Type == "removed" && d.TableName == "old_tbl":
					sawDropTable = true
					switch dialect {
					case PostgreSQL, SQLite:
						if d.SQL != `DROP TABLE "old_tbl";` {
							t.Fatalf("unexpected drop SQL: %s", d.SQL)
						}
					case SQLServer:
						if d.SQL != `DROP TABLE [old_tbl];` {
							t.Fatalf("unexpected drop SQL: %s", d.SQL)
						}
					}
				case strings.HasPrefix(d.Detail, "Add column: email"):
					sawAddCol = true
					if dialect != MySQL && (strings.Contains(d.SQL, "AFTER") || strings.Contains(d.SQL, "`")) {
						t.Fatalf("non-MySQL dialect leaked MySQL syntax: %s", d.SQL)
					}
					if dialect == MySQL && !strings.Contains(d.SQL, "AFTER `id`") {
						t.Fatalf("expected MySQL AFTER `id`, got: %s", d.SQL)
					}
				case strings.HasPrefix(d.Detail, "Modify column: age"):
					sawModCol = true
					if dialect == SQLite && !strings.HasPrefix(d.SQL, "--") {
						t.Fatalf("expected SQLite comment guidance for ALTER COLUMN, got: %s", d.SQL)
					}
					if dialect == PostgreSQL && !strings.Contains(d.SQL, `ALTER COLUMN "age" TYPE bigint`) {
						t.Fatalf("expected PG ALTER COLUMN TYPE, got: %s", d.SQL)
					}
				case strings.HasPrefix(d.Detail, "Add index: idx_uniq_email_age"):
					sawAddIdx = true
					if !strings.Contains(d.SQL, "UNIQUE INDEX") {
						t.Fatalf("expected UNIQUE INDEX in %s", d.SQL)
					}
					// Ordered by SeqInIdx: email before age
					if strings.Index(d.SQL, "email") > strings.Index(d.SQL, "age") {
						t.Fatalf("composite index column order wrong: %s", d.SQL)
					}
				}
			}

			if !sawNewTable || !sawDropTable || !sawAddCol || !sawModCol || !sawAddIdx {
				t.Fatalf("missing expected diff kinds: new=%v drop=%v addCol=%v modCol=%v addIdx=%v",
					sawNewTable, sawDropTable, sawAddCol, sawModCol, sawAddIdx)
			}
		})
	}

	if isNumericDefault("-") || isNumericDefault(".") || isNumericDefault("1.2.3") {
		t.Fatal("invalid numeric strings should not pass isNumericDefault")
	}
	if !isNumericDefault("-42.5") || !isNumericDefault("0") {
		t.Fatal("valid numeric defaults rejected")
	}
}

func TestEscapeValueAndRowComparison(t *testing.T) {
	tm := time.Date(2026, 11, 22, 22, 30, 5, 0, time.FixedZone("CST", 8*3600))
	if got := escapeValueForDialect(MySQL, tm); got != "'2026-11-22 14:30:05'" {
		t.Fatalf("unexpected time SQL: %s", got)
	}
	if got := escapeValueForDialect(MySQL, `C:\tmp\'a`); got != `'C:\\tmp\\''a'` {
		t.Fatalf("unexpected MySQL backslash escape: %s", got)
	}
	if got := escapeValueForDialect(PostgreSQL, true); got != "TRUE" {
		t.Fatalf("unexpected PG bool: %s", got)
	}
	if got := escapeValueForDialect(MySQL, true); got != "1" {
		t.Fatalf("unexpected MySQL bool: %s", got)
	}
	if got := escapeValueForDialect(SQLite, []byte{0xde, 0xad, 0xbe, 0xef}); got != "X'deadbeef'" {
		t.Fatalf("unexpected SQLite blob hex: %s", got)
	}
	if got := escapeValueForDialect(PostgreSQL, []byte{0xde, 0xad}); got != `E'\\xdead'` {
		t.Fatalf("unexpected PG blob hex: %s", got)
	}
	if got := escapeValueForDialect(SQLServer, []byte{0xde, 0xad}); got != "0xdead" {
		t.Fatalf("unexpected SQLServer blob hex: %s", got)
	}

	// nil vs "<nil>" must never be equal
	if rowsEqual(map[string]interface{}{"a": nil}, map[string]interface{}{"a": "<nil>"}) {
		t.Fatal("nil and literal \"<nil>\" should not be treated as equal")
	}
	if !rowsEqual(map[string]interface{}{"b": []byte{1, 2}}, map[string]interface{}{"b": []byte{1, 2}}) {
		t.Fatal("identical byte slices should be equal")
	}
	if rowsEqual(map[string]interface{}{"b": []byte{1, 2}}, map[string]interface{}{"b": []byte{1, 3}}) {
		t.Fatal("different byte slices should not be equal")
	}

	// PK values containing '|' must never collide
	k1 := buildPrimaryKeyKey(map[string]interface{}{"k1": "a|b", "k2": "c"}, []string{"k1", "k2"})
	k2 := buildPrimaryKeyKey(map[string]interface{}{"k1": "a", "k2": "b|c"}, []string{"k1", "k2"})
	if k1 == k2 {
		t.Fatalf("PK key collision detected: %q == %q", k1, k2)
	}

	// PK-only table must return empty UPDATE SQL rather than malformed UPDATE SET WHERE
	if sql := generateUpdateSQL(SQLite, "join_tbl", map[string]interface{}{"a": 1, "b": 2}, []string{"a", "b"}, []string{"a", "b"}); sql != "" {
		t.Fatalf("expected empty UPDATE SQL for PK-only table, got %q", sql)
	}
}

func TestSQLiteEndToEndSchemaAndDataSync(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.db")
	dstPath := filepath.Join(dir, "target.db")

	srcDB, err := sql.Open("sqlite3", srcPath)
	if err != nil {
		t.Fatalf("open source.db: %v", err)
	}
	defer srcDB.Close()

	dstDB, err := sql.Open("sqlite3", dstPath)
	if err != nil {
		t.Fatalf("open target.db: %v", err)
	}
	defer dstDB.Close()

	_, err = srcDB.Exec(`
		CREATE TABLE items (
			org_id TEXT NOT NULL,
			item_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			payload BLOB,
			PRIMARY KEY (org_id, item_id)
		);
		CREATE UNIQUE INDEX idx_items_name ON items (name, org_id);
		CREATE TABLE extra_tbl (id INTEGER PRIMARY KEY, label TEXT);
		CREATE TABLE empty_tbl (id INTEGER PRIMARY KEY, note TEXT);
		CREATE VIEW v_items AS SELECT org_id, item_id, name FROM items;

		INSERT INTO items (org_id, item_id, name, payload) VALUES ('a|b', 1, 'it''s;one', X'deadbeef');
		INSERT INTO items (org_id, item_id, name, payload) VALUES ('a', 2, 'only-in-source', X'cafebabe');
	`)
	if err != nil {
		t.Fatalf("seed source.db: %v", err)
	}

	_, err = dstDB.Exec(`
		CREATE TABLE items (
			org_id TEXT NOT NULL,
			item_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			payload BLOB,
			PRIMARY KEY (org_id, item_id)
		);
		CREATE INDEX idx_items_name ON items (org_id, name);
		CREATE VIEW v_broken AS SELECT * FROM missing_table;

		INSERT INTO items (org_id, item_id, name, payload) VALUES ('a|b', 1, 'stale-name', X'0102');
		INSERT INTO items (org_id, item_id, name, payload) VALUES ('z', 99, 'only-in-target', X'0304');
	`)
	if err != nil {
		t.Fatalf("seed target.db: %v", err)
	}

	srcCfg := ConnectionConfig{Type: SQLite, FilePath: srcPath, Database: "main"}
	dstCfg := ConnectionConfig{Type: SQLite, FilePath: dstPath, Database: "main"}

	srcSchema, err := GetSchema(srcCfg)
	if err != nil {
		t.Fatalf("GetSchema(source) failed: %v", err)
	}
	dstSchema, err := GetSchema(dstCfg)
	if err != nil {
		t.Fatalf("GetSchema(target) failed (broken view should be skipped): %v", err)
	}

	if _, hasView := srcSchema.Tables["v_items"]; hasView {
		t.Fatal("view v_items should not be included in schema tables")
	}

	schemaDiffs := CompareSchemasWithDialect(srcSchema, dstSchema, SQLite)
	var sawRecreateIdx, sawExtraTbl bool
	for _, d := range schemaDiffs {
		if d.TableName == "extra_tbl" && d.Type == "added" {
			sawExtraTbl = true
		}
		if d.TableName == "items" && d.Detail == "Recreate index: idx_items_name" {
			sawRecreateIdx = true
		}
	}
	if !sawExtraTbl || !sawRecreateIdx {
		t.Fatalf("unexpected schema diffs: %+v", schemaDiffs)
	}

	tables, err := GetTablesForSync(srcCfg)
	if err != nil {
		t.Fatalf("GetTablesForSync failed: %v", err)
	}
	var itemsInfo *TableDataInfo
	for i := range tables {
		if tables[i].TableName == "items" {
			itemsInfo = &tables[i]
		}
		if tables[i].TableName == "v_items" {
			t.Fatal("view returned in GetTablesForSync")
		}
	}
	if itemsInfo == nil || len(itemsInfo.PrimaryKeys) != 2 || itemsInfo.PrimaryKeys[0] != "org_id" || itemsInfo.PrimaryKeys[1] != "item_id" {
		t.Fatalf("unexpected composite PK info: %+v", itemsInfo)
	}

	// Browser pagination & BLOB formatting + empty table handling
	page, err := GetTableData(srcCfg, "items", 1, 10)
	if err != nil {
		t.Fatalf("GetTableData(items) failed: %v", err)
	}
	if page.TotalCount != 2 || len(page.Rows) != 2 {
		t.Fatalf("unexpected page data: total=%d rows=%d", page.TotalCount, len(page.Rows))
	}
	if got := page.Rows[0].Values["payload"]; got != "<BLOB 4 bytes>" && page.Rows[1].Values["payload"] != "<BLOB 4 bytes>" {
		t.Fatalf("expected binary BLOB display placeholder, got: %+v", page.Rows)
	}

	emptyPage, err := GetTableData(srcCfg, "empty_tbl", 0, 0)
	if err != nil || emptyPage.Page != 1 || emptyPage.PageSize != 50 || emptyPage.TotalCount != 0 {
		t.Fatalf("unexpected empty table result: %+v err=%v", emptyPage, err)
	}

	// Data comparison & SQL replay convergence
	dataDiffs, err := CompareTableData(srcCfg, dstCfg, "items")
	if err != nil {
		t.Fatalf("CompareTableData failed: %v", err)
	}
	if len(dataDiffs) != 3 {
		t.Fatalf("expected 3 diffs (1 insert, 1 update, 1 delete), got %d: %+v", len(dataDiffs), dataDiffs)
	}

	for _, d := range dataDiffs {
		if _, err := dstDB.Exec(d.SQL); err != nil {
			t.Fatalf("executing generated sync SQL %q failed: %v", d.SQL, err)
		}
	}

	afterDiffs, err := CompareTableData(srcCfg, dstCfg, "items")
	if err != nil {
		t.Fatalf("CompareTableData after sync failed: %v", err)
	}
	if len(afterDiffs) != 0 {
		t.Fatalf("expected 0 diffs after applying sync SQL, got %+v", afterDiffs)
	}
}
