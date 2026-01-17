# Skeema GUI

A cross-platform MySQL schema and data synchronization tool with a modern GUI.

[中文文档](#中文文档)

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

## Screenshots

| Schema Compare | Data Sync |
|----------------|-----------|
| Compare table structures | Sync data between databases |

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

Use the **Save** button to store frequently used connections.

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

---

# 中文文档

一个跨平台的 MySQL 数据库结构和数据同步工具，具有现代化的图形界面。

## 功能特性

- **结构比对** - 比较源数据库和目标数据库的表结构差异
- **数据同步** - 支持选择性的 INSERT/UPDATE/DELETE 数据同步
- **表设计器** - 可视化设计和创建新表
- **表浏览器** - 浏览表结构和数据，支持分页
- **数据库管理** - 创建新数据库，支持字符集和排序规则选项
- **连接管理** - 保存和管理多个数据库连接
- **跨平台** - 支持 macOS、Windows 和 Linux

## 安装

### 下载预编译版本

从 [Releases](https://github.com/nanablast/skeema-gui/releases) 下载适合你平台的最新版本。

### 从源码构建

**前置要求:**
- Go 1.21+
- Node.js 18+
- Wails CLI

```bash
# 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 克隆仓库
git clone https://github.com/nanablast/skeema-gui.git
cd skeema-gui

# 构建当前平台版本
wails build

# 或以开发模式运行
wails dev
```

## 使用方法

### 1. 连接数据库

输入源数据库和目标数据库的连接信息：
- 主机、端口、用户名、密码
- 选择或创建数据库

使用 **保存** 按钮存储常用连接。

### 2. 结构比对

1. 选择 **Schema Compare** 标签
2. 点击 **Compare Schemas** 分析差异
3. 查看生成的 SQL 语句
4. 单独执行或批量执行

### 3. 数据同步

1. 选择 **Data Sync** 标签
2. 点击 **Refresh** 加载表（仅支持有主键的表）
3. 选择表并点击 **Compare Data**
4. 选择要同步的操作类型（INSERT/UPDATE/DELETE）
5. 执行同步

### 4. 表设计器

1. 选择 **Table Designer** 标签
2. 定义表名、列和索引
3. 预览生成的 CREATE TABLE SQL
4. 在目标数据库上创建表

### 5. 表浏览器

1. 选择 **Table Browser** 标签
2. 在源数据库和目标数据库之间切换
3. 浏览表结构和数据

## 技术栈

- **后端:** Go + [Wails](https://wails.io/)
- **前端:** Vue 3 + TypeScript + Vite
- **数据库:** MySQL (go-sql-driver/mysql)

## 配置文件

保存的连接存储在：
- **macOS/Linux:** `~/.skeema-gui/connections.json`
- **Windows:** `C:\Users\{用户名}\.skeema-gui\connections.json`

## 许可证

MIT License
