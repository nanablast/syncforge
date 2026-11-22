package main

import (
	"path/filepath"
	"testing"

	"syncforge/database"
)

func TestSplitSQLStatements(t *testing.T) {
	input := `
-- lead comment; should be ignored
INSERT INTO notes (body) VALUES ('hello; ''world'';');
/* multi-line
   comment; with semicolon; */
INSERT INTO notes (body) VALUES ('backslash quote \' and ; semicolon');
CREATE TABLE "t;1" (` + "`c;2`" + ` TEXT);
INSERT INTO notes (body) VALUES ('中文;测试;数据');
   ;
`
	stmts := splitSQLStatements(input)
	if len(stmts) != 4 {
		t.Fatalf("expected 4 statements, got %d: %#v", len(stmts), stmts)
	}
	if stmts[0] != "INSERT INTO notes (body) VALUES ('hello; ''world'';')" {
		t.Fatalf("unexpected stmt[0]: %q", stmts[0])
	}
	if stmts[1] != `INSERT INTO notes (body) VALUES ('backslash quote \' and ; semicolon')` {
		t.Fatalf("unexpected stmt[1]: %q", stmts[1])
	}
	if stmts[3] != "INSERT INTO notes (body) VALUES ('中文;测试;数据')" {
		t.Fatalf("unexpected stmt[3]: %q", stmts[3])
	}
}

func TestExecuteSQLTransactionRollback(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tx.db")
	cfg := database.ConnectionConfig{
		Type:     database.SQLite,
		FilePath: dbPath,
		Database: "main",
	}

	app := NewApp()
	if err := app.ExecuteSQL(cfg, "CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL);"); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	// Second statement fails PK constraint -> first statement must roll back atomically
	err := app.ExecuteSQL(cfg, `
		INSERT INTO items (id, name) VALUES (1, 'first; ''quoted''');
		INSERT INTO items (id, name) VALUES (1, 'duplicate');
	`)
	if err == nil {
		t.Fatalf("expected error on duplicate primary key, got nil")
	}

	data, err := app.GetTableData(cfg, "items", 1, 50)
	if err != nil {
		t.Fatalf("GetTableData failed: %v", err)
	}
	if data.TotalCount != 0 {
		t.Fatalf("expected rollback to leave 0 rows, got %d", data.TotalCount)
	}

	// Valid multi-statement script commits cleanly
	err = app.ExecuteSQL(cfg, `
		INSERT INTO items (id, name) VALUES (1, 'alpha;''beta''');
		INSERT INTO items (id, name) VALUES (2, '中文内容');
	`)
	if err != nil {
		t.Fatalf("valid ExecuteSQL failed: %v", err)
	}

	data, err = app.GetTableData(cfg, "items", 1, 50)
	if err != nil {
		t.Fatalf("GetTableData failed: %v", err)
	}
	if data.TotalCount != 2 {
		t.Fatalf("expected 2 rows after commit, got %d", data.TotalCount)
	}
}
