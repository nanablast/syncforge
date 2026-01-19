package database

import (
	"fmt"
	"strings"
)

// TableRowData holds a row of table data
type TableRowData struct {
	Values map[string]interface{} `json:"values"`
}

// TableDataResult holds paginated table data
type TableDataResult struct {
	Columns    []string       `json:"columns"`
	Rows       []TableRowData `json:"rows"`
	TotalCount int            `json:"totalCount"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
}

// GetTableData retrieves paginated table data
func GetTableData(config ConnectionConfig, tableName string, page, pageSize int) (*TableDataResult, error) {
	db, err := Connect(config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	dbType := config.Type
	if dbType == "" {
		dbType = MySQL
	}

	// Get total count
	var totalCount int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(dbType, tableName))
	err = db.QueryRow(countQuery).Scan(&totalCount)
	if err != nil {
		return nil, err
	}

	// Get columns
	columns, err := getColumns(db, dbType, config.Database, tableName)
	if err != nil {
		return nil, err
	}

	// Calculate offset
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	// Build query with database-specific pagination
	quotedCols := make([]string, len(columns))
	for i, col := range columns {
		quotedCols[i] = quoteIdentifier(dbType, col)
	}

	var query string
	switch dbType {
	case SQLServer:
		// SQL Server uses OFFSET FETCH
		query = fmt.Sprintf("SELECT %s FROM %s ORDER BY (SELECT NULL) OFFSET %d ROWS FETCH NEXT %d ROWS ONLY",
			strings.Join(quotedCols, ", "), quoteIdentifier(dbType, tableName), offset, pageSize)
	default:
		// MySQL, PostgreSQL, SQLite use LIMIT OFFSET
		query = fmt.Sprintf("SELECT %s FROM %s LIMIT %d OFFSET %d",
			strings.Join(quotedCols, ", "), quoteIdentifier(dbType, tableName), pageSize, offset)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resultRows []TableRowData
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		rowData := TableRowData{Values: make(map[string]interface{})}
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				rowData.Values[col] = string(b)
			} else {
				rowData.Values[col] = val
			}
		}
		resultRows = append(resultRows, rowData)
	}

	return &TableDataResult{
		Columns:    columns,
		Rows:       resultRows,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// GetTableStructure retrieves detailed table structure
func GetTableStructure(config ConnectionConfig, tableName string) (*TableInfo, error) {
	db, err := Connect(config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	switch config.Type {
	case MySQL, "":
		return getMySQLTableInfo(db, tableName)
	case PostgreSQL:
		return getPostgreSQLTableInfo(db, tableName)
	case SQLite:
		return getSQLiteTableInfo(db, tableName)
	case SQLServer:
		return getSQLServerTableInfo(db, tableName)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

// GetAllTables returns all tables with basic info
func GetAllTables(config ConnectionConfig) ([]TableDataInfo, error) {
	return GetTablesForSync(config)
}
