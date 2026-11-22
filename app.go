package main

import (
	"context"
	"strings"

	"syncforge/database"
	"syncforge/updater"
)

// App struct
type App struct {
	ctx             context.Context
	connectionStore *database.ConnectionStore
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	store, err := database.NewConnectionStore()
	if err == nil {
		a.connectionStore = store
	}
}

// TestConnection tests database connection
func (a *App) TestConnection(config database.ConnectionConfig) error {
	return database.TestConnection(config)
}

// GetDatabases returns list of databases
func (a *App) GetDatabases(config database.ConnectionConfig) ([]string, error) {
	return database.GetDatabases(config)
}

// GetSchema retrieves database schema
func (a *App) GetSchema(config database.ConnectionConfig) (*database.SchemaInfo, error) {
	return database.GetSchema(config)
}

// CompareSchemas compares two database schemas
func (a *App) CompareSchemas(source, target database.ConnectionConfig) ([]database.DiffResult, error) {
	sourceSchema, err := database.GetSchema(source)
	if err != nil {
		return nil, err
	}

	targetSchema, err := database.GetSchema(target)
	if err != nil {
		return nil, err
	}

	dialect := target.Type
	if dialect == "" {
		dialect = database.MySQL
	}

	return database.CompareSchemasWithDialect(sourceSchema, targetSchema, dialect), nil
}

// ExecuteSQL executes SQL on target database inside a transaction
func (a *App) ExecuteSQL(config database.ConnectionConfig, sqlStmt string) error {
	db, err := database.Connect(config)
	if err != nil {
		return err
	}
	defer db.Close()

	statements := splitSQLStatements(sqlStmt)
	if len(statements) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// splitSQLStatements splits SQL string into individual executable statements.
func splitSQLStatements(sqlStmt string) []string {
	var statements []string
	var current strings.Builder

	runes := []rune(sqlStmt)
	n := len(runes)

	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < n; i++ {
		c := runes[i]

		if inLineComment {
			if c == '\n' {
				inLineComment = false
				current.WriteRune(c)
			}
			continue
		}

		if inBlockComment {
			if c == '*' && i+1 < n && runes[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		if inSingleQuote {
			current.WriteRune(c)
			if c == '\\' && i+1 < n {
				current.WriteRune(runes[i+1])
				i++
				continue
			}
			if c == '\'' {
				if i+1 < n && runes[i+1] == '\'' {
					current.WriteRune(runes[i+1])
					i++
					continue
				}
				inSingleQuote = false
			}
			continue
		}

		if inDoubleQuote {
			current.WriteRune(c)
			if c == '"' {
				if i+1 < n && runes[i+1] == '"' {
					current.WriteRune(runes[i+1])
					i++
					continue
				}
				inDoubleQuote = false
			}
			continue
		}

		if inBacktick {
			current.WriteRune(c)
			if c == '`' {
				if i+1 < n && runes[i+1] == '`' {
					current.WriteRune(runes[i+1])
					i++
					continue
				}
				inBacktick = false
			}
			continue
		}

		// Comments
		if c == '-' && i+1 < n && runes[i+1] == '-' {
			inLineComment = true
			i++
			continue
		}
		if c == '/' && i+1 < n && runes[i+1] == '*' {
			inBlockComment = true
			i++
			continue
		}

		switch c {
		case '\'':
			inSingleQuote = true
			current.WriteRune(c)
		case '"':
			inDoubleQuote = true
			current.WriteRune(c)
		case '`':
			inBacktick = true
			current.WriteRune(c)
		case ';':
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
		default:
			current.WriteRune(c)
		}
	}

	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

// GetTablesForSync returns tables available for data sync
func (a *App) GetTablesForSync(config database.ConnectionConfig) ([]database.TableDataInfo, error) {
	return database.GetTablesForSync(config)
}

// CompareTableData compares data between source and target tables
func (a *App) CompareTableData(source, target database.ConnectionConfig, tableName string) ([]database.DataDiffResult, error) {
	return database.CompareTableData(source, target, tableName)
}

// GetDataSyncSummary returns sync summary for a table
func (a *App) GetDataSyncSummary(source, target database.ConnectionConfig, tableName string) (*database.TableDataInfo, error) {
	return database.GetDataSyncSummary(source, target, tableName)
}

// CreateDatabase creates a new database
func (a *App) CreateDatabase(config database.ConnectionConfig, dbName, charset, collation string) error {
	return database.CreateDatabase(config, dbName, charset, collation)
}

// GetTableStructure retrieves detailed table structure
func (a *App) GetTableStructure(config database.ConnectionConfig, tableName string) (*database.TableInfo, error) {
	return database.GetTableStructure(config, tableName)
}

// GetTableData retrieves paginated table data
func (a *App) GetTableData(config database.ConnectionConfig, tableName string, page, pageSize int) (*database.TableDataResult, error) {
	return database.GetTableData(config, tableName, page, pageSize)
}

// GetAllTables returns all tables with basic info
func (a *App) GetAllTables(config database.ConnectionConfig) ([]database.TableDataInfo, error) {
	return database.GetAllTables(config)
}

// GetSavedConnections returns all saved connections
func (a *App) GetSavedConnections() []database.SavedConnection {
	if a.connectionStore == nil {
		return []database.SavedConnection{}
	}
	return a.connectionStore.GetAll()
}

// SaveConnection saves a connection configuration
func (a *App) SaveConnection(name string, config database.ConnectionConfig) error {
	if a.connectionStore == nil {
		return nil
	}
	return a.connectionStore.Save(database.SavedConnection{
		Name:   name,
		Config: config,
	})
}

// DeleteConnection deletes a saved connection
func (a *App) DeleteConnection(name string) error {
	if a.connectionStore == nil {
		return nil
	}
	return a.connectionStore.Delete(name)
}

// GetAppVersion returns the current app version
func (a *App) GetAppVersion() string {
	return updater.GetCurrentVersion()
}

// CheckForUpdates checks for available updates
func (a *App) CheckForUpdates() (*updater.UpdateInfo, error) {
	return updater.CheckForUpdates()
}

// OpenReleaseURL opens the release page in browser
func (a *App) OpenReleaseURL(url string) error {
	return updater.OpenReleaseURL(url)
}

// DownloadAndApplyUpdate downloads and applies the update
func (a *App) DownloadAndApplyUpdate(downloadURL string) error {
	filePath, err := updater.DownloadUpdate(downloadURL, nil)
	if err != nil {
		return err
	}

	return updater.ApplyUpdate(filePath)
}
