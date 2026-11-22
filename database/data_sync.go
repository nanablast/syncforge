package database

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// DataSyncConfig holds sync configuration
type DataSyncConfig struct {
	SourceConfig ConnectionConfig `json:"sourceConfig"`
	TargetConfig ConnectionConfig `json:"targetConfig"`
	TableName    string           `json:"tableName"`
	SyncInsert   bool             `json:"syncInsert"`
	SyncUpdate   bool             `json:"syncUpdate"`
	SyncDelete   bool             `json:"syncDelete"`
}

// TableDataInfo holds table data comparison info
type TableDataInfo struct {
	TableName   string   `json:"tableName"`
	PrimaryKeys []string `json:"primaryKeys"`
	Columns     []string `json:"columns"`
	SourceCount int      `json:"sourceCount"`
	TargetCount int      `json:"targetCount"`
	InsertCount int      `json:"insertCount"`
	UpdateCount int      `json:"updateCount"`
	DeleteCount int      `json:"deleteCount"`
}

// DataDiffResult holds data difference details
type DataDiffResult struct {
	Type       string                 `json:"type"` // "insert", "update", "delete"
	TableName  string                 `json:"tableName"`
	PrimaryKey map[string]interface{} `json:"primaryKey"`
	OldValues  map[string]interface{} `json:"oldValues,omitempty"`
	NewValues  map[string]interface{} `json:"newValues,omitempty"`
	SQL        string                 `json:"sql"`
}

// GetTablesForSync returns list of tables available for data sync
func GetTablesForSync(config ConnectionConfig) ([]TableDataInfo, error) {
	db, err := Connect(config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	dbType := config.Type
	if dbType == "" {
		dbType = MySQL
	}

	tableNames, err := getTableNames(db, dbType, config.Database)
	if err != nil {
		return nil, err
	}

	var tables []TableDataInfo
	for _, tableName := range tableNames {
		info := TableDataInfo{TableName: tableName}

		info.PrimaryKeys, err = getPrimaryKeys(db, dbType, config.Database, tableName)
		if err != nil {
			return nil, err
		}

		info.Columns, err = getColumns(db, dbType, config.Database, tableName)
		if err != nil {
			return nil, err
		}

		var count int
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(dbType, tableName))
		err = db.QueryRow(countQuery).Scan(&count)
		if err != nil {
			return nil, err
		}
		info.SourceCount = count

		tables = append(tables, info)
	}

	return tables, nil
}

