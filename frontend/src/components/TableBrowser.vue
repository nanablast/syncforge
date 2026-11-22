<template>
  <div class="table-browser">
    <div class="browser-header">
      <div class="header-left">
        <h3>{{ t('browser.title') }}</h3>
        <p class="hint">{{ t('browser.hint') }}</p>
      </div>
      <div class="target-switch">
        <button
          class="switch-btn"
          :class="{ active: browserTarget === 'source' }"
          @click="$emit('switch-target', 'source')"
        >
          {{ t('browser.source') }}
        </button>
        <button
          class="switch-btn"
          :class="{ active: browserTarget === 'target' }"
          @click="$emit('switch-target', 'target')"
        >
          {{ t('browser.target') }}
        </button>
      </div>
    </div>

    <div class="browser-layout">
      <!-- Table List Sidebar -->
      <div class="table-sidebar">
        <div class="sidebar-header">
          <span>{{ t('browser.tables') }}</span>
          <button class="btn-refresh" @click="loadTables" :disabled="loadingTables">
            {{ loadingTables ? '...' : t('browser.refresh') }}
          </button>
        </div>
        <div class="table-list">
          <div
            v-for="table in tables"
            :key="table.tableName"
            class="table-item"
            :class="{ selected: selectedTable === table.tableName }"
            @click="selectTable(table.tableName)"
          >
            <span class="table-icon">T</span>
            <span class="table-name">{{ table.tableName }}</span>
            <span class="row-count">{{ table.sourceCount }}</span>
          </div>
          <div v-if="tables.length === 0 && !loadingTables" class="no-tables">
            {{ t('browser.noTables') }}
          </div>
        </div>
      </div>

      <!-- Main Content -->
      <div class="main-content" v-if="selectedTable">
        <!-- Tabs -->
        <div class="content-tabs">
          <button
            class="content-tab"
            :class="{ active: viewMode === 'structure' }"
            @click="viewMode = 'structure'"
          >
            {{ t('browser.structure') }}
          </button>
          <button
            class="content-tab"
            :class="{ active: viewMode === 'data' }"
            @click="viewMode = 'data'; loadData()"
          >
            {{ t('browser.data') }}
          </button>
        </div>

        <!-- Structure View -->
        <div class="structure-view" v-if="viewMode === 'structure'">
          <div class="section" v-if="tableStructure">
            <h4>{{ t('browser.columns') }}</h4>
            <table class="structure-table">
              <thead>
                <tr>
                  <th>{{ t('browser.name') }}</th>
                  <th>{{ t('browser.type') }}</th>
                  <th>{{ t('browser.nullable') }}</th>
                  <th>{{ t('browser.key') }}</th>
                  <th>{{ t('browser.default') }}</th>
                  <th>{{ t('browser.extra') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="col in tableStructure.columns" :key="col.name">
                  <td class="col-name">{{ col.name }}</td>
                  <td class="col-type">{{ col.type }}</td>
                  <td>{{ col.nullable }}</td>
                  <td><span v-if="col.key" class="key-badge">{{ col.key }}</span></td>
                  <td class="col-default">{{ col.default ?? 'NULL' }}</td>
                  <td class="col-extra">{{ col.extra }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="section" v-if="tableStructure && tableStructure.indexes.length > 0">
            <h4>{{ t('browser.indexes') }}</h4>
            <table class="structure-table">
              <thead>
                <tr>
                  <th>{{ t('browser.name') }}</th>
                  <th>{{ t('browser.column') }}</th>
                  <th>{{ t('browser.unique') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(idx, i) in tableStructure.indexes" :key="i">
                  <td>{{ idx.name }}</td>
                  <td>{{ idx.column }}</td>
                  <td>{{ idx.nonUnique === 0 ? t('browser.yes') : t('browser.no') }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="section" v-if="tableStructure">
            <h4>{{ t('browser.createTableSQL') }}</h4>
            <pre class="sql-preview"><code>{{ tableStructure.createSql }}</code></pre>
          </div>
        </div>

        <!-- Data View -->
        <div class="data-view" v-if="viewMode === 'data'">
          <div v-if="loadingData" class="loading">{{ t('browser.loadingData') }}</div>
          <div v-else-if="tableData">
            <div class="data-info">
              {{ t('browser.showing') }} {{ tableData.rows.length }} {{ t('browser.of') }} {{ tableData.totalCount }}
              ({{ t('browser.page') }} {{ tableData.page }})
            </div>
            <div class="data-table-wrapper">
              <table class="data-table">
                <thead>
                  <tr>
                    <th v-for="col in tableData.columns" :key="col">{{ col }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(row, i) in tableData.rows" :key="i">
                    <td v-for="col in tableData.columns" :key="col">
                      {{ formatValue(row.values[col]) }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div class="pagination">
              <button
                class="btn btn-page"
                @click="changePage(-1)"
                :disabled="currentPage <= 1"
              >{{ t('browser.prev') }}</button>
              <span class="page-info">{{ t('browser.page') }} {{ currentPage }} {{ t('browser.of') }} {{ totalPages }}</span>
              <button
                class="btn btn-page"
                @click="changePage(1)"
                :disabled="currentPage >= totalPages"
              >{{ t('browser.next') }}</button>
            </div>
          </div>
        </div>
      </div>

      <!-- Empty State -->
      <div class="main-content empty" v-else>
        <div class="empty-message">
          {{ t('browser.selectTableHint') }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { GetAllTables, GetTableStructure, GetTableData } from '../../wailsjs/go/main/App'
import { database } from '../../wailsjs/go/models'

type ConnectionConfig = database.ConnectionConfig
type TableDataInfo = database.TableDataInfo
type TableInfo = database.TableInfo
type TableDataResult = database.TableDataResult

const { t } = useI18n()

const props = defineProps<{
  config: ConnectionConfig
  connected: boolean
  browserTarget: 'source' | 'target'
}>()

const emit = defineEmits<{
  'switch-target': [target: 'source' | 'target']
}>()

// State
const tables = ref<TableDataInfo[]>([])
const loadingTables = ref(false)
const selectedTable = ref<string | null>(null)
const viewMode = ref<'structure' | 'data'>('structure')
const tableStructure = ref<TableInfo | null>(null)
const tableData = ref<TableDataResult | null>(null)
const loadingData = ref(false)
const currentPage = ref(1)
const pageSize = 50

// Computed
const isSourceReady = computed(() => {
  if (!props.connected) return false
  if (props.config.type === 'sqlite') {
    return Boolean(props.config.filePath)
  }
  return Boolean(props.config.database)
})

const totalPages = computed(() => {
  if (!tableData.value) return 1
  return Math.max(1, Math.ceil(tableData.value.totalCount / pageSize))
})

// Watch for connection / database changes
watch([isSourceReady, () => props.config.database, () => props.config.filePath], ([ready]) => {
  selectedTable.value = null
  tableStructure.value = null
  tableData.value = null
  if (ready) {
    loadTables()
  } else {
    tables.value = []
  }
})

// Watch for target switch
watch(() => props.browserTarget, () => {
  selectedTable.value = null
  tableStructure.value = null
  tableData.value = null
  if (isSourceReady.value) {
    loadTables()
  } else {
    tables.value = []
  }
})

// Methods
async function loadTables() {
  if (!isSourceReady.value) {
    tables.value = []
    return
  }

  loadingTables.value = true
  try {
    tables.value = await GetAllTables(props.config) || []
  } catch (e: any) {
    console.error('Failed to load tables:', e)
  } finally {
    loadingTables.value = false
  }
}

async function selectTable(tableName: string) {
  selectedTable.value = tableName
  viewMode.value = 'structure'
  currentPage.value = 1
  tableData.value = null

  try {
    tableStructure.value = await GetTableStructure(props.config, tableName)
  } catch (e: any) {
    console.error('Failed to load table structure:', e)
  }
}

async function loadData() {
  if (!selectedTable.value) return

  loadingData.value = true
  try {
    tableData.value = await GetTableData(props.config, selectedTable.value, currentPage.value, pageSize)
  } catch (e: any) {
    console.error('Failed to load table data:', e)
  } finally {
    loadingData.value = false
  }
}

function changePage(delta: number) {
  currentPage.value += delta
  loadData()
}

function formatValue(val: any): string {
  if (val === null || val === undefined) return 'NULL'
  if (typeof val === 'object') return JSON.stringify(val)
  const str = String(val)
  if (str.length > 100) return str.substring(0, 100) + '...'
  return str
}

// Initial load
if (isSourceReady.value) {
  loadTables()
}
</script>

<style scoped>
.table-browser {
  background: #16213e;
  border-radius: 10px;
  padding: 20px;
  margin-top: 20px;
}

.browser-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 15px;
}

.header-left h3 {
  color: #4fc3f7;
  margin-bottom: 5px;
}

.hint {
  color: #888;
  font-size: 13px;
}

.target-switch {
  display: flex;
  gap: 5px;
  background: #0f0f23;
  border-radius: 6px;
  padding: 4px;
}

.switch-btn {
  padding: 6px 16px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: #888;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}

.switch-btn:hover {
  color: #4fc3f7;
}

.switch-btn.active {
  background: #4fc3f7;
  color: #1a1a2e;
  font-weight: bold;
}

.browser-layout {
  display: flex;
  gap: 20px;
  min-height: 400px;
}

/* Sidebar */
.table-sidebar {
  width: 220px;
  flex-shrink: 0;
  background: #0f0f23;
  border-radius: 8px;
  overflow: hidden;
}

.sidebar-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  background: rgba(255, 255, 255, 0.05);
  font-size: 12px;
  color: #888;
  font-weight: bold;
}

.btn-refresh {
  padding: 4px 8px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: #4fc3f7;
  cursor: pointer;
}

.btn-refresh:hover:not(:disabled) {
  background: rgba(79, 195, 247, 0.1);
}

.table-list {
  max-height: 350px;
  overflow-y: auto;
}

.table-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  cursor: pointer;
  border-bottom: 1px solid #1a1a2e;
  transition: background 0.2s;
}

.table-item:hover {
  background: rgba(79, 195, 247, 0.1);
}

.table-item.selected {
  background: rgba(79, 195, 247, 0.2);
  border-left: 3px solid #4fc3f7;
}

.table-icon {
  font-size: 12px;
}

.table-item .table-name {
  flex: 1;
  font-size: 13px;
  color: #eee;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.table-item .row-count {
  font-size: 11px;
  color: #666;
}

.no-tables {
  padding: 20px;
  text-align: center;
  color: #666;
  font-size: 13px;
}

/* Main Content */
.main-content {
  flex: 1;
  min-width: 0;
}

.main-content.empty {
  display: flex;
  align-items: center;
  justify-content: center;
  background: #0f0f23;
  border-radius: 8px;
}

.empty-message {
  color: #666;
  font-size: 14px;
}

.content-tabs {
  display: flex;
  gap: 5px;
  margin-bottom: 15px;
}

.content-tab {
  padding: 8px 16px;
  border: none;
  border-radius: 5px 5px 0 0;
  background: #0f0f23;
  color: #888;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s;
}

.content-tab:hover {
  color: #4fc3f7;
}

.content-tab.active {
  background: #1a4a7a;
  color: #4fc3f7;
}

/* Structure View */
.structure-view .section {
  margin-bottom: 20px;
}

.structure-view h4 {
  color: #fff;
  margin-bottom: 10px;
  font-size: 14px;
}

.structure-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.structure-table th,
.structure-table td {
  padding: 8px 10px;
  text-align: left;
  border-bottom: 1px solid #333;
}

.structure-table th {
  background: rgba(255, 255, 255, 0.05);
  color: #888;
  font-weight: normal;
}

.structure-table td {
  color: #ccc;
}

.structure-table .col-name {
  color: #4fc3f7;
  font-weight: bold;
}

.structure-table .col-type {
  color: #81c784;
}

.structure-table .col-extra {
  color: #ff9800;
}

.key-badge {
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 11px;
  background: rgba(79, 195, 247, 0.2);
  color: #4fc3f7;
}

.sql-preview {
  background: #0a0a1a;
  padding: 15px;
  border-radius: 6px;
  overflow-x: auto;
  max-height: 200px;
}

.sql-preview code {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 12px;
  color: #81c784;
  white-space: pre-wrap;
}

/* Data View */
.data-view {
  background: #0f0f23;
  border-radius: 8px;
  padding: 15px;
}

.loading {
  text-align: center;
  color: #888;
  padding: 40px;
}

.data-info {
  font-size: 12px;
  color: #888;
  margin-bottom: 10px;
}

.data-table-wrapper {
  overflow-x: auto;
  max-height: 300px;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.data-table th,
.data-table td {
  padding: 6px 10px;
  text-align: left;
  border-bottom: 1px solid #333;
  white-space: nowrap;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.data-table th {
  background: rgba(255, 255, 255, 0.05);
  color: #888;
  position: sticky;
  top: 0;
}

.data-table td {
  color: #ccc;
}

.data-table tr:hover td {
  background: rgba(79, 195, 247, 0.05);
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 15px;
  margin-top: 15px;
}

.page-info {
  font-size: 13px;
  color: #888;
}

.btn-page {
  padding: 6px 12px;
  border: 1px solid #333;
  border-radius: 4px;
  background: transparent;
  color: #4fc3f7;
  cursor: pointer;
  font-size: 12px;
}

.btn-page:hover:not(:disabled) {
  background: rgba(79, 195, 247, 0.1);
}

.btn-page:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
