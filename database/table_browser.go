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
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	db, err := Connect(config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	dbType := config.Type
	if dbType == "" {
		dbType = MySQL
	}

	var totalCount int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(dbType, tableName))
	err = db.QueryRow(countQuery).Scan(&totalCount)
	if err != nil {
		return nil, err
	}

	columns, err := getColumns(db, dbType, config.Database, tableName)
	if err != nil {
		return nil, err
	}

	if len(columns) == 0 {
		return &TableDataResult{
			Columns:    []string{},
			Rows:       []TableRowData{},
			TotalCount: totalCount,
			Page:       page,
			PageSize:   pageSize,
		}, nil
	}

	offset := (page - 1) * pageSize

	quotedCols := make([]string, len(columns))
	for i, col := range columns {
		quotedCols[i] = quoteIdentifier(dbType, col)
	}

	orderCols, _ := getPrimaryKeys(db, dbType, config.Database, tableName)
	if len(orderCols) == 0 {
		orderCols = []string{columns[0]}
	}
	quotedOrderCols := make([]string, len(orderCols))
	for i, col := range orderCols {
		quotedOrderCols[i] = quoteIdentifier(dbType, col)
	}
	orderBy := strings.Join(quotedOrderCols, ", ")

	var query string
	switch dbType {
	case SQLServer:
		query = fmt.Sprintf("SELECT %s FROM %s ORDER BY %s OFFSET %d ROWS FETCH NEXT %d ROWS ONLY",
			strings.Join(quotedCols, ", "), quoteIdentifier(dbType, tableName), orderBy, offset, pageSize)
	default:
		query = fmt.Sprintf("SELECT %s FROM %s ORDER BY %s LIMIT %d OFFSET %d",
			strings.Join(quotedCols, ", "), quoteIdentifier(dbType, tableName), orderBy, pageSize, offset)
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

		rowData := TableRowData{Values: make(map[string]interface{}, len(columns))}
		for i, col := range columns {
			norm := normalizeScannedValue(values[i])
			if b, ok := norm.([]byte); ok {
				rowData.Values[col] = fmt.Sprintf("<BLOB %d bytes>", len(b))
			} else {
				rowData.Values[col] = norm
			}
		}
		resultRows = append(resultRows, rowData)
	}
	if err := rows.Err(); err != nil {
		return nil, err
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
