package database

import (
	"database/sql"
	"fmt"
	"strings"
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
	TableName    string   `json:"tableName"`
	PrimaryKeys  []string `json:"primaryKeys"`
	Columns      []string `json:"columns"`
	SourceCount  int      `json:"sourceCount"`
	TargetCount  int      `json:"targetCount"`
	InsertCount  int      `json:"insertCount"`
	UpdateCount  int      `json:"updateCount"`
	DeleteCount  int      `json:"deleteCount"`
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

		// Get primary keys
		info.PrimaryKeys, err = getPrimaryKeys(db, dbType, config.Database, tableName)
		if err != nil {
			return nil, err
		}

		// Get columns
		info.Columns, err = getColumns(db, dbType, config.Database, tableName)
		if err != nil {
			return nil, err
		}

		// Get row count
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
		query = "SHOW TABLES"
	case PostgreSQL:
		query = "SELECT tablename FROM pg_tables WHERE schemaname = 'public'"
	case SQLite:
		query = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
	case SQLServer:
		query = "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE = 'BASE TABLE'"
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
	return tableNames, nil
}

// quoteIdentifier quotes an identifier based on database type
func quoteIdentifier(dbType DBType, name string) string {
	switch dbType {
	case MySQL, "":
		return fmt.Sprintf("`%s`", name)
	case PostgreSQL:
		return fmt.Sprintf("\"%s\"", name)
	case SQLite:
		return fmt.Sprintf("\"%s\"", name)
	case SQLServer:
		return fmt.Sprintf("[%s]", name)
	default:
		return fmt.Sprintf("`%s`", name)
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

	// Get primary keys
	primaryKeys, err := getPrimaryKeys(sourceDB, sourceType, sourceConfig.Database, tableName)
	if err != nil {
		return nil, err
	}
	if len(primaryKeys) == 0 {
		return nil, fmt.Errorf("table %s has no primary key", tableName)
	}

	// Get columns
	columns, err := getColumns(sourceDB, sourceType, sourceConfig.Database, tableName)
	if err != nil {
		return nil, err
	}

	var results []DataDiffResult

	// Get source data
	sourceData, err := getTableData(sourceDB, sourceType, tableName, columns, primaryKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to get source data: %v", err)
	}

	// Get target data
	targetData, err := getTableData(targetDB, targetType, tableName, columns, primaryKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to get target data: %v", err)
	}

	// Find inserts and updates
	for pkKey, sourceRow := range sourceData {
		if targetRow, exists := targetData[pkKey]; exists {
			// Check for updates
			if !rowsEqual(sourceRow, targetRow) {
				pk := extractPrimaryKey(sourceRow, primaryKeys)
				results = append(results, DataDiffResult{
					Type:       "update",
					TableName:  tableName,
					PrimaryKey: pk,
					OldValues:  targetRow,
					NewValues:  sourceRow,
					SQL:        generateUpdateSQL(targetType, tableName, sourceRow, primaryKeys),
				})
			}
		} else {
			// Insert
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

	// Find deletes
	for pkKey, targetRow := range targetData {
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

	// Get primary keys
	info.PrimaryKeys, _ = getPrimaryKeys(sourceDB, sourceType, sourceConfig.Database, tableName)
	info.Columns, _ = getColumns(sourceDB, sourceType, sourceConfig.Database, tableName)

	// Get counts
	sourceDB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(sourceType, tableName))).Scan(&info.SourceCount)
	targetDB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(targetType, tableName))).Scan(&info.TargetCount)

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
			WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
			AND CONSTRAINT_NAME = 'PRIMARY'
			ORDER BY ORDINAL_POSITION`
		args = []interface{}{database, tableName}
	case PostgreSQL:
		query = `
			SELECT a.attname
			FROM pg_index i
			JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
			WHERE i.indrelid = $1::regclass AND i.indisprimary
			ORDER BY array_position(i.indkey, a.attnum)`
		args = []interface{}{tableName}
	case SQLite:
		// SQLite uses PRAGMA, handled separately
		rows, err := db.Query(fmt.Sprintf("PRAGMA table_info('%s')", tableName))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var pks []string
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue interface{}
			if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
				return nil, err
			}
			if pk > 0 {
				pks = append(pks, name)
			}
		}
		return pks, nil
	case SQLServer:
		query = `
			SELECT c.COLUMN_NAME
			FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
			JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE c ON tc.CONSTRAINT_NAME = c.CONSTRAINT_NAME
			WHERE tc.TABLE_NAME = @p1 AND tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
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
	return pks, nil
}

