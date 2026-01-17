<template>
  <div class="data-sync">
    <div class="sync-header">
      <h3>📊 Data Sync</h3>
      <p class="hint">Compare and sync data from Source to Target</p>
    </div>

    <div class="sync-layout">
      <!-- Left Panel: Table Selection -->
      <div class="left-panel">
        <div class="panel-header">
          <h4>Select Tables</h4>
          <button class="btn btn-refresh" @click="loadTables" :disabled="loadingTables">
            {{ loadingTables ? 'Loading...' : '🔄 Refresh' }}
          </button>
        </div>

        <div class="select-all-row" v-if="selectableTables.length > 0">
          <label class="checkbox-label select-all">
            <input type="checkbox" :checked="isAllSelected" @change="toggleSelectAll" />
            <span>Select All ({{ selectableTables.length }})</span>
          </label>
        </div>

        <div class="table-list" v-if="tables.length > 0">
          <div
            v-for="(table, index) in tables"
            :key="table.tableName"
            class="table-item"
            :class="{ selected: selectedTables.includes(table.tableName), 'no-pk': table.primaryKeys.length === 0 }"
            @click="handleTableClick($event, table, index)"
          >
            <div class="table-checkbox">
              <input
                type="checkbox"
                :checked="selectedTables.includes(table.tableName)"
                :disabled="table.primaryKeys.length === 0"
                @click.stop.prevent="handleTableClick($event, table, index)"
              />
            </div>
            <div class="table-content">
              <span class="table-name">{{ table.tableName }}</span>
              <span class="table-info">
                <span v-if="table.primaryKeys.length === 0" class="no-pk-badge">No PK</span>
                <span v-else class="pk-badge">PK: {{ table.primaryKeys.join(', ') }}</span>
                <span class="row-count">{{ table.sourceCount }} rows</span>
              </span>
            </div>
          </div>
        </div>
        <div v-else-if="!loadingTables" class="empty-tables">
          Connect to databases and click Refresh to load tables
        </div>

        <!-- Compare Selected Button -->
        <div class="compare-actions" v-if="selectedTables.length > 0">
          <button class="btn btn-compare" @click="compareSelectedTables" :disabled="comparing">
            {{ comparing ? 'Comparing...' : `🔍 Compare ${selectedTables.length} Table(s)` }}
          </button>
        </div>
      </div>

      <!-- Right Panel: Comparison Results -->
      <div class="right-panel">
        <div class="panel-header">
          <h4>Comparison Results</h4>
          <span class="result-count" v-if="hasCompared">{{ comparedTablesCount }} table(s) compared</span>
        </div>

        <!-- Progress Log - Terminal Style (show when comparing OR when logs exist) -->
        <div class="terminal" v-if="comparing || logs.length > 0">
          <div class="terminal-header">
            <span class="terminal-dot red"></span>
            <span class="terminal-dot yellow"></span>
            <span class="terminal-dot green"></span>
            <span class="terminal-title">Data Compare</span>
          </div>
          <div class="terminal-body" ref="terminalBody">
            <div v-for="(log, i) in logs" :key="i" class="terminal-line">
              <span class="terminal-prompt">$</span>
              <span class="terminal-text" :class="log.type">{{ log.message }}</span>
              <span class="terminal-status" v-if="log.type === 'done'">✓</span>
              <span class="terminal-status error" v-else-if="log.type === 'error'">✗</span>
            </div>
            <div v-if="comparing" class="terminal-line">
              <span class="terminal-prompt">$</span>
              <span class="terminal-text">{{ currentStep }}</span>
              <span class="terminal-dots">
                <span class="dot" :class="{ active: dotIndex === 0 }">.</span>
                <span class="dot" :class="{ active: dotIndex === 1 }">.</span>
                <span class="dot" :class="{ active: dotIndex === 2 }">.</span>
              </span>
            </div>
            <div v-if="comparing" class="terminal-cursor"></div>
          </div>
        </div>

        <!-- Summary -->
        <div class="sync-summary" v-if="summary">
          <div class="summary-item insert">
            <span class="count">{{ summary.insertCount }}</span>
            <span class="label">Insert</span>
          </div>
          <div class="summary-item update">
            <span class="count">{{ summary.updateCount }}</span>
            <span class="label">Update</span>
          </div>
          <div class="summary-item delete">
            <span class="count">{{ summary.deleteCount }}</span>
            <span class="label">Delete</span>
          </div>
        </div>

        <!-- Sync Options -->
        <div class="sync-options" v-if="dataDiffs.length > 0">
          <label class="checkbox-label">
            <input type="checkbox" v-model="syncInsert" />
            <span>INSERT ({{ insertDiffs.length }})</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" v-model="syncUpdate" />
            <span>UPDATE ({{ updateDiffs.length }})</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" v-model="syncDelete" />
            <span>DELETE ({{ deleteDiffs.length }})</span>
          </label>
        </div>

        <!-- Diff List -->
        <div class="diff-list" v-if="filteredDiffs.length > 0">
          <div
            v-for="(diff, index) in filteredDiffs.slice(0, showLimit)"
            :key="index"
            class="diff-item"
            :class="diff.type"
          >
            <div class="diff-header">
              <span class="diff-badge" :class="diff.type">{{ diff.type.toUpperCase() }}</span>
              <span class="pk-info">{{ formatPrimaryKey(diff.primaryKey) }}</span>
            </div>
            <div class="diff-sql">
              <code>{{ diff.sql }}</code>
            </div>
          </div>

          <div v-if="filteredDiffs.length > showLimit" class="show-more">
            <button class="btn btn-small" @click="showLimit += 50">
              Show more ({{ filteredDiffs.length - showLimit }} remaining)
            </button>
          </div>
        </div>

        <div v-else-if="hasCompared && dataDiffs.length === 0 && !comparing" class="no-diff">
          ✅ Data is identical, no sync needed
        </div>

        <div v-else-if="!comparing && logs.length === 0" class="empty-results">
          Select tables and click Compare to see differences
        </div>

        <!-- Execute Actions -->
        <div class="sync-actions" v-if="filteredDiffs.length > 0">
          <button class="btn btn-copy" @click="copySQL">
            📋 Copy SQL ({{ filteredDiffs.length }})
          </button>
          <button class="btn btn-execute" @click="showConfirmDialog = true">
            ▶️ Execute Sync
          </button>
        </div>
      </div>
    </div>

    <!-- Confirm Dialog -->
    <div class="dialog-overlay" v-if="showConfirmDialog" @click.self="showConfirmDialog = false">
      <div class="dialog">
        <h4>⚠️ Confirm Sync</h4>
        <p>Execute {{ filteredDiffs.length }} SQL statements on target database?</p>
        <div class="dialog-actions">
          <button class="btn btn-cancel" @click="showConfirmDialog = false">Cancel</button>
          <button class="btn btn-confirm" @click="executeSync">Execute</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onUnmounted } from 'vue'
