package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	mysqldrv "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// DBType represents the type of database
type DBType string

const (
	MySQL      DBType = "mysql"
	PostgreSQL DBType = "postgresql"
	SQLite     DBType = "sqlite"
	SQLServer  DBType = "sqlserver"
)

// ConnectionConfig holds database connection parameters
type ConnectionConfig struct {
	Type     DBType `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	// SQLite specific
	FilePath string `json:"filePath,omitempty"`
}

// TableInfo holds table structure information
type TableInfo struct {
	Name      string       `json:"name"`
	CreateSQL string       `json:"createSql"`
	Columns   []ColumnInfo `json:"columns"`
	Indexes   []IndexInfo  `json:"indexes"`
}

// ColumnInfo holds column details
type ColumnInfo struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Nullable string  `json:"nullable"`
	Key      string  `json:"key"`
	Default  *string `json:"default"`
	Extra    string  `json:"extra"`
	Position int     `json:"position"`
}

// IndexInfo holds index details
type IndexInfo struct {
	Name      string `json:"name"`
	NonUnique int    `json:"nonUnique"`
	Column    string `json:"column"`
	SeqInIdx  int    `json:"seqInIndex"`
}

// SchemaInfo holds complete database schema
type SchemaInfo struct {
	Database string               `json:"database"`
	Tables   map[string]TableInfo `json:"tables"`
}

// DiffResult holds comparison result
type DiffResult struct {
	Type      string `json:"type"` // "added", "removed", "modified"
	TableName string `json:"tableName"`
	Detail    string `json:"detail"`
	SQL       string `json:"sql"`
}

func mysqlDSN(config ConnectionConfig, includeDB bool) string {
	cfg := mysqldrv.NewConfig()
	cfg.User = config.User
	cfg.Passwd = config.Password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", config.Host, config.Port)
	if includeDB {
		cfg.DBName = config.Database
	}
	cfg.ParseTime = true
	cfg.MultiStatements = true
	cfg.AllowNativePasswords = true
	cfg.Timeout = 10 * time.Second
	return cfg.FormatDSN()
}

func postgresDSN(config ConnectionConfig) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(config.User, config.Password),
		Host:   fmt.Sprintf("%s:%d", config.Host, config.Port),
		Path:   "/" + config.Database,
	}
	q := u.Query()
	q.Set("sslmode", "prefer")
	q.Set("connect_timeout", "10")
	u.RawQuery = q.Encode()
	return u.String()
}

func sqlserverDSN(config ConnectionConfig) string {
	u := &url.URL{
		Scheme: "sqlserver",
		User:   url.UserPassword(config.User, config.Password),
		Host:   fmt.Sprintf("%s:%d", config.Host, config.Port),
	}
	q := u.Query()
	if config.Database != "" {
		q.Set("database", config.Database)
	}
	q.Set("connection timeout", "10")
	u.RawQuery = q.Encode()
	return u.String()
}

// buildDSN builds the connection string for the given database type
func buildDSN(config ConnectionConfig) (string, string, error) {
	switch config.Type {
	case MySQL, "":
		return "mysql", mysqlDSN(config, true), nil

	case PostgreSQL:
		return "postgres", postgresDSN(config), nil

	case SQLite:
		if config.FilePath == "" {
			return "", "", fmt.Errorf("SQLite requires a file path")
		}
		return "sqlite3", config.FilePath, nil

	case SQLServer:
		return "sqlserver", sqlserverDSN(config), nil

	default:
		return "", "", fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

func connectWithDSN(driver, dsn string) (*sql.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// Connect creates a database connection
func Connect(config ConnectionConfig) (*sql.DB, error) {
	driver, dsn, err := buildDSN(config)
	if err != nil {
		return nil, err
	}
	return connectWithDSN(driver, dsn)
}

// TestConnection tests if the connection works
func TestConnection(config ConnectionConfig) error {
	db, err := Connect(config)
	if err != nil {
		return err
	}
	defer db.Close()
	return nil
}

// GetDatabases returns list of databases
func GetDatabases(config ConnectionConfig) ([]string, error) {
	switch config.Type {
	case MySQL, "":
		return getMySQLDatabases(config)
	case PostgreSQL:
		return getPostgreSQLDatabases(config)
	case SQLite:
		return []string{"main"}, nil
	case SQLServer:
		return getSQLServerDatabases(config)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

func getMySQLDatabases(config ConnectionConfig) ([]string, error) {
	db, err := connectWithDSN("mysql", mysqlDSN(config, false))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if name != "information_schema" && name != "mysql" &&
			name != "performance_schema" && name != "sys" {
			databases = append(databases, name)
		}
	}

	return databases, rows.Err()
}

func getPostgreSQLDatabases(config ConnectionConfig) ([]string, error) {
	cfg := config
	cfg.Database = "postgres"

	db, err := Connect(cfg)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT datname FROM pg_database WHERE datistemplate = false AND datname NOT IN ('postgres') ORDER BY datname")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		databases = append(databases, name)
	}

	return databases, rows.Err()
}

func getSQLServerDatabases(config ConnectionConfig) ([]string, error) {
	cfg := config
	cfg.Database = "master"

	db, err := Connect(cfg)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT name FROM sys.databases WHERE name NOT IN ('master', 'tempdb', 'model', 'msdb') ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		databases = append(databases, name)
	}

	return databases, rows.Err()
}

// GetSchema retrieves complete schema information
func GetSchema(config ConnectionConfig) (*SchemaInfo, error) {
	switch config.Type {
	case MySQL, "":
		return getMySQLSchema(config)
	case PostgreSQL:
		return getPostgreSQLSchema(config)
	case SQLite:
		return getSQLiteSchema(config)
	case SQLServer:
		return getSQLServerSchema(config)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

func getMySQLSchema(config ConnectionConfig) (*SchemaInfo, error) {
	db, err := Connect(config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	schema := &SchemaInfo{
		Database: config.Database,
		Tables:   make(map[string]TableInfo),
	}

	rows, err := db.Query("SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, tableName := range tableNames {
		tableInfo, err := getMySQLTableInfo(db, tableName)
		if err != nil {
			return nil, err
		}
		schema.Tables[tableName] = *tableInfo
	}

	return schema, nil
}

func scanMySQLString(val interface{}) string {
	switch v := val.(type) {
	case []byte:
		return string(v)
	case string:
		return v
	default:
		return ""
	}
}

func scanMySQLInt(val interface{}) int {
	switch v := val.(type) {
	case int64:
		return int(v)
	case uint64:
		return int(v)
	case int:
		return v
	case []byte:
		n, _ := strconv.Atoi(string(v))
		return n
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

func getMySQLTableInfo(db *sql.DB, tableName string) (*TableInfo, error) {
	info := &TableInfo{
		Name: tableName,
	}

	quotedTable := quoteIdentifier(MySQL, tableName)

	var tbl, createSQL string
	err := db.QueryRow(fmt.Sprintf("SHOW CREATE TABLE %s", quotedTable)).Scan(&tbl, &createSQL)
	if err != nil {
		return nil, err
	}
	info.CreateSQL = createSQL

	colRows, err := db.Query(`
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY, COLUMN_DEFAULT, EXTRA, ORDINAL_POSITION
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`, tableName)
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	for colRows.Next() {
		var col ColumnInfo
		if err := colRows.Scan(&col.Name, &col.Type, &col.Nullable, &col.Key, &col.Default, &col.Extra, &col.Position); err != nil {
			return nil, err
		}
		info.Columns = append(info.Columns, col)
	}
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	idxRows, err := db.Query(fmt.Sprintf("SHOW INDEX FROM %s", quotedTable))
	if err != nil {
		return nil, err
	}
	defer idxRows.Close()

	cols, err := idxRows.Columns()
	if err != nil {
		return nil, err
	}
	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for idxRows.Next() {
		if err := idxRows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		idx := IndexInfo{}
		for i, col := range cols {
			val := values[i]
			switch col {
			case "Key_name":
				idx.Name = scanMySQLString(val)
			case "Non_unique":
				idx.NonUnique = scanMySQLInt(val)
			case "Column_name":
				idx.Column = scanMySQLString(val)
			case "Seq_in_index":
				idx.SeqInIdx = scanMySQLInt(val)
			}
		}
		info.Indexes = append(info.Indexes, idx)
	}
	if err := idxRows.Err(); err != nil {
		return nil, err
	}

	return info, nil
}

func getPostgreSQLSchema(config ConnectionConfig) (*SchemaInfo, error) {
	db, err := Connect(config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	schema := &SchemaInfo{
		Database: config.Database,
		Tables:   make(map[string]TableInfo),
	}

	rows, err := db.Query(`
		SELECT tablename FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, tableName := range tableNames {
		tableInfo, err := getPostgreSQLTableInfo(db, tableName)
		if err != nil {
			return nil, err
		}
		schema.Tables[tableName] = *tableInfo
	}

	return schema, nil
}

