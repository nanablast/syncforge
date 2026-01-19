package database

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
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

// buildDSN builds the connection string for the given database type
func buildDSN(config ConnectionConfig) (string, string, error) {
	switch config.Type {
	case MySQL, "":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
			config.User, config.Password, config.Host, config.Port, config.Database)
		return "mysql", dsn, nil

	case PostgreSQL:
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			config.Host, config.Port, config.User, config.Password, config.Database)
		return "postgres", dsn, nil

	case SQLite:
		if config.FilePath == "" {
			return "", "", fmt.Errorf("SQLite requires a file path")
		}
		return "sqlite3", config.FilePath, nil

	case SQLServer:
		dsn := fmt.Sprintf("server=%s;port=%d;user id=%s;password=%s;database=%s",
			config.Host, config.Port, config.User, config.Password, config.Database)
		return "sqlserver", dsn, nil

	default:
		return "", "", fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

// Connect creates a database connection
func Connect(config ConnectionConfig) (*sql.DB, error) {
	driver, dsn, err := buildDSN(config)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	return db, nil
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
		// SQLite doesn't have multiple databases
		return []string{"main"}, nil
	case SQLServer:
		return getSQLServerDatabases(config)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

func getMySQLDatabases(config ConnectionConfig) ([]string, error) {
	cfg := config
	cfg.Database = ""

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/",
		cfg.User, cfg.Password, cfg.Host, cfg.Port)

	db, err := sql.Open("mysql", dsn)
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
		// Skip system databases
		if name != "information_schema" && name != "mysql" &&
			name != "performance_schema" && name != "sys" {
			databases = append(databases, name)
		}
	}

	return databases, nil
}

func getPostgreSQLDatabases(config ConnectionConfig) ([]string, error) {
	cfg := config
	cfg.Database = "postgres"

	db, err := Connect(cfg)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT datname FROM pg_database WHERE datistemplate = false AND datname NOT IN ('postgres')")
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

	return databases, nil
}

func getSQLServerDatabases(config ConnectionConfig) ([]string, error) {
	cfg := config
	cfg.Database = "master"

	db, err := Connect(cfg)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT name FROM sys.databases WHERE name NOT IN ('master', 'tempdb', 'model', 'msdb')")
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

	return databases, nil
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

	rows, err := db.Query("SHOW TABLES")
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

	for _, tableName := range tableNames {
		tableInfo, err := getMySQLTableInfo(db, tableName)
		if err != nil {
			return nil, err
		}
		schema.Tables[tableName] = *tableInfo
	}

	return schema, nil
}