import { GetTablesForSync, CompareTableData, ExecuteSQL } from '../../wailsjs/go/main/App'
import { database } from '../../wailsjs/go/models'

type ConnectionConfig = database.ConnectionConfig
type TableDataInfo = database.TableDataInfo
type DataDiffResult = database.DataDiffResult

const props = defineProps<{
  sourceConfig: ConnectionConfig
  targetConfig: ConnectionConfig
  sourceConnected: boolean
  targetConnected: boolean
}>()

const emit = defineEmits<{
  'execute': [sql: string]
}>()

// State
const tables = ref<TableDataInfo[]>([])
const loadingTables = ref(false)
const selectedTables = ref<string[]>([])
const lastClickedIndex = ref<number | null>(null)
const comparing = ref(false)
const hasCompared = ref(false)
const dataDiffs = ref<DataDiffResult[]>([])
const comparedTablesCount = ref(0)
const showLimit = ref(50)
const showConfirmDialog = ref(false)

// Progress log
interface LogEntry {
  message: string
  type: 'progress' | 'done' | 'error'
  time?: string
}
const logs = ref<LogEntry[]>([])
const currentStep = ref('')
const dotIndex = ref(0)
const terminalBody = ref<HTMLElement | null>(null)
let dotInterval: number | null = null

function startDotAnimation() {
  dotInterval = window.setInterval(() => {
    dotIndex.value = (dotIndex.value + 1) % 3
  }, 400)
}

function stopDotAnimation() {
  if (dotInterval) {
    clearInterval(dotInterval)
    dotInterval = null
  }
}

onUnmounted(() => {
  stopDotAnimation()
})