func getPostgreSQLTableInfo(db *sql.DB, tableName string) (*TableInfo, error) {
	info := &TableInfo{
		Name: tableName,
	}

	colRows, err := db.Query(`
		SELECT
			a.attname,
			pg_catalog.format_type(a.atttypid, a.atttypmod) AS col_type,
			CASE WHEN a.attnotnull THEN 'NO' ELSE 'YES' END AS is_nullable,
			pg_get_expr(ad.adbin, ad.adrelid) AS col_default,
			a.attnum AS ordinal_position,
			CASE WHEN EXISTS (
				SELECT 1 FROM pg_index pi
				WHERE pi.indrelid = c.oid AND pi.indisprimary AND a.attnum = ANY(pi.indkey)
			) THEN 'PRI' ELSE '' END AS col_key
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
		WHERE n.nspname = 'public'
		  AND c.relname = $1
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		ORDER BY a.attnum`, tableName)
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	var createParts []string
	var pkCols []string
	for colRows.Next() {
		var col ColumnInfo
		var colDefault sql.NullString
		if err := colRows.Scan(&col.Name, &col.Type, &col.Nullable, &colDefault, &col.Position, &col.Key); err != nil {
			return nil, err
		}
		if colDefault.Valid {
			col.Default = &colDefault.String
		}
		info.Columns = append(info.Columns, col)
		if col.Key == "PRI" {
			pkCols = append(pkCols, quoteIdentifier(PostgreSQL, col.Name))
		}

		colDef := fmt.Sprintf("%s %s", quoteIdentifier(PostgreSQL, col.Name), col.Type)
		if col.Nullable == "NO" {
			colDef += " NOT NULL"
		}
		if col.Default != nil {
			colDef += fmt.Sprintf(" DEFAULT %s", *col.Default)
		}
		createParts = append(createParts, colDef)
	}
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	if len(pkCols) > 0 {
		createParts = append(createParts, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkCols, ", ")))
	}
	info.CreateSQL = fmt.Sprintf("CREATE TABLE %s (\n  %s\n);", quoteIdentifier(PostgreSQL, tableName), strings.Join(createParts, ",\n  "))

	idxRows, err := db.Query(`
		SELECT
			ic.relname,
			CASE WHEN i.indisunique THEN 0 ELSE 1 END AS non_unique,
			a.attname,
			k.ord::int AS seq_in_idx
		FROM pg_index i
		JOIN pg_class t ON t.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		JOIN pg_class ic ON ic.oid = i.indexrelid
		CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY AS k(attnum, ord)
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
		WHERE n.nspname = 'public'
		  AND t.relname = $1
		  AND NOT i.indisprimary
		  AND k.attnum > 0
		ORDER BY ic.relname, k.ord`, tableName)
	if err != nil {
		return nil, err
	}
	defer idxRows.Close()

	for idxRows.Next() {
		var idx IndexInfo
		if err := idxRows.Scan(&idx.Name, &idx.NonUnique, &idx.Column, &idx.SeqInIdx); err != nil {
			return nil, err
		}
		info.Indexes = append(info.Indexes, idx)
	}
	if err := idxRows.Err(); err != nil {
		return nil, err
	}

	return info, nil
}

