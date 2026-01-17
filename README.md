# Skeema GUI

A cross-platform MySQL schema and data synchronization tool with a modern GUI.

[中文文档](README.zh-CN.md)

![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-blue)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![Vue](https://img.shields.io/badge/Vue-3-4FC08D?logo=vue.js)
![License](https://img.shields.io/badge/license-MIT-green)

## Features

- **Schema Comparison** - Compare table structures between source and target databases
- **Data Synchronization** - Sync data with selective INSERT/UPDATE/DELETE operations
- **Table Designer** - Visually design and create new tables
- **Table Browser** - Browse table structures and data with pagination
- **Database Management** - Create new databases with charset/collation options
- **Connection Manager** - Save and manage multiple database connections
- **Cross-Platform** - Runs on macOS, Windows, and Linux

## Installation

### Download Pre-built Binaries

Download the latest release for your platform from [Releases](https://github.com/nanablast/skeema-gui/releases).

### Build from Source

**Prerequisites:**
- Go 1.21+
- Node.js 18+
- Wails CLI

```bash
# Install Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Clone the repository
git clone https://github.com/nanablast/skeema-gui.git
cd skeema-gui

# Build for current platform
wails build

# Or run in development mode
wails dev
```

**Build for specific platforms:**

```bash
wails build -platform darwin/universal    # macOS (Intel + Apple Silicon)
wails build -platform windows/amd64       # Windows 64-bit
wails build -platform linux/amd64         # Linux 64-bit
```

## Usage

### 1. Connect to Databases

Enter connection details for both Source and Target databases:
- Host, Port, User, Password
- Select or create a database

Use the 💾 button to store frequently used connections.

### 2. Schema Compare

1. Select the **Schema Compare** tab
2. Click **Compare Schemas** to analyze differences
3. Review the generated SQL statements
4. Execute individual statements or all at once

### 3. Data Sync

1. Select the **Data Sync** tab
2. Click **Refresh** to load tables (only tables with primary keys are supported)
3. Select a table and click **Compare Data**
4. Choose which operations to sync (INSERT/UPDATE/DELETE)
5. Execute the synchronization

### 4. Table Designer

1. Select the **Table Designer** tab
2. Define table name, columns, and indexes
3. Preview the generated CREATE TABLE SQL
4. Create the table on the target database

### 5. Table Browser

1. Select the **Table Browser** tab
2. Switch between Source and Target databases
3. Browse table structures and data

## Tech Stack

- **Backend:** Go + [Wails](https://wails.io/)
- **Frontend:** Vue 3 + TypeScript + Vite
- **Database:** MySQL (go-sql-driver/mysql)

## Configuration

Saved connections are stored in:
- **macOS/Linux:** `~/.skeema-gui/connections.json`
- **Windows:** `C:\Users\{username}\.skeema-gui\connections.json`

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License
