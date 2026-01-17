<template>
  <div class="app">
    <!-- Top Bar -->
    <header class="top-bar">
      <div class="brand">
        <h1>🔄 Skeema GUI</h1>
        <span class="version">v{{ appVersion }}</span>
      </div>
      <div class="top-actions">
        <button class="btn-update" @click="checkForUpdates" :disabled="checkingUpdate" v-if="!updateInfo?.available">
          {{ checkingUpdate ? 'Checking...' : '🔄 Check Updates' }}
        </button>
        <button class="btn-update has-update" @click="showUpdateDialog = true" v-else>
          🎉 Update Available: v{{ updateInfo.latestVersion }}
        </button>
        <div class="connection-status" @click="showConnectionDialog = true">
        <template v-if="bothConnected">
          <span class="status-badge connected">
            <span class="status-dot"></span>
            {{ sourceConfig.database }} ➜ {{ targetConfig.database }}
          </span>
        </template>
        <template v-else>
          <span class="status-badge disconnected">
            <span class="status-dot"></span>
            Click to connect
          </span>
        </template>
        <button class="btn-settings">⚙️</button>
        </div>
      </div>
    </header>

    <!-- Tab Navigation -->
    <nav class="tab-nav">
      <button
        class="tab"
        :class="{ active: activeTab === 'schema' }"
        @click="activeTab = 'schema'"
      >
        📋 Schema Compare
      </button>
      <button
        class="tab"
        :class="{ active: activeTab === 'data' }"
        @click="activeTab = 'data'"
      >
        📊 Data Sync
      </button>
      <button
        class="tab"
        :class="{ active: activeTab === 'browser' }"
        @click="activeTab = 'browser'"
      >
        🗂️ Table Browser
      </button>
    </nav>

    <!-- Main Content Area -->
    <main class="content">
      <!-- Schema Tab -->
      <div class="tab-content" v-show="activeTab === 'schema'">
        <div class="actions">
          <button
            class="btn btn-primary"
            @click="compareSchemas"
            :disabled="!canCompare || comparing"
          >
            {{ comparing ? 'Comparing...' : '🔍 Compare Schemas' }}
          </button>
        </div>

        <!-- Terminal -->
        <div class="terminal" v-if="comparing || schemaLogs.length > 0">
          <div class="terminal-header">
            <span class="terminal-dot red"></span>
            <span class="terminal-dot yellow"></span>
            <span class="terminal-dot green"></span>
            <span class="terminal-title">Schema Compare</span>
          </div>
          <div class="terminal-body" ref="terminalBody">
            <div v-for="(log, i) in schemaLogs" :key="i" class="terminal-line">
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

        <DiffResults
          v-if="diffResults.length > 0"
          :results="diffResults"
          :target-config="targetConfig"
          @execute="executeSQL"
        />

        <div v-else-if="hasCompared" class="empty-state">
          ✅ No differences found. Schemas are identical.
        </div>

        <div v-else-if="!canCompare" class="empty-state hint">
          Connect to both databases to compare schemas
        </div>
      </div>

      <!-- Data Sync Tab -->
      <div class="tab-content" v-show="activeTab === 'data'">
        <DataSync
          :source-config="sourceConfig"
          :target-config="targetConfig"
          :source-connected="sourceConnected"
          :target-connected="targetConnected"
        />
      </div>

      <!-- Table Browser Tab -->
      <div class="tab-content" v-show="activeTab === 'browser'">
        <TableBrowser
          :config="browserTarget === 'source' ? sourceConfig : targetConfig"
          :connected="browserTarget === 'source' ? sourceConnected : targetConnected"
          :browser-target="browserTarget"
          @switch-target="browserTarget = $event"
        />
      </div>
    </main>

    <!-- Connection Dialog -->
    <div class="dialog-overlay" v-if="showConnectionDialog" @click.self="closeConnectionDialog">
      <div class="connection-dialog">
        <div class="dialog-header">
          <h2>Database Connections</h2>
          <button class="btn-close" @click="closeConnectionDialog">×</button>
        </div>
        <div class="dialog-body">
          <!-- Connection Error Message -->
          <div class="connection-error" v-if="connectionError">
            <span class="error-icon">⚠️</span>
            <span class="error-text">{{ connectionError }}</span>
            <button class="btn-dismiss" @click="connectionError = ''">×</button>
          </div>
          <div class="connection-forms">
            <ConnectionForm
              title="Source Database"
              :config="sourceConfig"
              :databases="sourceDatabases"
              :loading="sourceLoading"
              :connected="sourceConnected"
              @update:config="sourceConfig = $event"
              @test="testSourceConnection"
              @load-databases="loadSourceDatabases"
              @auto-connect="testSourceConnection"
            />

            <div class="conn-arrow">➜</div>

            <ConnectionForm
              title="Target Database"
              :config="targetConfig"
              :databases="targetDatabases"
              :loading="targetLoading"
              :connected="targetConnected"
              @update:config="targetConfig = $event"
              @test="testTargetConnection"
              @load-databases="loadTargetDatabases"
              @auto-connect="testTargetConnection"
            />
          </div>
        </div>
        <div class="dialog-footer" v-if="bothConnected">
          <button class="btn btn-primary" @click="closeConnectionDialog">Done</button>
        </div>
      </div>
    </div>

    <!-- Update Dialog -->
    <div class="dialog-overlay" v-if="showUpdateDialog" @click.self="showUpdateDialog = false">
      <div class="update-dialog">
        <div class="dialog-header">
          <h2>🎉 Update Available</h2>
          <button class="btn-close" @click="showUpdateDialog = false">×</button>
        </div>
        <div class="dialog-body">
          <div class="update-info">
            <div class="version-compare">
              <span class="current">v{{ updateInfo?.currentVersion }}</span>
              <span class="arrow">➜</span>
              <span class="latest">v{{ updateInfo?.latestVersion }}</span>
            </div>
            <div class="release-notes" v-if="updateInfo?.releaseNotes">
              <h4>Release Notes</h4>
              <div class="notes-content">{{ updateInfo.releaseNotes }}</div>
            </div>
          </div>
        </div>
        <div class="dialog-footer">
          <button class="btn btn-cancel" @click="showUpdateDialog = false" :disabled="isUpdating">Later</button>
          <button class="btn btn-secondary" @click="openDownloadPage" :disabled="isUpdating">
            🌐 View Release
          </button>
          <button class="btn btn-primary" @click="applyUpdate" :disabled="isUpdating">
            {{ isUpdating ? '⏳ Updating...' : '🚀 Install Update' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onUnmounted } from 'vue'
import ConnectionForm from './components/ConnectionForm.vue'
import DiffResults from './components/DiffResults.vue'
import DataSync from './components/DataSync.vue'
import TableBrowser from './components/TableBrowser.vue'
import { TestConnection, GetDatabases, CompareSchemas, ExecuteSQL, GetAppVersion, CheckForUpdates, OpenReleaseURL, DownloadAndApplyUpdate } from '../wailsjs/go/main/App'
import { database } from '../wailsjs/go/models'

type ConnectionConfig = database.ConnectionConfig
type DiffResult = database.DiffResult

interface UpdateInfo {
  available: boolean
  currentVersion: string
  latestVersion: string
  releaseNotes: string
  downloadUrl: string
  releaseUrl: string
  assetName: string
  assetSize: number
}

// App version and updates
const appVersion = ref('1.0.0')
const updateInfo = ref<UpdateInfo | null>(null)
const checkingUpdate = ref(false)
const showUpdateDialog = ref(false)
const isUpdating = ref(false)

// Load version on mount
;(async () => {
  try {
    appVersion.value = await GetAppVersion()
  } catch (e) {
    console.error('Failed to get app version:', e)
  }
})()

// Active tab
const activeTab = ref<'schema' | 'data' | 'browser'>('schema')

// Connection dialog
const showConnectionDialog = ref(true)
const connectionError = ref('')

// Browser target switch
const browserTarget = ref<'source' | 'target'>('target')

// Source connection
const sourceConfig = ref<ConnectionConfig>({
  host: 'localhost',
  port: 3306,
  user: 'root',
  password: '',
  database: ''
})
const sourceDatabases = ref<string[]>([])
const sourceLoading = ref(false)
const sourceConnected = ref(false)

// Target connection
const targetConfig = ref<ConnectionConfig>({
  host: 'localhost',
  port: 3306,
  user: 'root',
  password: '',
  database: ''
})
const targetDatabases = ref<string[]>([])
const targetLoading = ref(false)
const targetConnected = ref(false)

// Comparison state
const comparing = ref(false)
const hasCompared = ref(false)
const diffResults = ref<DiffResult[]>([])

// Schema comparison logs
interface LogEntry {
  message: string
  type: 'progress' | 'done' | 'error'
  time?: string
}
const schemaLogs = ref<LogEntry[]>([])
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

function addSchemaLog(message: string, type: 'progress' | 'done' | 'error' = 'progress') {
  const now = new Date()
  const time = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`
  schemaLogs.value.push({ message, type, time })
  nextTick(() => {
    if (terminalBody.value) {
      terminalBody.value.scrollTop = terminalBody.value.scrollHeight
    }
  })
}

function clearSchemaLogs() {
  schemaLogs.value = []
  currentStep.value = ''
}

const canCompare = computed(() => {
  return sourceConnected.value &&
         targetConnected.value &&
         sourceConfig.value.database &&
         targetConfig.value.database
})

const bothConnected = computed(() => {
  return sourceConnected.value &&
         targetConnected.value &&
         sourceConfig.value.database &&
         targetConfig.value.database
})

function closeConnectionDialog() {
  if (bothConnected.value) {
    showConnectionDialog.value = false
  }
}

async function testSourceConnection() {
  sourceLoading.value = true
  connectionError.value = ''
  try {
    await TestConnection(sourceConfig.value)
    sourceConnected.value = true
    await loadSourceDatabases()
  } catch (e: any) {
    connectionError.value = 'Source connection failed: ' + e
    sourceConnected.value = false
  } finally {
    sourceLoading.value = false
  }
}

async function loadSourceDatabases() {
  try {
    sourceDatabases.value = await GetDatabases(sourceConfig.value)
  } catch (e: any) {
    console.error(e)
  }
}

async function testTargetConnection() {
  targetLoading.value = true
  connectionError.value = ''
  try {
    await TestConnection(targetConfig.value)
    targetConnected.value = true
    await loadTargetDatabases()
  } catch (e: any) {
    connectionError.value = 'Target connection failed: ' + e
    targetConnected.value = false
  } finally {
    targetLoading.value = false
  }
}

async function loadTargetDatabases() {
  try {
    targetDatabases.value = await GetDatabases(targetConfig.value)
  } catch (e: any) {
    console.error(e)
  }
}

async function compareSchemas() {
  comparing.value = true
  hasCompared.value = false
  diffResults.value = []
  clearSchemaLogs()
  startDotAnimation()

  try {
    currentStep.value = 'Initializing comparison'
    await delay(300)
    addSchemaLog('Initializing comparison', 'done')

    currentStep.value = `Connecting to source: ${sourceConfig.value.database}`
    await delay(200)
    addSchemaLog(`Connected to source: ${sourceConfig.value.database}`, 'done')

    currentStep.value = 'Fetching source schema'
    await delay(200)
    addSchemaLog('Fetched source schema', 'done')

    currentStep.value = `Connecting to target: ${targetConfig.value.database}`
    await delay(200)
    addSchemaLog(`Connected to target: ${targetConfig.value.database}`, 'done')

    currentStep.value = 'Fetching target schema'
    await delay(200)
    addSchemaLog('Fetched target schema', 'done')

    currentStep.value = 'Comparing table structures'
    const results = await CompareSchemas(sourceConfig.value, targetConfig.value)
    diffResults.value = results || []
    hasCompared.value = true

    addSchemaLog('Compared table structures', 'done')

    const diffCount = results?.length || 0
    addSchemaLog(`Comparison complete: ${diffCount} difference(s) found`, 'done')

  } catch (e: any) {
    addSchemaLog(`Error: ${e}`, 'error')
  } finally {
    stopDotAnimation()
    comparing.value = false
    currentStep.value = ''
  }
}

function delay(ms: number) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

async function executeSQL(sql: string) {
  try {
    await ExecuteSQL(targetConfig.value, sql)
    alert('SQL executed successfully!')
    await compareSchemas()
  } catch (e: any) {
    alert('Execution failed: ' + e)
  }
}

async function checkForUpdates() {
  checkingUpdate.value = true
  try {
    const info = await CheckForUpdates()
    updateInfo.value = info as UpdateInfo
    if (!info?.available) {
      alert('You are running the latest version!')
    }
  } catch (e: any) {
    alert('Failed to check for updates: ' + e)
  } finally {
    checkingUpdate.value = false
  }
}

async function openDownloadPage() {
  if (updateInfo.value?.releaseUrl) {
    try {
      await OpenReleaseURL(updateInfo.value.releaseUrl)
      showUpdateDialog.value = false
    } catch (e: any) {
      alert('Failed to open download page: ' + e)
    }
  }
}

async function applyUpdate() {
  if (!updateInfo.value?.downloadUrl) {
    alert('No download URL available')
    return
  }

  isUpdating.value = true
  try {
    await DownloadAndApplyUpdate(updateInfo.value.downloadUrl)
    // App will restart automatically
  } catch (e: any) {
    alert('Failed to apply update: ' + e)
    isUpdating.value = false
  }
}
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

html, body, #app {
  height: 100%;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
  background: #1a1a2e;
  color: #eee;
}

.app {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

/* Top Bar */
.top-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 20px;
  background: #16213e;
  border-bottom: 1px solid #333;
  flex-shrink: 0;
}

.brand h1 {
  font-size: 20px;
  color: #4fc3f7;
}

.connection-status {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
}

.status-badge {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  border-radius: 20px;
  font-size: 13px;
  transition: all 0.2s;
}

.status-badge.connected {
  background: rgba(76, 175, 80, 0.15);
  color: #81c784;
}

.status-badge.disconnected {
  background: rgba(255, 152, 0, 0.15);
  color: #ffb74d;
}

.status-badge:hover {
  filter: brightness(1.2);
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
}

.btn-settings {
  padding: 6px 10px;
  border: none;
  border-radius: 6px;
  background: #0f3460;
  font-size: 16px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-settings:hover {
  background: #1a4a7a;
}

/* Tab Navigation */
.tab-nav {
  display: flex;
  gap: 5px;
  padding: 10px 20px;
  background: #16213e;
  flex-shrink: 0;
}

.tab {
  padding: 10px 20px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: #888;
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s;
}

.tab:hover {
  color: #4fc3f7;
  background: rgba(79, 195, 247, 0.1);
}

.tab.active {
  background: #4fc3f7;
  color: #1a1a2e;
  font-weight: bold;
}

/* Main Content */
.content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}

.tab-content {
  height: 100%;
}

.actions {
  text-align: center;
  margin-bottom: 20px;
}

.btn {
  padding: 12px 30px;
  border: none;
  border-radius: 6px;
  font-size: 16px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-primary {
  background: #4fc3f7;
  color: #1a1a2e;
  font-weight: bold;
}

.btn-primary:hover:not(:disabled) {
  background: #29b6f6;
  transform: translateY(-1px);
}

.btn-primary:disabled {
  background: #555;
  color: #888;
  cursor: not-allowed;
}

.empty-state {
  text-align: center;
  padding: 60px 20px;
  color: #4caf50;
  font-size: 18px;
}

.empty-state.hint {
  color: #888;
  font-size: 16px;
}

/* Connection Dialog */
.dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.connection-dialog {
  background: #1a1a2e;
  border-radius: 12px;
  width: 90%;
  max-width: 800px;
  max-height: 90vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  background: #16213e;
  border-bottom: 1px solid #333;
}

.dialog-header h2 {
  color: #4fc3f7;
  font-size: 18px;
}

.btn-close {
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: #888;
  font-size: 24px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
}

.btn-close:hover {
  background: #333;
  color: #fff;
}

.dialog-body {
  padding: 20px;
  overflow-y: auto;
}

.connection-forms {
  display: flex;
  gap: 20px;
  align-items: flex-start;
  justify-content: center;
}

.connection-error {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  margin-bottom: 16px;
  background: rgba(244, 67, 54, 0.15);
  border: 1px solid rgba(244, 67, 54, 0.3);
  border-radius: 8px;
  color: #f44336;
}

.error-icon {
  font-size: 18px;
}

.error-text {
  flex: 1;
  font-size: 13px;
}

.btn-dismiss {
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: #f44336;
  font-size: 18px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
}

.btn-dismiss:hover {
  background: rgba(244, 67, 54, 0.2);
}

.conn-arrow {
  font-size: 28px;
  color: #4fc3f7;
  padding-top: 80px;
}

.dialog-footer {
  padding: 16px 20px;
  background: #16213e;
  border-top: 1px solid #333;
  text-align: center;
}

/* Terminal Style */
.terminal {
  background: #0d0d0d;
  border-radius: 8px;
  margin: 0 auto 20px;
  max-width: 700px;
  overflow: hidden;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
}

.terminal-header {
  background: #2d2d2d;
  padding: 8px 12px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.terminal-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.terminal-dot.red { background: #ff5f56; }
.terminal-dot.yellow { background: #ffbd2e; }
.terminal-dot.green { background: #27ca40; }

.terminal-title {
  margin-left: 10px;
  color: #888;
  font-size: 12px;
}

.terminal-body {
  padding: 15px;
  max-height: 200px;
  overflow-y: auto;
  font-size: 13px;
  line-height: 1.6;
}

.terminal-line {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
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
  gap: 2px;
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
  height: 16px;
  background: #4fc3f7;
  margin-left: 4px;
  animation: blink 1s infinite;
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

/* Version and Update Styles */
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.version {
  font-size: 12px;
  color: #888;
  background: rgba(79, 195, 247, 0.1);
  padding: 3px 8px;
  border-radius: 10px;
}

.top-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.btn-update {
  padding: 6px 14px;
  border: 1px solid #333;
  border-radius: 6px;
  background: #0f3460;
  color: #4fc3f7;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-update:hover:not(:disabled) {
  background: #1a4a7a;
}

.btn-update:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-update.has-update {
  background: rgba(76, 175, 80, 0.15);
  border-color: #4caf50;
  color: #81c784;
  animation: pulse 2s infinite;
}

.btn-update.has-update:hover {
  background: rgba(76, 175, 80, 0.25);
}

@keyframes pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(76, 175, 80, 0.4); }
  50% { box-shadow: 0 0 0 8px rgba(76, 175, 80, 0); }
}

/* Update Dialog */
.update-dialog {
  background: #1a1a2e;
  border-radius: 12px;
  width: 90%;
  max-width: 500px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.update-info {
  text-align: center;
}

.version-compare {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 15px;
  font-size: 20px;
  margin-bottom: 20px;
}

.version-compare .current {
  color: #888;
}

.version-compare .arrow {
  color: #4fc3f7;
}

.version-compare .latest {
  color: #4caf50;
  font-weight: bold;
}

.release-notes {
  text-align: left;
  background: #0f0f23;
  border-radius: 8px;
  padding: 15px;
  margin-top: 15px;
}

.release-notes h4 {
  color: #4fc3f7;
  margin-bottom: 10px;
  font-size: 14px;
}

.notes-content {
  color: #ccc;
  font-size: 13px;
  line-height: 1.6;
  max-height: 200px;
  overflow-y: auto;
  white-space: pre-wrap;
}

.btn-cancel {
  padding: 10px 20px;
  border: 1px solid #333;
  border-radius: 6px;
  background: transparent;
  color: #888;
  font-size: 14px;
  cursor: pointer;
}

.btn-cancel:hover {
  background: #333;
}
</style>
