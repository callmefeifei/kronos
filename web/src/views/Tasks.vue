<template>
  <div class="tasks">
    <div class="page-header">
      <div>
        <h2 class="page-title">任务管理</h2>
        <p class="page-desc">查看和管理定时任务</p>
      </div>
      <el-button type="primary" @click="$router.push('/tasks/create')">
        <el-icon><Plus /></el-icon>
        新建任务
      </el-button>
    </div>

    <div class="content-card">
      <!-- Filters -->
      <div class="filter-bar">
        <el-input
          v-model="filters.search"
          placeholder="搜索任务名称"
          clearable
          style="width: 240px"
          @clear="handleSearch"
          @keyup.enter="handleSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select
          v-model="filters.type"
          placeholder="任务类型"
          clearable
          style="width: 140px"
          @change="handleSearch"
        >
          <el-option label="提醒" value="remind" />
          <el-option label="脚本" value="script" />
          <el-option label="Agent" value="agent" />
        </el-select>
      </div>

      <!-- Table -->
      <el-table
        v-loading="loading"
        :data="tasks"
        stripe
        style="width: 100%"
      >
        <el-table-column label="任务名称" min-width="180">
          <template #default="{ row }">
            <router-link :to="`/tasks/${row.id}`" class="task-name-link">
              {{ row.name }}
            </router-link>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="typeTagMap[row.type]" size="small" disable-transitions>
              {{ typeTextMap[row.type] || row.type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="调度" prop="schedule_expr" min-width="140">
          <template #default="{ row }">
            <span class="mono-text">{{ row.schedule_expr || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="上次状态" width="100">
          <template #default="{ row }">
            <el-tag
              v-if="row.last_status"
              :type="statusTagMap[row.last_status]"
              size="small"
              disable-transitions
            >
              {{ row.last_status }}
            </el-tag>
            <span v-else class="text-secondary">-</span>
          </template>
        </el-table-column>
        <el-table-column label="上次运行" width="170">
          <template #default="{ row }">
            <span class="text-secondary">{{ row.last_run_at || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="启用" width="80" align="center">
          <template #default="{ row }">
            <el-switch
              v-model="row.enabled"
              size="small"
              :loading="row._toggling"
              @change="(val) => handleToggleEnabled(row, val)"
            />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="$router.push(`/tasks/${row.id}/edit`)">
              编辑
            </el-button>
            <el-button link type="primary" size="small" @click="handleRunNow(row)">
              立即运行
            </el-button>
            <el-popconfirm
              title="确认删除此任务？"
              confirm-button-text="确认"
              cancel-button-text="取消"
              @confirm="handleDelete(row)"
            >
              <template #reference>
                <el-button link type="danger" size="small">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>

      <!-- Pagination -->
      <div class="pagination-wrap" v-if="total > 0">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.size"
          :total="total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          background
          @size-change="fetchTasks"
          @current-change="fetchTasks"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import { listTasks, deleteTask, runTask, enableTask, disableTask } from '@/api/task'

const typeTagMap = { remind: '', script: 'success', agent: 'warning' }
const typeTextMap = { remind: '提醒', script: '脚本', agent: 'Agent' }
const statusTagMap = { success: 'success', failed: 'danger', running: 'warning', skipped: 'info' }

const loading = ref(false)
const tasks = ref([])
const total = ref(0)
const filters = reactive({ search: '', type: '' })
const pagination = reactive({ page: 1, size: 20 })

async function fetchTasks() {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      size: pagination.size,
    }
    if (filters.search) params.search = filters.search
    if (filters.type) params.type = filters.type
    const res = await listTasks(params)
    tasks.value = (res.items || []).map((t) => ({ ...t, _toggling: false }))
    total.value = res.total || 0
  } catch {
    // error already handled by interceptor
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.page = 1
  fetchTasks()
}

async function handleToggleEnabled(row, val) {
  row._toggling = true
  try {
    if (val) {
      await enableTask(row.id)
      ElMessage.success('已启用')
    } else {
      await disableTask(row.id)
      ElMessage.success('已禁用')
    }
  } catch {
    row.enabled = !val // revert on failure
  } finally {
    row._toggling = false
  }
}

async function handleRunNow(row) {
  try {
    await ElMessageBox.confirm(`确认立即运行任务「${row.name}」？`, '立即运行', {
      confirmButtonText: '确认',
      cancelButtonText: '取消',
      type: 'info',
    })
    await runTask(row.id)
    ElMessage.success('任务已触发')
    fetchTasks()
  } catch {
    // cancelled or error
  }
}

async function handleDelete(row) {
  try {
    await deleteTask(row.id)
    ElMessage.success('已删除')
    fetchTasks()
  } catch {
    // error handled by interceptor
  }
}

onMounted(fetchTasks)
</script>

<script>
import { ElMessageBox } from 'element-plus'
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
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
.filter-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}
.task-name-link {
  color: #165dff;
  text-decoration: none;
  font-weight: 500;
}
.task-name-link:hover {
  text-decoration: underline;
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
.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
