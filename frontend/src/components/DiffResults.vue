<template>
  <div class="diff-results">
    <div class="results-header">
      <h3>{{ t('diff.differencesFound', { count: results.length }) }}</h3>
      <div class="header-actions">
        <button class="btn btn-copy" @click="copyAllSQL">
          {{ copiedAll ? t('diff.copied') : t('diff.copyAll') }}
        </button>
        <button class="btn btn-execute-all" @click="showConfirmDialog = true" v-if="results.length > 0">
          {{ t('diff.executeAll') }}
        </button>
      </div>
    </div>

    <div class="results-list">
      <div
        v-for="(result, index) in results"
        :key="index"
        class="result-item"
        :class="result.type"
      >
        <div class="result-header">
          <span class="badge" :class="result.type">
            {{ getTypeLabel(result.type) }}
          </span>
          <span class="table-name">{{ result.tableName }}</span>
          <span class="detail">{{ result.detail }}</span>
        </div>
        <div class="sql-block">
          <pre><code>{{ result.sql }}</code></pre>
          <div class="sql-actions">
            <button class="btn-small" @click="copySingleSQL(result.sql, index)">
              {{ copiedIndex === index ? t('diff.copied') : t('diff.copy') }}
            </button>
            <button class="btn-small btn-run" @click="$emit('execute', result.sql)">
              {{ t('diff.run') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Full SQL Preview -->
    <div class="full-sql">
      <h4>{{ t('diff.completeScript') }}</h4>
      <pre><code>{{ fullSQL }}</code></pre>
    </div>

    <!-- Confirm Dialog -->
    <div class="dialog-overlay" v-if="showConfirmDialog" @click.self="showConfirmDialog = false">
      <div class="dialog">
        <h4>{{ t('diff.confirmTitle') }}</h4>
        <p>{{ t('diff.confirmMsg', { count: results.length }) }}</p>
        <div class="dialog-actions">
          <button class="btn btn-cancel" @click="showConfirmDialog = false">{{ t('diff.cancel') }}</button>
          <button class="btn btn-confirm" @click="executeAll">{{ t('diff.execute') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { database } from '../../wailsjs/go/models'

type DiffResult = database.DiffResult

const { t } = useI18n()

const props = defineProps<{
  results: DiffResult[]
}>()

const emit = defineEmits<{
  'execute': [sql: string]
}>()

const showConfirmDialog = ref(false)
const copiedAll = ref(false)
const copiedIndex = ref<number | null>(null)

function getTypeLabel(type: string): string {
  switch (type) {
    case 'added': return t('diff.addBadge')
    case 'removed': return t('diff.dropBadge')
    case 'modified': return t('diff.modifyBadge')
    default: return type
  }
}

const fullSQL = computed(() => {
  return props.results.map(r => r.sql).join('\n\n')
})

function copyAllSQL() {
  navigator.clipboard.writeText(fullSQL.value)
  copiedAll.value = true
  window.setTimeout(() => {
    copiedAll.value = false
  }, 1500)
}

function copySingleSQL(sql: string, index: number) {
  navigator.clipboard.writeText(sql)
  copiedIndex.value = index
  window.setTimeout(() => {
    if (copiedIndex.value === index) {
      copiedIndex.value = null
    }
  }, 1500)
}

function executeAll() {
  showConfirmDialog.value = false
  emit('execute', fullSQL.value)
}
</script>

<style scoped>
.diff-results {
  background: #16213e;
  border-radius: 10px;
  padding: 20px;
  margin-top: 20px;
}

.results-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.results-header h3 {
  color: #4fc3f7;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.btn {
  padding: 8px 16px;
  border: none;
  border-radius: 5px;
  cursor: pointer;
  font-size: 13px;
}

.btn-copy {
  background: #0f3460;
  color: #4fc3f7;
}

.btn-execute-all {
  background: #4caf50;
  color: white;
}

.btn-copy:hover {
  background: #1a4a7a;
}

.btn-execute-all:hover {
  background: #45a045;
}

.results-list {
  display: flex;
  flex-direction: column;
  gap: 15px;
  margin-bottom: 20px;
}

.result-item {
  background: #0f0f23;
  border-radius: 8px;
  overflow: hidden;
  border-left: 4px solid;
}

.result-item.added {
  border-left-color: #4caf50;
}

.result-item.removed {
  border-left-color: #f44336;
}

.result-item.modified {
  border-left-color: #ff9800;
}

.result-header {
  padding: 12px 15px;
  display: flex;
  align-items: center;
  gap: 10px;
  background: rgba(255, 255, 255, 0.03);
}

.badge {
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: bold;
}

.badge.added {
  background: rgba(76, 175, 80, 0.2);
  color: #4caf50;
}

.badge.removed {
  background: rgba(244, 67, 54, 0.2);
  color: #f44336;
}

.badge.modified {
  background: rgba(255, 152, 0, 0.2);
  color: #ff9800;
}

.table-name {
  font-weight: bold;
  color: #fff;
}

.detail {
  color: #888;
  font-size: 13px;
}

.sql-block {
  padding: 15px;
  position: relative;
}

.sql-block pre {
  margin: 0;
  overflow-x: auto;
}

.sql-block code {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
  color: #81c784;
}

.sql-actions {
  position: absolute;
  top: 10px;
  right: 10px;
  display: flex;
  gap: 5px;
  opacity: 0;
  transition: opacity 0.2s;
}

.sql-block:hover .sql-actions {
  opacity: 1;
}

.btn-small {
  padding: 4px 10px;
  border: none;
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
  background: #333;
  color: #eee;
}

.btn-small:hover {
  background: #444;
}

.btn-run {
  background: #4caf50;
  color: white;
}

.btn-run:hover {
  background: #45a045;
}

.full-sql {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid #333;
}

.full-sql h4 {
  color: #4fc3f7;
  margin-bottom: 10px;
}

.full-sql pre {
  background: #0f0f23;
  padding: 15px;
  border-radius: 8px;
  overflow-x: auto;
  max-height: 300px;
}

.full-sql code {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
  color: #81c784;
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
</style>