func getColumns(db *sql.DB, dbType DBType, database, tableName string) ([]string, error) {
	var query string
	var args []interface{}

	switch dbType {
	case MySQL, "":
		query = `
			SELECT COLUMN_NAME
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
			ORDER BY ORDINAL_POSITION`
		args = []interface{}{database, tableName}
	case PostgreSQL:
		query = `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
			ORDER BY ordinal_position`
		args = []interface{}{tableName}
	case SQLite:
		rows, err := db.Query(fmt.Sprintf("PRAGMA table_info('%s')", tableName))
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
		return cols, nil
	case SQLServer:
		query = `
			SELECT COLUMN_NAME
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_NAME = @p1
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
	return cols, nil
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

		row := make(map[string]interface{})
		var pkParts []string
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}

		// Build primary key string
		for _, pk := range primaryKeys {
			pkParts = append(pkParts, fmt.Sprintf("%v", row[pk]))
		}
		pkKey := strings.Join(pkParts, "|")
		data[pkKey] = row
	}

	return data, nil
}

func rowsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if fmt.Sprintf("%v", v) != fmt.Sprintf("%v", b[k]) {
			return false
		}
	}
	return true
}

func extractPrimaryKey(row map[string]interface{}, primaryKeys []string) map[string]interface{} {
	pk := make(map[string]interface{})
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
			vals = append(vals, escapeValue(val))
		}
	}

	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);",
		quoteIdentifier(dbType, tableName),
		strings.Join(cols, ", "),
		strings.Join(vals, ", "))
}

func generateUpdateSQL(dbType DBType, tableName string, row map[string]interface{}, primaryKeys []string) string {
	var sets []string
	var wheres []string

	for col, val := range row {
		isPK := false
		for _, pk := range primaryKeys {
			if col == pk {
				isPK = true
				break
			}
		}
		if !isPK {
			sets = append(sets, fmt.Sprintf("%s = %s", quoteIdentifier(dbType, col), escapeValue(val)))
		}
	}

	for _, pk := range primaryKeys {
		wheres = append(wheres, fmt.Sprintf("%s = %s", quoteIdentifier(dbType, pk), escapeValue(row[pk])))
	}

	return fmt.Sprintf("UPDATE %s SET %s WHERE %s;",
		quoteIdentifier(dbType, tableName),
		strings.Join(sets, ", "),
		strings.Join(wheres, " AND "))
}

func generateDeleteSQL(dbType DBType, tableName string, primaryKeys []string, pk map[string]interface{}) string {
	var wheres []string
	for _, key := range primaryKeys {
		wheres = append(wheres, fmt.Sprintf("%s = %s", quoteIdentifier(dbType, key), escapeValue(pk[key])))
	}
	return fmt.Sprintf("DELETE FROM %s WHERE %s;", quoteIdentifier(dbType, tableName), strings.Join(wheres, " AND "))
}

func escapeValue(val interface{}) string {
	if val == nil {
		return "NULL"
	}
	switch v := val.(type) {
	case int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "1"
		}
		return "0"
	default:
		s := fmt.Sprintf("%v", v)
		s = strings.ReplaceAll(s, "'", "''")
		return fmt.Sprintf("'%s'", s)
	}
}