// getTableNames returns table names for the given database type
func getTableNames(db *sql.DB, dbType DBType, database string) ([]string, error) {
	var query string
	var args []interface{}

	switch dbType {
	case MySQL, "":
		query = "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME"
	case PostgreSQL:
		query = "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename"
	case SQLite:
		query = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
	case SQLServer:
		query = "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = SCHEMA_NAME() AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME"
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	rows, err := db.Query(query, args...)
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
	return tableNames, rows.Err()
}

func escapeSQLitePragmaName(name string) string {
	return strings.ReplaceAll(name, "'", "''")
}

// quoteIdentifier quotes and escapes an identifier based on database type
func quoteIdentifier(dbType DBType, name string) string {
	switch dbType {
	case MySQL, "":
		return fmt.Sprintf("`%s`", strings.ReplaceAll(name, "`", "``"))
	case PostgreSQL, SQLite:
		return fmt.Sprintf("\"%s\"", strings.ReplaceAll(name, "\"", "\"\""))
	case SQLServer:
		return fmt.Sprintf("[%s]", strings.ReplaceAll(name, "]", "]]"))
	default:
		return fmt.Sprintf("`%s`", strings.ReplaceAll(name, "`", "``"))
	}
}

// CompareTableData compares data between source and target tables
func CompareTableData(sourceConfig, targetConfig ConnectionConfig, tableName string) ([]DataDiffResult, error) {
	sourceDB, err := Connect(sourceConfig)
	if err != nil {
		return nil, fmt.Errorf("source connection failed: %v", err)
	}
	defer sourceDB.Close()

	targetDB, err := Connect(targetConfig)
	if err != nil {
		return nil, fmt.Errorf("target connection failed: %v", err)
	}
	defer targetDB.Close()

	sourceType := sourceConfig.Type
	if sourceType == "" {
		sourceType = MySQL
	}
	targetType := targetConfig.Type
	if targetType == "" {
		targetType = MySQL
	}

	primaryKeys, err := getPrimaryKeys(sourceDB, sourceType, sourceConfig.Database, tableName)
	if err != nil {
		return nil, err
	}
	if len(primaryKeys) == 0 {
		return nil, fmt.Errorf("table %s has no primary key", tableName)
	}

	columns, err := getColumns(sourceDB, sourceType, sourceConfig.Database, tableName)
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("table %s has no columns", tableName)
	}

	sourceData, err := getTableData(sourceDB, sourceType, tableName, columns, primaryKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to get source data: %v", err)
	}

	targetData, err := getTableData(targetDB, targetType, tableName, columns, primaryKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to get target data: %v", err)
	}

	sourceKeys := make([]string, 0, len(sourceData))
	for k := range sourceData {
		sourceKeys = append(sourceKeys, k)
	}
	sort.Strings(sourceKeys)

	targetKeys := make([]string, 0, len(targetData))
	for k := range targetData {
		targetKeys = append(targetKeys, k)
	}
	sort.Strings(targetKeys)

	var results []DataDiffResult

	for _, pkKey := range sourceKeys {
		sourceRow := sourceData[pkKey]
		if targetRow, exists := targetData[pkKey]; exists {
			if !rowsEqual(sourceRow, targetRow) {
				updateSQL := generateUpdateSQL(targetType, tableName, sourceRow, columns, primaryKeys)
				if updateSQL != "" {
					pk := extractPrimaryKey(sourceRow, primaryKeys)
					results = append(results, DataDiffResult{
						Type:       "update",
						TableName:  tableName,
						PrimaryKey: pk,
						OldValues:  targetRow,
						NewValues:  sourceRow,
						SQL:        updateSQL,
					})
				}
			}
		} else {
			pk := extractPrimaryKey(sourceRow, primaryKeys)
			results = append(results, DataDiffResult{
				Type:       "insert",
				TableName:  tableName,
				PrimaryKey: pk,
				NewValues:  sourceRow,
				SQL:        generateInsertSQL(targetType, tableName, sourceRow, columns),
			})
		}
	}

	for _, pkKey := range targetKeys {
		targetRow := targetData[pkKey]
		if _, exists := sourceData[pkKey]; !exists {
			pk := extractPrimaryKey(targetRow, primaryKeys)
			results = append(results, DataDiffResult{
				Type:       "delete",
				TableName:  tableName,
				PrimaryKey: pk,
				OldValues:  targetRow,
				SQL:        generateDeleteSQL(targetType, tableName, primaryKeys, pk),
			})
		}
	}

	return results, nil
}

// GetDataSyncSummary returns a summary of data differences for a table
func GetDataSyncSummary(sourceConfig, targetConfig ConnectionConfig, tableName string) (*TableDataInfo, error) {
	diffs, err := CompareTableData(sourceConfig, targetConfig, tableName)
	if err != nil {
		return nil, err
	}

	sourceDB, err := Connect(sourceConfig)
	if err != nil {
		return nil, err
	}
	defer sourceDB.Close()

	targetDB, err := Connect(targetConfig)
	if err != nil {
		return nil, err
	}
	defer targetDB.Close()

	sourceType := sourceConfig.Type
	if sourceType == "" {
		sourceType = MySQL
	}
	targetType := targetConfig.Type
	if targetType == "" {
		targetType = MySQL
	}

	info := &TableDataInfo{TableName: tableName}

	info.PrimaryKeys, _ = getPrimaryKeys(sourceDB, sourceType, sourceConfig.Database, tableName)
	info.Columns, _ = getColumns(sourceDB, sourceType, sourceConfig.Database, tableName)

	_ = sourceDB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(sourceType, tableName))).Scan(&info.SourceCount)
	_ = targetDB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(targetType, tableName))).Scan(&info.TargetCount)

	for _, diff := range diffs {
		switch diff.Type {
		case "insert":
			info.InsertCount++
		case "update":
			info.UpdateCount++
		case "delete":
			info.DeleteCount++
		}
	}

	return info, nil
}