async function addLog(message: string, type: 'progress' | 'done' | 'error' = 'progress') {
  const now = new Date()
  const time = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`
  logs.value.push({ message, type, time })
  await nextTick()
  if (terminalBody.value) {
    terminalBody.value.scrollTop = terminalBody.value.scrollHeight
  }
}

function clearLogs() {
  logs.value = []
  currentStep.value = ''
}

// Sync options
const syncInsert = ref(true)
const syncUpdate = ref(true)
const syncDelete = ref(false)

// Computed
const selectableTables = computed(() => tables.value.filter(t => t.primaryKeys.length > 0))

const isAllSelected = computed(() => {
  return selectableTables.value.length > 0 &&
         selectableTables.value.every(t => selectedTables.value.includes(t.tableName))
})

const insertDiffs = computed(() => dataDiffs.value.filter(d => d.type === 'insert'))
const updateDiffs = computed(() => dataDiffs.value.filter(d => d.type === 'update'))
const deleteDiffs = computed(() => dataDiffs.value.filter(d => d.type === 'delete'))

const summary = computed(() => {
  if (dataDiffs.value.length === 0) return null
  return {
    insertCount: insertDiffs.value.length,
    updateCount: updateDiffs.value.length,
    deleteCount: deleteDiffs.value.length
  }
})

const filteredDiffs = computed(() => {
  return dataDiffs.value.filter(d => {
    if (d.type === 'insert' && !syncInsert.value) return false
    if (d.type === 'update' && !syncUpdate.value) return false
    if (d.type === 'delete' && !syncDelete.value) return false
    return true
  })
})

// Methods
async function loadTables() {
  if (!props.sourceConnected || !props.sourceConfig.database) {
    alert('Please connect to source database first')
    return
  }

  loadingTables.value = true
  try {
    tables.value = await GetTablesForSync(props.sourceConfig) || []
    selectedTables.value = []
    lastClickedIndex.value = null
  } catch (e: any) {
    alert('Failed to load tables: ' + e)
  } finally {
    loadingTables.value = false
  }
}

function toggleTableSelection(table: TableDataInfo) {
  if (table.primaryKeys.length === 0) {
    return
  }
  const index = selectedTables.value.indexOf(table.tableName)
  if (index === -1) {
    selectedTables.value.push(table.tableName)
  } else {
    selectedTables.value.splice(index, 1)
  }
}

function handleTableClick(event: MouseEvent | Event, table: TableDataInfo, index: number) {
  if (table.primaryKeys.length === 0) {
    return
  }

  // Check if shift key is pressed (for range selection)
  const isShiftKey = event instanceof MouseEvent && event.shiftKey

  if (isShiftKey && lastClickedIndex.value !== null) {
    // Shift+click: select range
    const start = Math.min(lastClickedIndex.value, index)
    const end = Math.max(lastClickedIndex.value, index)

    for (let i = start; i <= end; i++) {
      const t = tables.value[i]
      if (t.primaryKeys.length > 0 && !selectedTables.value.includes(t.tableName)) {
        selectedTables.value.push(t.tableName)
      }
    }
  } else {
    // Normal click: toggle single selection
    toggleTableSelection(table)
    lastClickedIndex.value = index
  }
}

function toggleSelectAll() {
  if (isAllSelected.value) {
    selectedTables.value = []
  } else {
    selectedTables.value = selectableTables.value.map(t => t.tableName)
  }
}

async function compareSelectedTables() {
  if (selectedTables.value.length === 0) return

  comparing.value = true
  hasCompared.value = false
  dataDiffs.value = []
  comparedTablesCount.value = 0
  clearLogs()
  startDotAnimation()

  // Force UI update before starting
  await nextTick()

  try {
    await addLog(`Starting comparison for ${selectedTables.value.length} table(s)`, 'done')

    for (let i = 0; i < selectedTables.value.length; i++) {
      const tableName = selectedTables.value[i]
      currentStep.value = `Comparing table: ${tableName} (${i + 1}/${selectedTables.value.length})`

      // Force UI update to show current step
      await nextTick()

      try {
        const diffs = await CompareTableData(props.sourceConfig, props.targetConfig, tableName)
        if (diffs && diffs.length > 0) {
          dataDiffs.value.push(...diffs)
        }
        await addLog(`Compared ${tableName}: ${diffs?.length || 0} difference(s)`, 'done')
        comparedTablesCount.value++
      } catch (e: any) {
        await addLog(`Error comparing ${tableName}: ${e}`, 'error')
      }
    }

    hasCompared.value = true
    const totalDiffs = dataDiffs.value.length
    await addLog(`Comparison complete: ${totalDiffs} total difference(s) found`, 'done')

  } catch (e: any) {
    await addLog(`Error: ${e}`, 'error')
  } finally {
    stopDotAnimation()
    comparing.value = false
    currentStep.value = ''
  }
}

function formatPrimaryKey(pk: Record<string, any>): string {
  return Object.entries(pk).map(([k, v]) => `${k}=${v}`).join(', ')
}

function copySQL() {
  const sql = filteredDiffs.value.map(d => d.sql).join('\n')
  navigator.clipboard.writeText(sql)
  alert(`Copied ${filteredDiffs.value.length} SQL statements`)
}

async function executeSync() {
  if (filteredDiffs.value.length === 0) return

  showConfirmDialog.value = false

  try {
    const sql = filteredDiffs.value.map(d => d.sql).join('\n')
    await ExecuteSQL(props.targetConfig, sql)
    await compareSelectedTables()
  } catch (e: any) {
    console.error('Sync failed:', e)
  }
}
</script>

<style scoped>
.data-sync {
  background: #16213e;
  border-radius: 10px;
  padding: 20px;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.sync-header h3 {
  color: #4fc3f7;
  margin-bottom: 5px;
}

.hint {
  color: #888;
  font-size: 13px;
  margin-bottom: 15px;
}

.sync-layout {
  display: flex;
  gap: 20px;
  flex: 1;
  min-height: 0;
}

.left-panel {
  width: 320px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: #0f0f23;
  border-radius: 8px;
  padding: 15px;
}

.right-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: #0f0f23;
  border-radius: 8px;
  padding: 15px;
  min-width: 0;
  overflow: hidden;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  flex-shrink: 0;
}

.panel-header h4 {
  color: #fff;
  margin: 0;
  font-size: 14px;
}

.result-count {
  color: #888;
  font-size: 12px;
}

.select-all-row {
  margin-bottom: 10px;
  padding-bottom: 10px;
  border-bottom: 1px solid #333;
}

.select-all {
  font-size: 13px;
  color: #4fc3f7;
}

.select-all input {
  accent-color: #4fc3f7;
}

.table-list {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.table-item {
  background: #16213e;
  padding: 10px;
  border-radius: 6px;
  cursor: pointer;
  border: 2px solid transparent;
  transition: all 0.2s;
  display: flex;
  align-items: flex-start;
  gap: 8px;
  user-select: none;
}

.table-checkbox {
  flex-shrink: 0;
  padding-top: 2px;
}

.table-checkbox input {
  width: 16px;
  height: 16px;
  cursor: pointer;
  accent-color: #4fc3f7;
}

.table-checkbox input:disabled {
  cursor: not-allowed;
  opacity: 0.3;
}

.table-content {
  flex: 1;
  min-width: 0;
}

.table-item:hover:not(.no-pk) {
  border-color: #4fc3f7;
}

.table-item.selected {
  border-color: #4fc3f7;
  background: rgba(79, 195, 247, 0.1);
}

.table-item.no-pk {
  opacity: 0.5;
  cursor: not-allowed;
}

.table-name {
  font-weight: bold;
  color: #fff;
  display: block;
  margin-bottom: 3px;
  font-size: 13px;
}

.table-info {
  display: flex;
  gap: 8px;
  font-size: 11px;
  flex-wrap: wrap;
}

.pk-badge {
  color: #81c784;
}

.no-pk-badge {
  color: #f44336;
}

.row-count {
  color: #888;
}

.empty-tables {
  color: #888;
  text-align: center;
  padding: 20px;
  font-size: 13px;
}

.compare-actions {
  margin-top: 12px;
  flex-shrink: 0;
}

.empty-results {
  color: #666;
  text-align: center;
  padding: 40px 20px;
  font-size: 13px;
}

.sync-summary {
  display: flex;
  gap: 15px;
  margin-bottom: 15px;
  flex-shrink: 0;
}

.summary-item {
  background: #16213e;
  padding: 12px 20px;
  border-radius: 6px;
  text-align: center;
  border-left: 3px solid;
}

.summary-item.insert {
  border-left-color: #4caf50;
}

.summary-item.update {
  border-left-color: #ff9800;
}

.summary-item.delete {
  border-left-color: #f44336;
}

.summary-item .count {
  display: block;
  font-size: 20px;
  font-weight: bold;
  color: #fff;
}

.summary-item .label {
  color: #888;
  font-size: 12px;
}

.sync-options {
  display: flex;
  gap: 15px;
  margin-bottom: 15px;
  flex-shrink: 0;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  color: #ccc;
  font-size: 13px;
}

.checkbox-label input {
  width: 16px;
  height: 16px;
  cursor: pointer;
}

.diff-list {
  flex: 1;
  overflow-y: auto;
  margin-bottom: 15px;
}

.diff-item {
  background: #16213e;
  border-radius: 6px;
  margin-bottom: 8px;
  border-left: 3px solid;
  overflow: hidden;
}

.diff-item.insert {
  border-left-color: #4caf50;
}

.diff-item.update {
  border-left-color: #ff9800;
}

.diff-item.delete {
  border-left-color: #f44336;
}

.diff-header {
  padding: 8px 12px;
  background: rgba(255, 255, 255, 0.03);
  display: flex;
  align-items: center;
  gap: 10px;
}

.diff-badge {
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: bold;
}

.diff-badge.insert {
  background: rgba(76, 175, 80, 0.2);
  color: #4caf50;
}

.diff-badge.update {
  background: rgba(255, 152, 0, 0.2);
  color: #ff9800;
}

.diff-badge.delete {
  background: rgba(244, 67, 54, 0.2);
  color: #f44336;
}

.pk-info {
  color: #888;
  font-size: 11px;
}

.diff-sql {
  padding: 8px 12px;
}

.diff-sql code {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 11px;
  color: #81c784;
  word-break: break-all;
}

.show-more {
  text-align: center;
  padding: 10px;
}

.no-diff {
  text-align: center;
  padding: 30px;
  color: #4caf50;
}

.sync-actions {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  flex-shrink: 0;
}

.btn {
  padding: 8px 16px;
  border: none;
  border-radius: 5px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s;
}

.btn-refresh {
  background: #4fc3f7;
  color: #1a1a2e;
  font-size: 12px;
  padding: 6px 12px;
}

.btn-compare {
  background: #4fc3f7;
  color: #1a1a2e;
  font-weight: bold;
  width: 100%;
}

.btn-copy {
  background: #0f3460;
  color: #4fc3f7;
}

.btn-execute {
  background: #4caf50;
  color: white;
  font-weight: bold;
}

.btn-small {
  background: #333;
  color: #eee;
}

.btn:hover:not(:disabled) {
  filter: brightness(1.1);
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Dialog styles */
.dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.dialog {
  background: #16213e;
  border-radius: 10px;
  padding: 25px;
  width: 400px;
  border: 1px solid #333;
  text-align: center;
}

.dialog h4 {
  color: #ff9800;
  margin-bottom: 15px;
  font-size: 18px;
}

.dialog p {
  color: #ccc;
  margin-bottom: 20px;
  font-size: 14px;
}

.dialog-actions {
  display: flex;
  gap: 10px;
  justify-content: center;
}

.btn-cancel {
  padding: 10px 24px;
  border: 1px solid #333;
  border-radius: 5px;
  background: transparent;
  color: #888;
  cursor: pointer;
  font-size: 14px;
}

.btn-cancel:hover {
  background: #333;
}

.btn-confirm {
  padding: 10px 24px;
  border: none;
  border-radius: 5px;
  background: #4caf50;
  color: white;
  cursor: pointer;
  font-size: 14px;
  font-weight: bold;
}

.btn-confirm:hover {
  background: #45a045;
}

/* Terminal Style */
.terminal {
  background: #0d0d0d;
  border-radius: 6px;
  margin-bottom: 15px;
  overflow: hidden;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
  flex-shrink: 0;
}

.terminal-header {
  background: #2d2d2d;
  padding: 6px 10px;
  display: flex;
  align-items: center;
  gap: 5px;
}

.terminal-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.terminal-dot.red { background: #ff5f56; }
.terminal-dot.yellow { background: #ffbd2e; }
.terminal-dot.green { background: #27ca40; }

.terminal-title {
  margin-left: 8px;
  color: #888;
  font-size: 11px;
}

.terminal-body {
  padding: 12px;
  max-height: 150px;
  overflow-y: auto;
  font-size: 12px;
  line-height: 1.5;
}

.terminal-line {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 3px;
}

.terminal-prompt {
  color: #4caf50;
  font-weight: bold;
}

.terminal-text {
  color: #eee;
}

.terminal-text.done {
  color: #81c784;
}

.terminal-text.error {
  color: #f44336;
}

.terminal-status {
  color: #4caf50;
  font-weight: bold;
}

.terminal-status.error {
  color: #f44336;
}

.terminal-dots {
  display: inline-flex;
  gap: 1px;
  margin-left: 4px;
}

.terminal-dots .dot {
  color: #555;
  transition: color 0.2s;
}

.terminal-dots .dot.active {
  color: #4fc3f7;
}

.terminal-cursor {
  display: inline-block;
  width: 8px;
  height: 14px;
  background: #4fc3f7;
  margin-left: 4px;
  animation: blink 1s infinite;
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}
</style>
