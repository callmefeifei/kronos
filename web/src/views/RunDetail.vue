<template>
  <div class="run-detail">
    <div class="page-header">
      <div>
        <h2 class="page-title">运行详情</h2>
        <p class="page-desc">查看任务运行结果和输出日志</p>
      </div>
      <el-button @click="$router.back()">
        <el-icon><ArrowLeft /></el-icon>
        返回
      </el-button>
    </div>

    <div class="content-card" v-loading="loading">
      <template v-if="run">
        <el-descriptions :column="3" border>
          <el-descriptions-item label="运行ID">{{ run.id }}</el-descriptions-item>
          <el-descriptions-item label="任务名称">
            <router-link v-if="run.task_id" :to="`/tasks/${run.task_id}`" class="link">
              {{ run.task_name || run.task_id }}
            </router-link>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="statusTagMap[run.status]" size="small" disable-transitions>
              {{ run.status }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="触发方式">{{ run.triggered_by || '-' }}</el-descriptions-item>
          <el-descriptions-item label="开始时间">{{ run.started_at || '-' }}</el-descriptions-item>
          <el-descriptions-item label="运行时长">{{ formatDuration(run.duration) }}</el-descriptions-item>
          <el-descriptions-item label="退出码">
            <span :class="run.exit_code === 0 ? 'exit-ok' : 'exit-err'">
              {{ run.exit_code ?? '-' }}
            </span>
          </el-descriptions-item>
          <el-descriptions-item label="完成时间">{{ run.finished_at || '-' }}</el-descriptions-item>
        </el-descriptions>

        <!-- Error section -->
        <div v-if="run.error" class="error-section">
          <h3 class="section-title">错误信息</h3>
          <div class="error-block">{{ run.error }}</div>
        </div>

        <!-- Output log -->
        <div class="log-section">
          <div class="section-header">
            <h3 class="section-title">输出日志</h3>
            <el-button size="small" @click="fetchOutput" :loading="outputLoading">
              刷新
            </el-button>
          </div>
          <div class="log-viewer" v-loading="outputLoading">
            <pre v-if="output">{{ output }}</pre>
            <div v-else class="log-empty">暂无输出</div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import { getRun, getRunOutput } from '@/api/run'

const route = useRoute()
const statusTagMap = { success: 'success', failed: 'danger', running: 'warning', skipped: 'info' }

const loading = ref(false)
const outputLoading = ref(false)
const run = ref(null)
const output = ref('')

function formatDuration(seconds) {
  if (seconds == null) return '-'
  if (seconds < 1) return `${Math.round(seconds * 1000)}ms`
  if (seconds < 60) return `${seconds.toFixed(1)}s`
  const m = Math.floor(seconds / 60)
  const s = Math.round(seconds % 60)
  return `${m}m ${s}s`
}

async function fetchDetail() {
  loading.value = true
  try {
    run.value = await getRun(route.params.id)
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
  }
}

async function fetchOutput() {
  outputLoading.value = true
  try {
    output.value = await getRunOutput(route.params.id)
  } catch {
    output.value = ''
  } finally {
    outputLoading.value = false
  }
}

onMounted(() => {
  fetchDetail()
  fetchOutput()
})
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
.link {
  color: #165dff;
  text-decoration: none;
  font-weight: 500;
}
.link:hover {
  text-decoration: underline;
}
.exit-ok {
  color: #00b42a;
  font-weight: 600;
}
.exit-err {
  color: #f53f3f;
  font-weight: 600;
}
.section-title {
  font-size: 16px;
  font-weight: 600;
  color: #1d2129;
  margin: 0;
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 24px;
  margin-bottom: 12px;
}
.error-section {
  margin-top: 24px;
}
.error-section .section-title {
  margin-bottom: 12px;
}
.error-block {
  background: #fff0f0;
  border: 1px solid #ffccc7;
  border-radius: 8px;
  padding: 16px;
  font-family: 'SF Mono', 'Monaco', 'Menlo', monospace;
  font-size: 14px;
  color: #cf1322;
  white-space: pre-wrap;
  word-break: break-all;
}
.log-section {
  margin-top: 0;
}
.log-viewer {
  background: #1e1e1e;
  border-radius: 8px;
  padding: 16px;
  max-height: 600px;
  overflow: auto;
}
.log-viewer pre {
  margin: 0;
  font-family: 'SF Mono', 'Monaco', 'Menlo', monospace;
  font-size: 14px;
  color: #fff;
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.6;
}
.log-empty {
  color: #666;
  font-size: 14px;
  text-align: center;
  padding: 40px 0;
}
</style>
