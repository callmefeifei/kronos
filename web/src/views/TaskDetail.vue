<template>
  <div class="task-detail">
    <div class="page-header">
      <div>
        <h2 class="page-title">任务详情</h2>
        <p class="page-desc">查看任务配置与执行历史</p>
      </div>
      <div class="header-actions">
        <el-button @click="$router.push(`/tasks/${taskId}/edit`)">编辑</el-button>
        <el-button type="primary" @click="handleRunNow">立即运行</el-button>
        <el-button @click="$router.push('/tasks')">返回列表</el-button>
      </div>
    </div>

    <!-- Task Info Card -->
    <div class="content-card" v-loading="loadingTask">
      <el-descriptions :column="2" border>
        <el-descriptions-item label="任务名称">{{ task.name }}</el-descriptions-item>
        <el-descriptions-item label="类型">
          <el-tag :type="typeTagMap[task.type]" size="small" disable-transitions>
            {{ typeTextMap[task.type] || task.type || '-' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="调度类型">{{ task.schedule_type || '-' }}</el-descriptions-item>
        <el-descriptions-item label="调度表达式">
          <span class="mono-text">{{ task.schedule_expr || '-' }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="超时时间">{{ task.timeout ? task.timeout + 's' : '-' }}</el-descriptions-item>
        <el-descriptions-item label="重试">
          {{ task.retry_count || 0 }} 次，间隔 {{ task.retry_interval || 0 }}s
        </el-descriptions-item>
        <el-descriptions-item label="通知时机">
          {{ (task.notify_on || []).join(', ') || '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="通知渠道">{{ task.notify_channel || '-' }}</el-descriptions-item>
        <el-descriptions-item label="启用状态">
          <el-tag :type="task.enabled ? 'success' : 'info'" size="small">
            {{ task.enabled ? '已启用' : '已禁用' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ task.created_at || '-' }}</el-descriptions-item>
        <el-descriptions-item label="目标" :span="2">
          <div class="target-content">{{ task.target || '-' }}</div>
        </el-descriptions-item>
      </el-descriptions>
    </div>

    <!-- Run History -->
    <div class="content-card" style="margin-top: 16px">
      <h3 class="section-title">执行历史</h3>
      <el-table
        v-loading="loadingRuns"
        :data="runs"
        stripe
        style="width: 100%"
      >
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusTagMap[row.status]" size="small" disable-transitions>
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="触发方式" prop="triggered_by" width="120">
          <template #default="{ row }">
            <span class="text-secondary">{{ row.triggered_by || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="开始时间" prop="started_at" width="180">
          <template #default="{ row }">
            <span class="text-secondary">{{ row.started_at || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="耗时" width="120">
          <template #default="{ row }">
            <span class="text-secondary">{{ formatDuration(row.duration) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <router-link :to="`/tasks/${taskId}/runs/${row.id}`" class="detail-link">
              查看详情
            </router-link>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrap" v-if="runsTotal > 0">
        <el-pagination
          v-model:current-page="runsPagination.page"
          v-model:page-size="runsPagination.size"
          :total="runsTotal"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          background
          @size-change="fetchRuns"
          @current-change="fetchRuns"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getTask, runTask, listTaskRuns } from '@/api/task'

const route = useRoute()
const taskId = computed(() => route.params.id)

const typeTagMap = { remind: '', script: 'success', agent: 'warning' }
const typeTextMap = { remind: '提醒', script: '脚本', agent: 'Agent' }
const statusTagMap = { success: 'success', failed: 'danger', running: 'warning', skipped: 'info' }

const loadingTask = ref(false)
const loadingRuns = ref(false)
const task = ref({})
const runs = ref([])
const runsTotal = ref(0)
const runsPagination = reactive({ page: 1, size: 10 })

function formatDuration(seconds) {
  if (seconds == null) return '-'
  if (seconds < 60) return `${seconds}s`
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return s > 0 ? `${m}m ${s}s` : `${m}m`
}

async function fetchTask() {
  loadingTask.value = true
  try {
    task.value = await getTask(taskId.value)
  } catch {
    // handled by interceptor
  } finally {
    loadingTask.value = false
  }
}

async function fetchRuns() {
  loadingRuns.value = true
  try {
    const res = await listTaskRuns(taskId.value, {
      page: runsPagination.page,
      size: runsPagination.size,
    })
    runs.value = res.items || []
    runsTotal.value = res.total || 0
  } catch {
    // handled by interceptor
  } finally {
    loadingRuns.value = false
  }
}

async function handleRunNow() {
  try {
    await ElMessageBox.confirm(
      `确认立即运行任务「${task.value.name || ''}」？`,
      '立即运行',
      { confirmButtonText: '确认', cancelButtonText: '取消', type: 'info' }
    )
    await runTask(taskId.value)
    ElMessage.success('任务已触发')
    fetchRuns()
  } catch {
    // cancelled or error
  }
}

onMounted(() => {
  fetchTask()
  fetchRuns()
})
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}
.header-actions {
  display: flex;
  gap: 8px;
}
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
.section-title {
  font-size: 16px;
  font-weight: 600;
  color: #1d2129;
  margin-bottom: 16px;
}
.mono-text {
  font-family: 'SF Mono', 'Monaco', 'Menlo', monospace;
  font-size: 13px;
  color: #4e5969;
}
.text-secondary {
  color: #86909c;
  font-size: 13px;
}
.target-content {
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 13px;
  color: #4e5969;
}
.detail-link {
  color: #165dff;
  text-decoration: none;
  font-size: 13px;
}
.detail-link:hover {
  text-decoration: underline;
}
.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
