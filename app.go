package main

import (
	"context"

	"skeema-gui/database"
	"skeema-gui/updater"
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

	return database.CompareSchemas(sourceSchema, targetSchema), nil
}

// ExecuteSQL executes SQL on target database
func (a *App) ExecuteSQL(config database.ConnectionConfig, sql string) error {
	db, err := database.Connect(config)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(sql)
	return err
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