func getMySQLTableInfo(db *sql.DB, tableName string) (*TableInfo, error) {
	info := &TableInfo{
		Name: tableName,
	}

	var tbl, createSQL string
	err := db.QueryRow(fmt.Sprintf("SHOW CREATE TABLE `%s`", tableName)).Scan(&tbl, &createSQL)
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

	idxRows, err := db.Query(fmt.Sprintf("SHOW INDEX FROM `%s`", tableName))
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
				if v, ok := val.([]byte); ok {
					idx.Name = string(v)
				}
			case "Non_unique":
				if v, ok := val.(int64); ok {
					idx.NonUnique = int(v)
				}
			case "Column_name":
				if v, ok := val.([]byte); ok {
					idx.Column = string(v)
				}
			case "Seq_in_index":
				if v, ok := val.(int64); ok {
					idx.SeqInIdx = int(v)
				}
			}
		}
		info.Indexes = append(info.Indexes, idx)
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
		WHERE schemaname = 'public'`)
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

	// PostgreSQL doesn't have SHOW CREATE TABLE, we need to build it
	colRows, err := db.Query(`
		SELECT column_name, data_type, is_nullable, column_default, ordinal_position
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position`, tableName)
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	var createParts []string
	for colRows.Next() {
		var col ColumnInfo
		var colDefault sql.NullString
		if err := colRows.Scan(&col.Name, &col.Type, &col.Nullable, &colDefault, &col.Position); err != nil {
			return nil, err
		}
		if colDefault.Valid {
			col.Default = &colDefault.String
		}
		info.Columns = append(info.Columns, col)

		// Build column definition
		colDef := fmt.Sprintf("%s %s", col.Name, col.Type)
		if col.Nullable == "NO" {
			colDef += " NOT NULL"
		}
		if col.Default != nil {
			colDef += fmt.Sprintf(" DEFAULT %s", *col.Default)
		}
		createParts = append(createParts, colDef)
	}

	info.CreateSQL = fmt.Sprintf("CREATE TABLE %s (\n  %s\n);", tableName, strings.Join(createParts, ",\n  "))

	// Get indexes
	idxRows, err := db.Query(`
		SELECT indexname, indexdef
		FROM pg_indexes
		WHERE schemaname = 'public' AND tablename = $1`, tableName)
	if err != nil {
		return nil, err
	}
	defer idxRows.Close()

	for idxRows.Next() {
		var idxName, idxDef string
		if err := idxRows.Scan(&idxName, &idxDef); err != nil {
			return nil, err
		}
		info.Indexes = append(info.Indexes, IndexInfo{
			Name:   idxName,
			Column: idxDef,
		})
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

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
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

	// Get CREATE TABLE statement
	var createSQL string
	err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&createSQL)
	if err != nil {
		return nil, err
	}
	info.CreateSQL = createSQL

	// Get columns
	colRows, err := db.Query(fmt.Sprintf("PRAGMA table_info('%s')", tableName))
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	for colRows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := colRows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
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
		if pk == 1 {
			col.Key = "PRI"
		}
		info.Columns = append(info.Columns, col)
	}

	// Get indexes
	idxRows, err := db.Query(fmt.Sprintf("PRAGMA index_list('%s')", tableName))
	if err != nil {
		return nil, err
	}
	defer idxRows.Close()

	for idxRows.Next() {
		var seq int
		var name string
		var unique int
		var origin, partial string
		if err := idxRows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, err
		}
		info.Indexes = append(info.Indexes, IndexInfo{
			Name:      name,
			NonUnique: 1 - unique,
		})
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

	rows, err := db.Query("SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE = 'BASE TABLE'")
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

	for _, tableName := range tableNames {
		tableInfo, err := getSQLServerTableInfo(db, tableName)
		if err != nil {
			return nil, err
		}
		schema.Tables[tableName] = *tableInfo
	}

	return schema, nil
}

func getSQLServerTableInfo(db *sql.DB, tableName string) (*TableInfo, error) {
	info := &TableInfo{
		Name: tableName,
	}

	// Get columns
	colRows, err := db.Query(`
		SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT, ORDINAL_POSITION
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_NAME = @p1
		ORDER BY ORDINAL_POSITION`, tableName)
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	var createParts []string
	for colRows.Next() {
		var col ColumnInfo
		var colDefault sql.NullString
		if err := colRows.Scan(&col.Name, &col.Type, &col.Nullable, &colDefault, &col.Position); err != nil {
			return nil, err
		}
		if colDefault.Valid {
			col.Default = &colDefault.String
		}
		info.Columns = append(info.Columns, col)

		colDef := fmt.Sprintf("[%s] %s", col.Name, col.Type)
		if col.Nullable == "NO" {
			colDef += " NOT NULL"
		}
		if col.Default != nil {
			colDef += fmt.Sprintf(" DEFAULT %s", *col.Default)
		}
		createParts = append(createParts, colDef)
	}

	info.CreateSQL = fmt.Sprintf("CREATE TABLE [%s] (\n  %s\n);", tableName, strings.Join(createParts, ",\n  "))

	// Get indexes
	idxRows, err := db.Query(`
		SELECT i.name, c.name as column_name, i.is_unique
		FROM sys.indexes i
		JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		WHERE i.object_id = OBJECT_ID(@p1) AND i.name IS NOT NULL`, tableName)
	if err != nil {
		return nil, err
	}
	defer idxRows.Close()

	for idxRows.Next() {
		var idxName, colName string
		var isUnique bool
		if err := idxRows.Scan(&idxName, &colName, &isUnique); err != nil {
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
		})
	}

	return info, nil
}

// CompareSchemas compares two schemas and returns differences
func CompareSchemas(source, target *SchemaInfo) []DiffResult {
	var results []DiffResult

	// Find tables only in source (need to add to target)
	for tableName, sourceTable := range source.Tables {
		if _, exists := target.Tables[tableName]; !exists {
			results = append(results, DiffResult{
				Type:      "added",
				TableName: tableName,
				Detail:    "Table exists in source but not in target",
				SQL:       sourceTable.CreateSQL + ";",
			})
		}
	}

	// Find tables only in target (need to remove from target)
	for tableName := range target.Tables {
		if _, exists := source.Tables[tableName]; !exists {
			results = append(results, DiffResult{
				Type:      "removed",
				TableName: tableName,
				Detail:    "Table exists in target but not in source",
				SQL:       fmt.Sprintf("DROP TABLE `%s`;", tableName),
			})
		}
	}

	// Compare existing tables
	for tableName, sourceTable := range source.Tables {
		if targetTable, exists := target.Tables[tableName]; exists {
			tableDiffs := compareTableStructure(tableName, sourceTable, targetTable)
			results = append(results, tableDiffs...)
		}
	}

	// Sort results by type and table name
	sort.Slice(results, func(i, j int) bool {
		if results[i].Type != results[j].Type {
			order := map[string]int{"added": 0, "modified": 1, "removed": 2}
			return order[results[i].Type] < order[results[j].Type]
		}
		return results[i].TableName < results[j].TableName
	})

	return results
}

func compareTableStructure(tableName string, source, target TableInfo) []DiffResult {
	var results []DiffResult

	sourceColMap := make(map[string]ColumnInfo)
	targetColMap := make(map[string]ColumnInfo)

	for _, col := range source.Columns {
		sourceColMap[col.Name] = col
	}
	for _, col := range target.Columns {
		targetColMap[col.Name] = col
	}

	// Find added columns
	for colName, sourceCol := range sourceColMap {
		if _, exists := targetColMap[colName]; !exists {
			afterClause := ""
			if sourceCol.Position > 1 {
				for _, c := range source.Columns {
					if c.Position == sourceCol.Position-1 {
						afterClause = fmt.Sprintf(" AFTER `%s`", c.Name)
						break
					}
				}
			} else {
				afterClause = " FIRST"
			}

			results = append(results, DiffResult{
				Type:      "modified",
				TableName: tableName,
				Detail:    fmt.Sprintf("Add column: %s", colName),
				SQL:       fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s%s;", tableName, colName, buildColumnDef(sourceCol), afterClause),
			})
		}
	}

	// Find removed columns
	for colName := range targetColMap {
		if _, exists := sourceColMap[colName]; !exists {
			results = append(results, DiffResult{
				Type:      "modified",
				TableName: tableName,
				Detail:    fmt.Sprintf("Drop column: %s", colName),
				SQL:       fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`;", tableName, colName),
			})
		}
	}

	// Find modified columns
	for colName, sourceCol := range sourceColMap {
		if targetCol, exists := targetColMap[colName]; exists {
			if !columnsEqual(sourceCol, targetCol) {
				results = append(results, DiffResult{
					Type:      "modified",
					TableName: tableName,
					Detail:    fmt.Sprintf("Modify column: %s (%s -> %s)", colName, targetCol.Type, sourceCol.Type),
					SQL:       fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` %s;", tableName, colName, buildColumnDef(sourceCol)),
				})
			}
		}
	}

	// Compare indexes
	sourceIdxMap := buildIndexMap(source.Indexes)
	targetIdxMap := buildIndexMap(target.Indexes)

	for idxName, sourceCols := range sourceIdxMap {
		if idxName == "PRIMARY" {
			continue // Skip primary key for now
		}
		if targetCols, exists := targetIdxMap[idxName]; !exists {
			results = append(results, DiffResult{
				Type:      "modified",
				TableName: tableName,
				Detail:    fmt.Sprintf("Add index: %s", idxName),
				SQL:       fmt.Sprintf("ALTER TABLE `%s` ADD INDEX `%s` (%s);", tableName, idxName, strings.Join(sourceCols, ", ")),
			})
		} else if !stringSlicesEqual(sourceCols, targetCols) {
			results = append(results, DiffResult{
				Type:      "modified",
				TableName: tableName,
				Detail:    fmt.Sprintf("Recreate index: %s", idxName),
				SQL:       fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`, ADD INDEX `%s` (%s);", tableName, idxName, idxName, strings.Join(sourceCols, ", ")),
			})
		}
	}

	for idxName := range targetIdxMap {
		if idxName == "PRIMARY" {
			continue
		}
		if _, exists := sourceIdxMap[idxName]; !exists {
			results = append(results, DiffResult{
				Type:      "modified",
				TableName: tableName,
				Detail:    fmt.Sprintf("Drop index: %s", idxName),
				SQL:       fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`;", tableName, idxName),
			})
		}
	}

	return results
}

func buildColumnDef(col ColumnInfo) string {
	def := col.Type
	if col.Nullable == "NO" {
		def += " NOT NULL"
	}
	if col.Default != nil {
		defaultVal := *col.Default
		// Don't quote numeric defaults, NULL, or function calls like CURRENT_TIMESTAMP
		if isNumericDefault(defaultVal) || isSpecialDefault(defaultVal) {
			def += fmt.Sprintf(" DEFAULT %s", defaultVal)
		} else {
			def += fmt.Sprintf(" DEFAULT '%s'", defaultVal)
		}
	}
	if col.Extra != "" {
		def += " " + col.Extra
	}
	return def
}

func isNumericDefault(val string) bool {
	if val == "" {
		return false
	}
	// Check if it's a number (integer or float, possibly negative)
	for i, c := range val {
		if i == 0 && c == '-' {
			continue
		}
		if c == '.' {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isSpecialDefault(val string) bool {
	upper := strings.ToUpper(val)
	specialDefaults := []string{"NULL", "CURRENT_TIMESTAMP", "CURRENT_DATE", "CURRENT_TIME", "NOW()", "TRUE", "FALSE"}
	for _, special := range specialDefaults {
		if upper == special {
			return true
		}
	}
	// Check for function calls like CURRENT_TIMESTAMP() or expressions
	if strings.HasSuffix(upper, "()") || strings.HasPrefix(upper, "(") {
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

func buildIndexMap(indexes []IndexInfo) map[string][]string {
	result := make(map[string][]string)
	for _, idx := range indexes {
		result[idx.Name] = append(result[idx.Name], fmt.Sprintf("`%s`", idx.Column))
	}
	return result
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
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/",
		config.User, config.Password, config.Host, config.Port)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	sqlStmt := fmt.Sprintf("CREATE DATABASE `%s`", dbName)
	if charset != "" {
		sqlStmt += fmt.Sprintf(" CHARACTER SET %s", charset)
	}
	if collation != "" {
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

	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
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

	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE [%s]", dbName))
	return err
}

// DropDatabase drops a database
func DropDatabase(config ConnectionConfig, dbName string) error {
	switch config.Type {
	case MySQL, "":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/",
			config.User, config.Password, config.Host, config.Port)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return err
		}
		defer db.Close()
		_, err = db.Exec(fmt.Sprintf("DROP DATABASE `%s`", dbName))
		return err
	case PostgreSQL:
		cfg := config
		cfg.Database = "postgres"
		db, err := Connect(cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		_, err = db.Exec(fmt.Sprintf("DROP DATABASE %s", dbName))
		return err
	case SQLServer:
		cfg := config
		cfg.Database = "master"
		db, err := Connect(cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		_, err = db.Exec(fmt.Sprintf("DROP DATABASE [%s]", dbName))
		return err
	default:
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}
}
