<template>
  <div class="connection-form">
    <div class="form-header">
      <h3>{{ title }}</h3>
      <div class="status" :class="{ connected }">
        {{ connected ? '✓ Connected' : '○ Not connected' }}
      </div>
    </div>

    <!-- Saved Connections -->
    <div class="form-group saved-connections">
      <label>Saved Connections</label>
      <div class="saved-row">
        <select v-model="selectedSaved" @change="loadSavedConnection">
          <option value="">-- Select saved --</option>
          <option v-for="conn in savedConnections" :key="conn.name" :value="conn.name">
            {{ conn.name }}
          </option>
        </select>
        <button class="btn-icon" @click="showSaveDialog = true" title="Save current connection">💾</button>
        <button
          class="btn-icon btn-delete"
          @click="deleteSelectedConnection"
          :disabled="!selectedSaved"
          title="Delete selected connection"
        >🗑️</button>
      </div>
    </div>

    <div class="form-group">
      <label>Host</label>
      <input
        type="text"
        :value="config.host"
        @input="updateField('host', ($event.target as HTMLInputElement).value)"
        placeholder="localhost"
      />
    </div>

    <div class="form-group">
      <label>Port</label>
      <input
        type="number"
        :value="config.port"
        @input="updateField('port', parseInt(($event.target as HTMLInputElement).value) || 3306)"
        placeholder="3306"
      />
    </div>

    <div class="form-group">
      <label>User</label>
      <input
        type="text"
        :value="config.user"
        @input="updateField('user', ($event.target as HTMLInputElement).value)"
        placeholder="root"
      />
    </div>

    <div class="form-group">
      <label>Password</label>
      <input
        type="password"
        :value="config.password"
        @input="updateField('password', ($event.target as HTMLInputElement).value)"
        placeholder="••••••••"
      />
    </div>

    <div class="form-group">
      <label>Database</label>
      <div class="database-row">
        <select
          :value="config.database"
          @change="updateField('database', ($event.target as HTMLSelectElement).value)"
          :disabled="databases.length === 0"
        >
          <option value="">-- Select database --</option>
          <option v-for="db in databases" :key="db" :value="db">{{ db }}</option>
        </select>
        <button
          class="btn-add-db"
          @click="showCreateDialog = true"
          :disabled="!connected"
          title="Create new database"
        >+</button>
      </div>
    </div>

    <button
      class="btn btn-connect"
      @click="$emit('test')"
      :disabled="loading"
    >
      {{ loading ? 'Connecting...' : 'Connect' }}
    </button>

    <!-- Create Database Dialog -->
    <div class="dialog-overlay" v-if="showCreateDialog" @click.self="showCreateDialog = false">
      <div class="dialog">
        <h4>Create Database</h4>
        <div class="dialog-form">
          <div class="form-group">
            <label>Database Name</label>
            <input type="text" v-model="newDbName" placeholder="new_database" />
          </div>
          <div class="form-group">
            <label>Charset</label>
            <select v-model="newDbCharset">
              <option value="utf8mb4">utf8mb4</option>
              <option value="utf8">utf8</option>
              <option value="latin1">latin1</option>
              <option value="gbk">gbk</option>
            </select>
          </div>
          <div class="form-group">
            <label>Collation</label>
            <select v-model="newDbCollation">
              <option value="utf8mb4_unicode_ci">utf8mb4_unicode_ci</option>
              <option value="utf8mb4_general_ci">utf8mb4_general_ci</option>
              <option value="utf8_general_ci">utf8_general_ci</option>
              <option value="latin1_swedish_ci">latin1_swedish_ci</option>
            </select>
          </div>
        </div>
        <div class="dialog-actions">
          <button class="btn btn-cancel" @click="showCreateDialog = false">Cancel</button>
          <button class="btn btn-create" @click="createDatabase" :disabled="!newDbName || creating">
            {{ creating ? 'Creating...' : 'Create' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Save Connection Dialog -->
    <div class="dialog-overlay" v-if="showSaveDialog" @click.self="showSaveDialog = false">
      <div class="dialog">
        <h4>Save Connection</h4>
        <div class="dialog-form">
          <div class="form-group">
            <label>Connection Name</label>
            <input type="text" v-model="saveConnName" placeholder="My Database" />
          </div>
        </div>
        <div class="dialog-actions">
          <button class="btn btn-cancel" @click="showSaveDialog = false">Cancel</button>
          <button class="btn btn-create" @click="saveConnection" :disabled="!saveConnName">
            Save
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { CreateDatabase, GetSavedConnections, SaveConnection, DeleteConnection } from '../../wailsjs/go/main/App'

interface ConnectionConfig {
  host: string
  port: number
  user: string
  password: string
  database: string
}

interface SavedConnection {
  name: string
  config: ConnectionConfig
}

const props = defineProps<{
  title: string
  config: ConnectionConfig
  databases: string[]
  loading: boolean
  connected: boolean
}>()

const emit = defineEmits<{
  'update:config': [config: ConnectionConfig]
  'test': []
  'load-databases': []
  'database-created': [dbName: string]
  'auto-connect': []
}>()

// Create database dialog state
const showCreateDialog = ref(false)
const newDbName = ref('')
const newDbCharset = ref('utf8mb4')
const newDbCollation = ref('utf8mb4_unicode_ci')
const creating = ref(false)

// Saved connections state
const savedConnections = ref<SavedConnection[]>([])
const selectedSaved = ref('')
const showSaveDialog = ref(false)
const saveConnName = ref('')

onMounted(async () => {
  await loadSavedConnections()
})

async function loadSavedConnections() {
  try {
    savedConnections.value = await GetSavedConnections() || []
  } catch (e) {
    console.error('Failed to load saved connections:', e)
  }
}

function loadSavedConnection() {
  if (!selectedSaved.value) return
  const conn = savedConnections.value.find(c => c.name === selectedSaved.value)
  if (conn) {
    emit('update:config', { ...conn.config })
    // Auto-connect after loading saved connection
    setTimeout(() => {
      emit('auto-connect')
    }, 50)
  }
}

async function saveConnection() {
  if (!saveConnName.value) return
  try {
    await SaveConnection(saveConnName.value, props.config)
    await loadSavedConnections()
    selectedSaved.value = saveConnName.value
    showSaveDialog.value = false
    saveConnName.value = ''
  } catch (e: any) {
    alert('Failed to save connection: ' + e)
  }
}

async function deleteSelectedConnection() {
  if (!selectedSaved.value) return
  try {
    await DeleteConnection(selectedSaved.value)
    await loadSavedConnections()
    selectedSaved.value = ''
  } catch (e: any) {
    alert('Failed to delete connection: ' + e)
  }
}

function updateField(field: keyof ConnectionConfig, value: string | number) {
  emit('update:config', { ...props.config, [field]: value })
}

async function createDatabase() {
  if (!newDbName.value) return

  creating.value = true
  try {
    await CreateDatabase(props.config, newDbName.value, newDbCharset.value, newDbCollation.value)
    alert(`Database "${newDbName.value}" created successfully!`)
    emit('database-created', newDbName.value)
    emit('load-databases')
    // Auto-select the new database
    emit('update:config', { ...props.config, database: newDbName.value })
    showCreateDialog.value = false
    newDbName.value = ''
  } catch (e: any) {
    alert('Failed to create database: ' + e)
  } finally {
    creating.value = false
  }
}
</script>

<style scoped>
.connection-form {
  background: #16213e;
  border-radius: 10px;
  padding: 20px;
  width: 320px;
}

.form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.form-header h3 {
  margin: 0;
  color: #4fc3f7;
}

.status {
  font-size: 12px;
  color: #888;
}

.status.connected {
  color: #4caf50;
}

.saved-connections {
  margin-bottom: 15px;
  padding-bottom: 15px;
  border-bottom: 1px solid #333;
}

.saved-row {
  display: flex;
  gap: 6px;
}

.saved-row select {
  flex: 1;
}

.btn-icon {
  width: 36px;
  height: 36px;
  border: 1px solid #333;
  border-radius: 5px;
  background: #0f3460;
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.btn-icon:hover:not(:disabled) {
  background: #1a4a7a;
}

.btn-icon:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-icon.btn-delete:hover:not(:disabled) {
  background: #5a2020;
}

.form-group {
  margin-bottom: 12px;
}

.form-group label {
  display: block;
  font-size: 12px;
  color: #888;
  margin-bottom: 4px;
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 10px;
  border: 1px solid #333;
  border-radius: 5px;
  background: #0f0f23;
  color: #eee;
  font-size: 14px;
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: #4fc3f7;
}

.form-group select {
  cursor: pointer;
}

.form-group select:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.btn-connect {
  width: 100%;
  padding: 10px;
  border: none;
  border-radius: 5px;
  background: #0f3460;
  color: #4fc3f7;
  font-size: 14px;
  cursor: pointer;
  transition: background 0.2s;
}

.btn-connect:hover:not(:disabled) {
  background: #1a4a7a;
}

.btn-connect:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.database-row {
  display: flex;
  gap: 8px;
}

.database-row select {
  flex: 1;
}

.btn-add-db {
  width: 36px;
  height: 36px;
  border: 1px solid #333;
  border-radius: 5px;
  background: #0f3460;
  color: #4fc3f7;
  font-size: 18px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-add-db:hover:not(:disabled) {
  background: #1a4a7a;
}

.btn-add-db:disabled {
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
  padding: 20px;
  width: 300px;
  border: 1px solid #333;
}

.dialog h4 {
  color: #4fc3f7;
  margin-bottom: 15px;
}

.dialog-form .form-group {
  margin-bottom: 12px;
}

.dialog-actions {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  margin-top: 15px;
}

.btn-cancel {
  padding: 8px 16px;
  border: 1px solid #333;
  border-radius: 5px;
  background: transparent;
  color: #888;
  cursor: pointer;
}

.btn-cancel:hover {
  background: #333;
}

.btn-create {
  padding: 8px 16px;
  border: none;
  border-radius: 5px;
  background: #4caf50;
  color: white;
  cursor: pointer;
}

.btn-create:hover:not(:disabled) {
  background: #45a045;
}

.btn-create:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
