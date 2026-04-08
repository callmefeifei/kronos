<template>
  <div class="notifications">
    <h2 class="page-title">通知记录</h2>
    <p class="page-desc">查看任务通知历史</p>

    <div class="content-card">
      <div class="filter-bar">
        <el-select
          v-model="filters.status"
          placeholder="通知状态"
          clearable
          style="width: 140px"
          @change="handleFilter"
        >
          <el-option label="已发送" value="sent" />
          <el-option label="失败" value="failed" />
        </el-select>
      </div>

      <el-table
        v-loading="loading"
        :data="notifications"
        stripe
        style="width: 100%"
        row-key="id"
        @expand-change="handleExpand"
      >
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="expand-content">
              <div class="expand-block">
                <h4>Payload</h4>
                <pre class="json-block">{{ formatJson(row.payload) }}</pre>
              </div>
              <div class="expand-block" v-if="row.response">
                <h4>Response</h4>
                <pre class="json-block">{{ formatJson(row.response) }}</pre>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="任务名称" prop="task_name" min-width="160" />
        <el-table-column label="通知渠道" width="120">
          <template #default="{ row }">
            <el-tag size="small" disable-transitions>{{ row.channel }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag
              :type="row.status === 'sent' ? 'success' : 'danger'"
              size="small"
              disable-transitions
            >
              {{ row.status === 'sent' ? '已发送' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" prop="created_at" width="170">
          <template #default="{ row }">
            <span class="text-secondary">{{ row.created_at || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Payload" min-width="200">
          <template #default="{ row }">
            <span class="text-secondary payload-preview">{{ truncate(row.payload) }}</span>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrap" v-if="total > 0">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.size"
          :total="total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          background
          @size-change="fetchNotifications"
          @current-change="fetchNotifications"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { listNotifications } from '@/api/notification'

const loading = ref(false)
const notifications = ref([])
const total = ref(0)
const filters = reactive({ status: '' })
const pagination = reactive({ page: 1, size: 20 })

function truncate(payload) {
  if (!payload) return '-'
  const str = typeof payload === 'string' ? payload : JSON.stringify(payload)
  return str.length > 80 ? str.slice(0, 80) + '...' : str
}

function formatJson(data) {
  if (!data) return '-'
  try {
    const obj = typeof data === 'string' ? JSON.parse(data) : data
    return JSON.stringify(obj, null, 2)
  } catch {
    return String(data)
  }
}

function handleExpand() {
  // no-op, expand handled by Element Plus
}

function handleFilter() {
  pagination.page = 1
  fetchNotifications()
}

async function fetchNotifications() {
  loading.value = true
  try {
    const params = { page: pagination.page, size: pagination.size }
    if (filters.status) params.status = filters.status
    const res = await listNotifications(params)
    notifications.value = res.items || []
    total.value = res.total || 0
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
  }
}

onMounted(fetchNotifications)
</script>

<style scoped>
.page-title {
  font-size: 20px;
  font-weight: 600;
  color: #1d2129;
}
.page-desc {
  font-size: 14px;
  color: #86909c;
  margin-top: 4px;
}
.content-card {
  background: #fff;
  border-radius: 12px;
  border: 1px solid #e5e6eb;
  padding: 24px;
  margin-top: 20px;
}
.filter-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}
.text-secondary {
  color: #86909c;
  font-size: 13px;
}
.payload-preview {
  font-family: 'SF Mono', 'Monaco', 'Menlo', monospace;
  font-size: 12px;
}
.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
.expand-content {
  padding: 16px 24px;
}
.expand-block {
  margin-bottom: 16px;
}
.expand-block h4 {
  font-size: 14px;
  font-weight: 600;
  color: #1d2129;
  margin: 0 0 8px 0;
}
.json-block {
  background: #f7f8fa;
  border: 1px solid #e5e6eb;
  border-radius: 8px;
  padding: 12px 16px;
  font-family: 'SF Mono', 'Monaco', 'Menlo', monospace;
  font-size: 13px;
  color: #1d2129;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
  max-height: 300px;
  overflow: auto;
}
</style>