func getSQLiteSchema(config ConnectionConfig) (*SchemaInfo, error) {
	db, err := Connect(config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	schema := &SchemaInfo{
		Database: "main",
		Tables:   make(map[string]TableInfo),
	}

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, tableName := range tableNames {
		tableInfo, err := getSQLiteTableInfo(db, tableName)
		if err != nil {
			return nil, err
		}
		schema.Tables[tableName] = *tableInfo
	}

	return schema, nil
}

func getSQLiteTableInfo(db *sql.DB, tableName string) (*TableInfo, error) {
	info := &TableInfo{
		Name: tableName,
	}

	var createSQL string
	err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&createSQL)
	if err != nil {
		return nil, err
	}
	info.CreateSQL = createSQL

	escapedName := escapeSQLitePragmaName(tableName)

	colRows, err := db.Query(fmt.Sprintf("PRAGMA table_info('%s')", escapedName))
	if err != nil {
		return nil, err
	}
	for colRows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := colRows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			_ = colRows.Close()
			return nil, err
		}

		col := ColumnInfo{
			Name:     name,
			Type:     colType,
			Position: cid + 1,
		}
		if notNull == 1 {
			col.Nullable = "NO"
		} else {
			col.Nullable = "YES"
		}
		if dfltValue.Valid {
			col.Default = &dfltValue.String
		}
		if pk > 0 {
			col.Key = "PRI"
		}
		info.Columns = append(info.Columns, col)
	}
	if err := colRows.Err(); err != nil {
		_ = colRows.Close()
		return nil, err
	}
	_ = colRows.Close()

	type sqliteIdxMeta struct {
		name      string
		nonUnique int
		origin    string
	}
	var idxList []sqliteIdxMeta

	idxRows, err := db.Query(fmt.Sprintf("PRAGMA index_list('%s')", escapedName))
	if err != nil {
		return nil, err
	}
	for idxRows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := idxRows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			_ = idxRows.Close()
			return nil, err
		}
		if origin == "pk" || strings.HasPrefix(name, "sqlite_autoindex_") {
			continue
		}
		idxList = append(idxList, sqliteIdxMeta{
			name:      name,
			nonUnique: 1 - unique,
			origin:    origin,
		})
	}
	if err := idxRows.Err(); err != nil {
		_ = idxRows.Close()
		return nil, err
	}
	_ = idxRows.Close()

	for _, meta := range idxList {
		infoRows, err := db.Query(fmt.Sprintf("PRAGMA index_info('%s')", escapeSQLitePragmaName(meta.name)))
		if err != nil {
			return nil, err
		}
		for infoRows.Next() {
			var seqno int
			var cid int
			var colName sql.NullString
			if err := infoRows.Scan(&seqno, &cid, &colName); err != nil {
				_ = infoRows.Close()
				return nil, err
			}
			if colName.Valid && colName.String != "" {
				info.Indexes = append(info.Indexes, IndexInfo{
					Name:      meta.name,
					NonUnique: meta.nonUnique,
					Column:    colName.String,
					SeqInIdx:  seqno + 1,
				})
			}
		}
		if err := infoRows.Err(); err != nil {
			_ = infoRows.Close()
			return nil, err
		}
		_ = infoRows.Close()
	}

	return info, nil
}

