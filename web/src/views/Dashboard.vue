<template>
  <div class="dashboard">
    <h2 class="page-title">仪表盘</h2>
    <p class="page-desc">任务调度系统概览</p>

    <!-- Stat Cards -->
    <el-row :gutter="16" class="stat-row">
      <el-col :span="6" v-for="card in statCards" :key="card.key">
        <div class="stat-card" :style="{ borderLeftColor: card.color }">
          <div class="stat-icon" :style="{ backgroundColor: card.bgColor }">
            <el-icon :size="22" :color="card.color"><component :is="card.icon" /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ card.value }}</div>
            <div class="stat-label">{{ card.label }}</div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- Recent Execution Timeline -->
    <div class="timeline-section">
      <div class="section-header">
        <h3 class="section-title">最近执行记录</h3>
        <el-button text :icon="Refresh" @click="fetchAll" :loading="loading">刷新</el-button>
      </div>

      <el-table
        v-if="timeline.length > 0"
        :data="timeline"
        style="width: 100%"
        class="timeline-table"
        :header-cell-style="{ background: '#f7f8fa', color: '#1d2129', fontWeight: 600 }"
      >
        <el-table-column prop="task_name" label="任务名称" min-width="180" />
        <el-table-column prop="status" label="状态" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small" effect="plain">
              {{ statusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="duration_ms" label="耗时" width="120" align="center">
          <template #default="{ row }">
            {{ formatDuration(row.duration_ms) }}
          </template>
        </el-table-column>
        <el-table-column prop="triggered_by" label="触发方式" width="120" align="center" />
        <el-table-column prop="started_at" label="执行时间" width="180" align="center">
          <template #default="{ row }">
            <el-tooltip :content="row.started_at" placement="top">
              <span>{{ relativeTime(row.started_at) }}</span>
            </el-tooltip>
          </template>
        </el-table-column>
      </el-table>

      <!-- Empty state -->
      <div v-else-if="!loading" class="empty-state">
        <el-icon :size="48" color="#c0c4cc"><Calendar /></el-icon>
        <p class="empty-title">暂无执行记录</p>
        <p class="empty-desc">创建任务后，执行记录将在这里展示</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { Histogram, Timer, SuccessFilled, VideoPlay, Refresh, Calendar } from '@element-plus/icons-vue'
import { getDashboardStats, getDashboardTimeline } from '@/api/dashboard'

const loading = ref(false)
const stats = ref({
  total_tasks: 0,
  active_tasks: 0,
  today_runs: 0,
  success_rate: 0,
})
const timeline = ref([])

const statCards = computed(() => [
  {
    key: 'total',
    label: '任务总数',
    value: stats.value.total_tasks,
    icon: Histogram,
    color: '#165dff',
    bgColor: '#e8f3ff',
  },
  {
    key: 'today',
    label: '今日执行',
    value: stats.value.today_runs,
    icon: Timer,
    color: '#0fc6c2',
    bgColor: '#e8fffb',
  },
  {
    key: 'rate',
    label: '成功率',
    value: stats.value.success_rate + '%',
    icon: SuccessFilled,
    color: '#00b42a',
    bgColor: '#e8ffea',
  },
  {
    key: 'active',
    label: '活跃任务',
    value: stats.value.active_tasks,
    icon: VideoPlay,
    color: '#ff7d00',
    bgColor: '#fff7e8',
  },
])

const statusMap = {
  success: { type: 'success', label: '成功' },
  failed: { type: 'danger', label: '失败' },
  timeout: { type: 'warning', label: '超时' },
  running: { type: '', label: '运行中' },
  skipped: { type: 'info', label: '跳过' },
}

function statusType(status) {
  return statusMap[status]?.type ?? 'info'
}

function statusLabel(status) {
  return statusMap[status]?.label ?? status
}

function formatDuration(ms) {
  if (ms == null) return '-'
  if (ms < 1000) return ms + 'ms'
  const sec = (ms / 1000).toFixed(1)
  if (sec < 60) return sec + 's'
  const min = Math.floor(sec / 60)
  const remSec = Math.round(sec % 60)
  return min + 'm ' + remSec + 's'
}

function relativeTime(dateStr) {
  if (!dateStr) return '-'
  const now = Date.now()
  const then = new Date(dateStr).getTime()
  const diff = now - then
  if (diff < 0) return '刚刚'
  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return '刚刚'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return minutes + ' 分钟前'
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return hours + ' 小时前'
  const days = Math.floor(hours / 24)
  if (days < 30) return days + ' 天前'
  return dateStr.slice(0, 10)
}

async function fetchStats() {
  try {
    const data = await getDashboardStats()
    stats.value = data
  } catch {
    // silently ignore — stats remain at defaults
  }
}

async function fetchTimeline() {
  try {
    const data = await getDashboardTimeline()
    timeline.value = data?.items || []
  } catch {
    // silently ignore
  }
}

async function fetchAll() {
  loading.value = true
  await Promise.all([fetchStats(), fetchTimeline()])
  loading.value = false
}

let refreshTimer = null

onMounted(() => {
  fetchAll()
  refreshTimer = setInterval(fetchAll, 30000)
})

onBeforeUnmount(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
})
</script>

<style scoped>
.dashboard {
  padding: 0;
}

.page-title {
  font-size: 20px;
  font-weight: 600;
  color: #1d2129;
  margin: 0;
}

.page-desc {
  font-size: 14px;
  color: #86909c;
  margin-top: 4px;
}

.stat-row {
  margin-top: 20px;
}

.stat-card {
  background: #fff;
  border-radius: 12px;
  border: 1px solid #e5e6eb;
  border-left: 3px solid #165dff;
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  transition: border-color 0.2s;
}

.stat-icon {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-info {
  flex: 1;
  min-width: 0;
}

.stat-value {
  font-size: 26px;
  font-weight: 700;
  color: #1d2129;
  line-height: 1.2;
}

.stat-label {
  font-size: 13px;
  color: #86909c;
  margin-top: 4px;
}

.timeline-section {
  margin-top: 24px;
  background: #fff;
  border-radius: 12px;
  border: 1px solid #e5e6eb;
  padding: 20px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: #1d2129;
  margin: 0;
}

.timeline-table {
  border-radius: 8px;
  overflow: hidden;
}

.empty-state {
  text-align: center;
  padding: 60px 0;
}

.empty-title {
  font-size: 15px;
  color: #1d2129;
  margin-top: 16px;
  font-weight: 500;
}

.empty-desc {
  font-size: 13px;
  color: #86909c;
  margin-top: 4px;
}
</style>