func getPrimaryKeys(db *sql.DB, dbType DBType, database, tableName string) ([]string, error) {
	var query string
	var args []interface{}

	switch dbType {
	case MySQL, "":
		query = `
			SELECT COLUMN_NAME
			FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
			AND CONSTRAINT_NAME = 'PRIMARY'
			ORDER BY ORDINAL_POSITION`
		args = []interface{}{tableName}
	case PostgreSQL:
		query = `
			SELECT a.attname
			FROM pg_index i
			JOIN pg_class c ON c.oid = i.indrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
			WHERE n.nspname = 'public' AND c.relname = $1 AND i.indisprimary
			ORDER BY array_position(i.indkey, a.attnum)`
		args = []interface{}{tableName}
	case SQLite:
		rows, err := db.Query(fmt.Sprintf("PRAGMA table_info('%s')", escapeSQLitePragmaName(tableName)))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		type pkEntry struct {
			name  string
			order int
		}
		var entries []pkEntry
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue interface{}
			if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
				return nil, err
			}
			if pk > 0 {
				entries = append(entries, pkEntry{name: name, order: pk})
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		sort.SliceStable(entries, func(i, j int) bool {
			return entries[i].order < entries[j].order
		})
		pks := make([]string, len(entries))
		for i, e := range entries {
			pks[i] = e.name
		}
		return pks, nil
	case SQLServer:
		query = `
			SELECT c.COLUMN_NAME
			FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
			JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE c
			  ON tc.CONSTRAINT_NAME = c.CONSTRAINT_NAME
			 AND tc.TABLE_SCHEMA = c.TABLE_SCHEMA
			 AND tc.TABLE_NAME = c.TABLE_NAME
			WHERE tc.TABLE_SCHEMA = SCHEMA_NAME()
			  AND tc.TABLE_NAME = @p1
			  AND tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
			ORDER BY c.ORDINAL_POSITION`
		args = []interface{}{tableName}
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pks []string
	for rows.Next() {
		var pk string
		if err := rows.Scan(&pk); err != nil {
			return nil, err
		}
		pks = append(pks, pk)
	}
	return pks, rows.Err()
}

func getColumns(db *sql.DB, dbType DBType, database, tableName string) ([]string, error) {
	var query string
	var args []interface{}

	switch dbType {
	case MySQL, "":
		query = `
			SELECT COLUMN_NAME
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
			ORDER BY ORDINAL_POSITION`
		args = []interface{}{tableName}
	case PostgreSQL:
		query = `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
			ORDER BY ordinal_position`
		args = []interface{}{tableName}
	case SQLite:
		rows, err := db.Query(fmt.Sprintf("PRAGMA table_info('%s')", escapeSQLitePragmaName(tableName)))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var cols []string
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue interface{}
			if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
				return nil, err
			}
			cols = append(cols, name)
		}
		return cols, rows.Err()
	case SQLServer:
		query = `
			SELECT COLUMN_NAME
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA = SCHEMA_NAME() AND TABLE_NAME = @p1
			ORDER BY ORDINAL_POSITION`
		args = []interface{}{tableName}
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

func normalizeScannedValue(val interface{}) interface{} {
	if b, ok := val.([]byte); ok {
		copied := make([]byte, len(b))
		copy(copied, b)
		if utf8.Valid(copied) && !bytes.ContainsRune(copied, 0) {
			return string(copied)
		}
		return copied
	}
	return val
}

func buildPrimaryKeyKey(row map[string]interface{}, primaryKeys []string) string {
	var b strings.Builder
	for _, pk := range primaryKeys {
		val := row[pk]
		if val == nil {
			b.WriteString("N;")
			continue
		}
		var repr string
		if raw, ok := val.([]byte); ok {
			repr = "B:" + hex.EncodeToString(raw)
		} else if t, ok := val.(time.Time); ok {
			repr = "T:" + t.UTC().Format(time.RFC3339Nano)
		} else {
			repr = fmt.Sprintf("V:%v", val)
		}
		fmt.Fprintf(&b, "%d:%s;", len(repr), repr)
	}
	return b.String()
}

func getTableData(db *sql.DB, dbType DBType, tableName string, columns, primaryKeys []string) (map[string]map[string]interface{}, error) {
	quotedCols := make([]string, len(columns))
	for i, col := range columns {
		quotedCols[i] = quoteIdentifier(dbType, col)
	}

	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(quotedCols, ", "), quoteIdentifier(dbType, tableName))
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := make(map[string]map[string]interface{})

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			row[col] = normalizeScannedValue(values[i])
		}

		pkKey := buildPrimaryKeyKey(row, primaryKeys)
		data[pkKey] = row
	}

	return data, rows.Err()
}

func valuesEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if ba, ok := a.([]byte); ok {
		if bb, ok := b.([]byte); ok {
			return bytes.Equal(ba, bb)
		}
		return false
	}
	if _, ok := b.([]byte); ok {
		return false
	}

	if ta, ok := a.(time.Time); ok {
		if tb, ok := b.(time.Time); ok {
			return ta.Equal(tb)
		}
	}

	return fmt.Sprintf("%T:%v", a, a) == fmt.Sprintf("%T:%v", b, b) || fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func rowsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		bv, exists := b[k]
		if !exists || !valuesEqual(v, bv) {
			return false
		}
	}
	return true
}

func extractPrimaryKey(row map[string]interface{}, primaryKeys []string) map[string]interface{} {
	pk := make(map[string]interface{}, len(primaryKeys))
	for _, key := range primaryKeys {
		pk[key] = row[key]
	}
	return pk
}

func generateInsertSQL(dbType DBType, tableName string, row map[string]interface{}, columns []string) string {
	var cols []string
	var vals []string

	for _, col := range columns {
		if val, ok := row[col]; ok {
			cols = append(cols, quoteIdentifier(dbType, col))
			vals = append(vals, escapeValueForDialect(dbType, val))
		}
	}

	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);",
		quoteIdentifier(dbType, tableName),
		strings.Join(cols, ", "),
		strings.Join(vals, ", "))
}

func generateUpdateSQL(dbType DBType, tableName string, row map[string]interface{}, columns, primaryKeys []string) string {
	pkSet := make(map[string]bool, len(primaryKeys))
	for _, pk := range primaryKeys {
		pkSet[pk] = true
	}

	var sets []string
	for _, col := range columns {
		if pkSet[col] {
			continue
		}
		if val, ok := row[col]; ok {
			sets = append(sets, fmt.Sprintf("%s = %s", quoteIdentifier(dbType, col), escapeValueForDialect(dbType, val)))
		}
	}

	if len(sets) == 0 {
		return ""
	}

	var wheres []string
	for _, pk := range primaryKeys {
		wheres = append(wheres, fmt.Sprintf("%s = %s", quoteIdentifier(dbType, pk), escapeValueForDialect(dbType, row[pk])))
	}

	return fmt.Sprintf("UPDATE %s SET %s WHERE %s;",
		quoteIdentifier(dbType, tableName),
		strings.Join(sets, ", "),
		strings.Join(wheres, " AND "))
}

func generateDeleteSQL(dbType DBType, tableName string, primaryKeys []string, pk map[string]interface{}) string {
	var wheres []string
	for _, key := range primaryKeys {
		wheres = append(wheres, fmt.Sprintf("%s = %s", quoteIdentifier(dbType, key), escapeValueForDialect(dbType, pk[key])))
	}
	return fmt.Sprintf("DELETE FROM %s WHERE %s;", quoteIdentifier(dbType, tableName), strings.Join(wheres, " AND "))
}

func escapeValue(val interface{}) string {
	return escapeValueForDialect(MySQL, val)
}

func escapeValueForDialect(dbType DBType, val interface{}) string {
	if val == nil {
		return "NULL"
	}
	switch v := val.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if dbType == PostgreSQL {
			if v {
				return "TRUE"
			}
			return "FALSE"
		}
		if v {
			return "1"
		}
		return "0"
	case time.Time:
		utc := v.UTC()
		if utc.Nanosecond() > 0 {
			return fmt.Sprintf("'%s'", utc.Format("2006-01-02 15:04:05.000000"))
		}
		return fmt.Sprintf("'%s'", utc.Format("2006-01-02 15:04:05"))
	case []byte:
		encoded := hex.EncodeToString(v)
		switch dbType {
		case PostgreSQL:
			return fmt.Sprintf("E'\\\\x%s'", encoded)
		case SQLServer:
			return "0x" + encoded
		default:
			return fmt.Sprintf("X'%s'", encoded)
		}
	default:
		s := fmt.Sprintf("%v", v)
		if dbType == MySQL || dbType == "" {
			s = strings.ReplaceAll(s, "\\", "\\\\")
			s = strings.ReplaceAll(s, "\x00", "\\0")
			s = strings.ReplaceAll(s, "\r", "\\r")
			s = strings.ReplaceAll(s, "\n", "\\n")
		}
		s = strings.ReplaceAll(s, "'", "''")
		return fmt.Sprintf("'%s'", s)
	}
}