func getSQLServerSchema(config ConnectionConfig) (*SchemaInfo, error) {
	db, err := Connect(config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	schema := &SchemaInfo{
		Database: config.Database,
		Tables:   make(map[string]TableInfo),
	}

	rows, err := db.Query(`
		SELECT TABLE_NAME
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = SCHEMA_NAME() AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, tableName := range tableNames {
		tableInfo, err := getSQLServerTableInfo(db, tableName)
		if err != nil {
			return nil, err
		}
		schema.Tables[tableName] = *tableInfo
	}

	return schema, nil
}

func formatSQLServerType(dataType string, charMaxLen, numPrecision, numScale sql.NullInt64) string {
	upper := strings.ToUpper(dataType)
	switch upper {
	case "VARCHAR", "NVARCHAR", "CHAR", "NCHAR", "VARBINARY", "BINARY":
		if charMaxLen.Valid {
			if charMaxLen.Int64 == -1 {
				return fmt.Sprintf("%s(max)", dataType)
			}
			return fmt.Sprintf("%s(%d)", dataType, charMaxLen.Int64)
		}
	case "DECIMAL", "NUMERIC":
		if numPrecision.Valid {
			if numScale.Valid {
				return fmt.Sprintf("%s(%d,%d)", dataType, numPrecision.Int64, numScale.Int64)
			}
			return fmt.Sprintf("%s(%d)", dataType, numPrecision.Int64)
		}
	}
	return dataType
}

func getSQLServerTableInfo(db *sql.DB, tableName string) (*TableInfo, error) {
	info := &TableInfo{
		Name: tableName,
	}

	colRows, err := db.Query(`
		SELECT COLUMN_NAME, DATA_TYPE, CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, NUMERIC_SCALE, IS_NULLABLE, COLUMN_DEFAULT, ORDINAL_POSITION
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = SCHEMA_NAME() AND TABLE_NAME = @p1
		ORDER BY ORDINAL_POSITION`, tableName)
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	var createParts []string
	for colRows.Next() {
		var col ColumnInfo
		var dataType string
		var charMaxLen, numPrecision, numScale sql.NullInt64
		var colDefault sql.NullString
		if err := colRows.Scan(&col.Name, &dataType, &charMaxLen, &numPrecision, &numScale, &col.Nullable, &colDefault, &col.Position); err != nil {
			return nil, err
		}
		col.Type = formatSQLServerType(dataType, charMaxLen, numPrecision, numScale)
		if colDefault.Valid {
			col.Default = &colDefault.String
		}
		info.Columns = append(info.Columns, col)

		colDef := fmt.Sprintf("%s %s", quoteIdentifier(SQLServer, col.Name), col.Type)
		if col.Nullable == "NO" {
			colDef += " NOT NULL"
		}
		if col.Default != nil {
			colDef += fmt.Sprintf(" DEFAULT %s", *col.Default)
		}
		createParts = append(createParts, colDef)
	}
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	info.CreateSQL = fmt.Sprintf("CREATE TABLE %s (\n  %s\n);", quoteIdentifier(SQLServer, tableName), strings.Join(createParts, ",\n  "))

	idxRows, err := db.Query(`
		SELECT i.name, c.name as column_name, i.is_unique, ic.key_ordinal
		FROM sys.indexes i
		JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		WHERE i.object_id = OBJECT_ID(QUOTENAME(SCHEMA_NAME()) + '.' + QUOTENAME(@p1))
		  AND i.name IS NOT NULL
		  AND i.is_primary_key = 0
		  AND ic.is_included_column = 0
		ORDER BY i.name, ic.key_ordinal`, tableName)
	if err != nil {
		return nil, err
	}
	defer idxRows.Close()

	for idxRows.Next() {
		var idxName, colName string
		var isUnique bool
		var keyOrdinal int
		if err := idxRows.Scan(&idxName, &colName, &isUnique, &keyOrdinal); err != nil {
			return nil, err
		}
		nonUnique := 1
		if isUnique {
			nonUnique = 0
		}
		info.Indexes = append(info.Indexes, IndexInfo{
			Name:      idxName,
			Column:    colName,
			NonUnique: nonUnique,
			SeqInIdx:  keyOrdinal,
		})
	}
	if err := idxRows.Err(); err != nil {
		return nil, err
	}

	return info, nil
}

// CompareSchemas compares two schemas and returns differences using MySQL syntax.
func CompareSchemas(source, target *SchemaInfo) []DiffResult {
	return CompareSchemasWithDialect(source, target, MySQL)
}

// CompareSchemasWithDialect compares two schemas and generates SQL using target dialect syntax.
func CompareSchemasWithDialect(source, target *SchemaInfo, dialect DBType) []DiffResult {
	if dialect == "" {
		dialect = MySQL
	}

	var results []DiffResult

	sourceNames := make([]string, 0, len(source.Tables))
	for name := range source.Tables {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)

	targetNames := make([]string, 0, len(target.Tables))
	for name := range target.Tables {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)

	for _, tableName := range sourceNames {
		sourceTable := source.Tables[tableName]
		if _, exists := target.Tables[tableName]; !exists {
			createSQL := strings.TrimRight(strings.TrimSpace(sourceTable.CreateSQL), ";")
			results = append(results, DiffResult{
				Type:      "added",
				TableName: tableName,
				Detail:    "Table exists in source but not in target",
				SQL:       createSQL + ";",
			})
		}
	}

	for _, tableName := range targetNames {
		if _, exists := source.Tables[tableName]; !exists {
			results = append(results, DiffResult{
				Type:      "removed",
				TableName: tableName,
				Detail:    "Table exists in target but not in source",
				SQL:       fmt.Sprintf("DROP TABLE %s;", quoteIdentifier(dialect, tableName)),
			})
		}
	}

	for _, tableName := range sourceNames {
		sourceTable := source.Tables[tableName]
		if targetTable, exists := target.Tables[tableName]; exists {
			tableDiffs := compareTableStructure(tableName, sourceTable, targetTable, dialect)
			results = append(results, tableDiffs...)
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Type != results[j].Type {
			order := map[string]int{"added": 0, "modified": 1, "removed": 2}
			return order[results[i].Type] < order[results[j].Type]
		}
		return results[i].TableName < results[j].TableName
	})

	return results
}

func compareTableStructure(tableName string, source, target TableInfo, dialect DBType) []DiffResult {
	var results []DiffResult

	sourceCols := make([]ColumnInfo, len(source.Columns))
	copy(sourceCols, source.Columns)
	sort.Slice(sourceCols, func(i, j int) bool {
		if sourceCols[i].Position != sourceCols[j].Position {
			return sourceCols[i].Position < sourceCols[j].Position
		}
		return sourceCols[i].Name < sourceCols[j].Name
	})

	targetCols := make([]ColumnInfo, len(target.Columns))
	copy(targetCols, target.Columns)
	sort.Slice(targetCols, func(i, j int) bool {
		if targetCols[i].Position != targetCols[j].Position {
			return targetCols[i].Position < targetCols[j].Position
		}
		return targetCols[i].Name < targetCols[j].Name
	})

	sourceColMap := make(map[string]ColumnInfo, len(sourceCols))
	targetColMap := make(map[string]ColumnInfo, len(targetCols))
	for _, col := range sourceCols {
		sourceColMap[col.Name] = col
	}
	for _, col := range targetCols {
		targetColMap[col.Name] = col
	}

	quotedTable := quoteIdentifier(dialect, tableName)
	availableCols := make(map[string]bool, len(targetColMap))
	for k := range targetColMap {
		availableCols[k] = true
	}

	// Find added columns in source position order so AFTER references stay valid
	for idx, sourceCol := range sourceCols {
		colName := sourceCol.Name
		if _, exists := targetColMap[colName]; !exists {
			colDef := buildColumnDefForDialect(sourceCol, dialect)
			quotedCol := quoteIdentifier(dialect, colName)
			var sqlStmt string

			switch dialect {
			case MySQL:
				afterClause := ""
				if idx == 0 || sourceCol.Position == 1 {
					afterClause = " FIRST"
				} else if idx > 0 {
					prevName := sourceCols[idx-1].Name
					if availableCols[prevName] {
						afterClause = fmt.Sprintf(" AFTER %s", quoteIdentifier(MySQL, prevName))
					}
				}
				sqlStmt = fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s%s;", quotedTable, quotedCol, colDef, afterClause)
			case SQLServer:
				sqlStmt = fmt.Sprintf("ALTER TABLE %s ADD %s %s;", quotedTable, quotedCol, colDef)
			default:
				sqlStmt = fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", quotedTable, quotedCol, colDef)
			}

			availableCols[colName] = true
			results = append(results, DiffResult{
				Type:      "modified",
				TableName: tableName,
				Detail:    fmt.Sprintf("Add column: %s", colName),
				SQL:       sqlStmt,
			})
		}
	}

	// Find removed columns
	for _, targetCol := range targetCols {
		colName := targetCol.Name
		if _, exists := sourceColMap[colName]; !exists {
			results = append(results, DiffResult{
				Type:      "modified",
				TableName: tableName,
				Detail:    fmt.Sprintf("Drop column: %s", colName),
				SQL:       fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", quotedTable, quoteIdentifier(dialect, colName)),
			})
		}
	}

	// Find modified columns
	for _, sourceCol := range sourceCols {
		colName := sourceCol.Name
		if targetCol, exists := targetColMap[colName]; exists {
			if !columnsEqual(sourceCol, targetCol) {
				results = append(results, DiffResult{
					Type:      "modified",
					TableName: tableName,
					Detail:    fmt.Sprintf("Modify column: %s (%s -> %s)", colName, targetCol.Type, sourceCol.Type),
					SQL:       buildModifyColumnSQL(tableName, sourceCol, targetCol, dialect),
				})
			}
		}
	}

	// Compare indexes
	sourceIdxMap := buildIndexSpecMap(source.Indexes)
	targetIdxMap := buildIndexSpecMap(target.Indexes)

	sourceIdxNames := make([]string, 0, len(sourceIdxMap))
	for name := range sourceIdxMap {
		sourceIdxNames = append(sourceIdxNames, name)
	}
	sort.Strings(sourceIdxNames)

	targetIdxNames := make([]string, 0, len(targetIdxMap))
	for name := range targetIdxMap {
		targetIdxNames = append(targetIdxNames, name)
	}
	sort.Strings(targetIdxNames)

	for _, idxName := range sourceIdxNames {
		if idxName == "PRIMARY" || strings.HasPrefix(idxName, "sqlite_autoindex_") {
			continue
		}
		sourceSpec := sourceIdxMap[idxName]
		if targetSpec, exists := targetIdxMap[idxName]; !exists {
			results = append(results, DiffResult{
				Type:      "modified",
				TableName: tableName,
				Detail:    fmt.Sprintf("Add index: %s", idxName),
				SQL:       buildAddIndexSQL(tableName, idxName, sourceSpec, dialect),
			})
		} else if !indexSpecsEqual(sourceSpec, targetSpec) {
			results = append(results, DiffResult{
				Type:      "modified",
				TableName: tableName,
				Detail:    fmt.Sprintf("Recreate index: %s", idxName),
				SQL:       buildRecreateIndexSQL(tableName, idxName, sourceSpec, dialect),
			})
		}
	}

	for _, idxName := range targetIdxNames {
		if idxName == "PRIMARY" || strings.HasPrefix(idxName, "sqlite_autoindex_") {
			continue
		}
		if _, exists := sourceIdxMap[idxName]; !exists {
			results = append(results, DiffResult{
				Type:      "modified",
				TableName: tableName,
				Detail:    fmt.Sprintf("Drop index: %s", idxName),
				SQL:       buildDropIndexSQL(tableName, idxName, dialect),
			})
		}
	}

	return results
}

func buildModifyColumnSQL(tableName string, sourceCol, targetCol ColumnInfo, dialect DBType) string {
	quotedTable := quoteIdentifier(dialect, tableName)
	quotedCol := quoteIdentifier(dialect, sourceCol.Name)

	switch dialect {
	case MySQL:
		return fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s %s;", quotedTable, quotedCol, buildColumnDefForDialect(sourceCol, dialect))
	case PostgreSQL:
		var stmts []string
		if sourceCol.Type != targetCol.Type {
			stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;", quotedTable, quotedCol, sourceCol.Type))
		}
		if sourceCol.Nullable != targetCol.Nullable {
			if sourceCol.Nullable == "NO" {
				stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;", quotedTable, quotedCol))
			} else {
				stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;", quotedTable, quotedCol))
			}
		}
		if !defaultsEqual(sourceCol.Default, targetCol.Default) {
			if sourceCol.Default == nil {
				stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;", quotedTable, quotedCol))
			} else {
				stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;", quotedTable, quotedCol, formatDefaultClause(*sourceCol.Default)))
			}
		}
		if len(stmts) == 0 {
			return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;", quotedTable, quotedCol, sourceCol.Type)
		}
		return strings.Join(stmts, "\n")
	case SQLServer:
		nullClause := "NULL"
		if sourceCol.Nullable == "NO" {
			nullClause = "NOT NULL"
		}
		return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s %s %s;", quotedTable, quotedCol, sourceCol.Type, nullClause)
	case SQLite:
		return fmt.Sprintf("-- SQLite does not support ALTER COLUMN directly; recreate table %s to change column %s (%s).", quotedTable, quotedCol, buildColumnDefForDialect(sourceCol, SQLite))
	default:
		return fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s %s;", quotedTable, quotedCol, buildColumnDefForDialect(sourceCol, dialect))
	}
}

type indexSpec struct {
	unique bool
	cols   []string
}

func buildIndexSpecMap(indexes []IndexInfo) map[string]indexSpec {
	grouped := make(map[string][]IndexInfo)
	for _, idx := range indexes {
		if idx.Name == "" || idx.Column == "" {
			continue
		}
		grouped[idx.Name] = append(grouped[idx.Name], idx)
	}

	result := make(map[string]indexSpec, len(grouped))
	for name, items := range grouped {
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].SeqInIdx < items[j].SeqInIdx
		})
		cols := make([]string, 0, len(items))
		unique := false
		for _, item := range items {
			if item.NonUnique == 0 {
				unique = true
			}
			cols = append(cols, item.Column)
		}
		result[name] = indexSpec{
			unique: unique,
			cols:   cols,
		}
	}
	return result
}

func indexSpecsEqual(a, b indexSpec) bool {
	return a.unique == b.unique && stringSlicesEqual(a.cols, b.cols)
}

func quoteIndexColumns(cols []string, dialect DBType) string {
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = quoteIdentifier(dialect, c)
	}
	return strings.Join(quoted, ", ")
}

func buildAddIndexSQL(tableName, idxName string, spec indexSpec, dialect DBType) string {
	colsSQL := quoteIndexColumns(spec.cols, dialect)
	quotedTable := quoteIdentifier(dialect, tableName)
	quotedIdx := quoteIdentifier(dialect, idxName)
	uniqueKW := ""
	if spec.unique {
		uniqueKW = "UNIQUE "
	}

	switch dialect {
	case MySQL:
		return fmt.Sprintf("ALTER TABLE %s ADD %sINDEX %s (%s);", quotedTable, uniqueKW, quotedIdx, colsSQL)
	default:
		return fmt.Sprintf("CREATE %sINDEX %s ON %s (%s);", uniqueKW, quotedIdx, quotedTable, colsSQL)
	}
}

func buildDropIndexSQL(tableName, idxName string, dialect DBType) string {
	quotedTable := quoteIdentifier(dialect, tableName)
	quotedIdx := quoteIdentifier(dialect, idxName)

	switch dialect {
	case MySQL:
		return fmt.Sprintf("ALTER TABLE %s DROP INDEX %s;", quotedTable, quotedIdx)
	case SQLServer:
		return fmt.Sprintf("DROP INDEX %s ON %s;", quotedIdx, quotedTable)
	default:
		return fmt.Sprintf("DROP INDEX IF EXISTS %s;", quotedIdx)
	}
}

func buildRecreateIndexSQL(tableName, idxName string, spec indexSpec, dialect DBType) string {
	colsSQL := quoteIndexColumns(spec.cols, dialect)
	quotedTable := quoteIdentifier(dialect, tableName)
	quotedIdx := quoteIdentifier(dialect, idxName)
	uniqueKW := ""
	if spec.unique {
		uniqueKW = "UNIQUE "
	}

	if dialect == MySQL {
		return fmt.Sprintf("ALTER TABLE %s DROP INDEX %s, ADD %sINDEX %s (%s);", quotedTable, quotedIdx, uniqueKW, quotedIdx, colsSQL)
	}
	return buildDropIndexSQL(tableName, idxName, dialect) + "\n" + buildAddIndexSQL(tableName, idxName, spec, dialect)
}

func buildColumnDef(col ColumnInfo) string {
	return buildColumnDefForDialect(col, MySQL)
}

func formatDefaultClause(defaultVal string) string {
	if isNumericDefault(defaultVal) || isSpecialDefault(defaultVal) {
		return defaultVal
	}
	if len(defaultVal) >= 2 && strings.HasPrefix(defaultVal, "'") && strings.HasSuffix(defaultVal, "'") {
		return defaultVal
	}
	escaped := strings.ReplaceAll(defaultVal, "'", "''")
	return fmt.Sprintf("'%s'", escaped)
}

func buildColumnDefForDialect(col ColumnInfo, dialect DBType) string {
	def := col.Type
	if col.Nullable == "NO" {
		def += " NOT NULL"
	}
	if col.Default != nil {
		def += " DEFAULT " + formatDefaultClause(*col.Default)
	}
	if dialect == MySQL && col.Extra != "" {
		def += " " + col.Extra
	}
	return def
}

func isNumericDefault(val string) bool {
	if val == "" {
		return false
	}
	hasDigit := false
	dotCount := 0
	for i, c := range val {
		if i == 0 && c == '-' {
			continue
		}
		if c == '.' {
			dotCount++
			if dotCount > 1 {
				return false
			}
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
		hasDigit = true
	}
	return hasDigit
}

func isSpecialDefault(val string) bool {
	upper := strings.ToUpper(strings.TrimSpace(val))
	specialDefaults := []string{
		"NULL", "CURRENT_TIMESTAMP", "CURRENT_DATE", "CURRENT_TIME",
		"NOW()", "TRUE", "FALSE", "GETDATE()", "NEWID()",
	}
	for _, special := range specialDefaults {
		if upper == special {
			return true
		}
	}
	if strings.HasSuffix(upper, "()") || strings.HasPrefix(upper, "(") {
		return true
	}
	if strings.Contains(upper, "::") {
		return true
	}
	return false
}

func columnsEqual(a, b ColumnInfo) bool {
	return a.Type == b.Type && a.Nullable == b.Nullable &&
		a.Extra == b.Extra && defaultsEqual(a.Default, b.Default)
}

func defaultsEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func isValidSQLIdentifierToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// CreateDatabase creates a new database
func CreateDatabase(config ConnectionConfig, dbName, charset, collation string) error {
	switch config.Type {
	case MySQL, "":
		return createMySQLDatabase(config, dbName, charset, collation)
	case PostgreSQL:
		return createPostgreSQLDatabase(config, dbName)
	case SQLServer:
		return createSQLServerDatabase(config, dbName)
	case SQLite:
		return fmt.Errorf("SQLite databases are file-based, use a new file path")
	default:
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

func createMySQLDatabase(config ConnectionConfig, dbName, charset, collation string) error {
	db, err := connectWithDSN("mysql", mysqlDSN(config, false))
	if err != nil {
		return err
	}
	defer db.Close()

	sqlStmt := fmt.Sprintf("CREATE DATABASE %s", quoteIdentifier(MySQL, dbName))
	if charset != "" {
		if !isValidSQLIdentifierToken(charset) {
			return fmt.Errorf("invalid charset: %s", charset)
		}
		sqlStmt += fmt.Sprintf(" CHARACTER SET %s", charset)
	}
	if collation != "" {
		if !isValidSQLIdentifierToken(collation) {
			return fmt.Errorf("invalid collation: %s", collation)
		}
		sqlStmt += fmt.Sprintf(" COLLATE %s", collation)
	}

	_, err = db.Exec(sqlStmt)
	return err
}

func createPostgreSQLDatabase(config ConnectionConfig, dbName string) error {
	cfg := config
	cfg.Database = "postgres"

	db, err := Connect(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", quoteIdentifier(PostgreSQL, dbName)))
	return err
}

func createSQLServerDatabase(config ConnectionConfig, dbName string) error {
	cfg := config
	cfg.Database = "master"

	db, err := Connect(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", quoteIdentifier(SQLServer, dbName)))
	return err
}

// DropDatabase drops a database
func DropDatabase(config ConnectionConfig, dbName string) error {
	switch config.Type {
	case MySQL, "":
		db, err := connectWithDSN("mysql", mysqlDSN(config, false))
		if err != nil {
			return err
		}
		defer db.Close()
		_, err = db.Exec(fmt.Sprintf("DROP DATABASE %s", quoteIdentifier(MySQL, dbName)))
		return err
	case PostgreSQL:
		cfg := config
		cfg.Database = "postgres"
		db, err := Connect(cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		_, err = db.Exec(fmt.Sprintf("DROP DATABASE %s", quoteIdentifier(PostgreSQL, dbName)))
		return err
	case SQLServer:
		cfg := config
		cfg.Database = "master"
		db, err := Connect(cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		_, err = db.Exec(fmt.Sprintf("DROP DATABASE %s", quoteIdentifier(SQLServer, dbName)))
		return err
	default:
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}
}
