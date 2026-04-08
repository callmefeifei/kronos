<template>
  <div class="task-form">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ isEdit ? '编辑任务' : '新建任务' }}</h2>
        <p class="page-desc">{{ isEdit ? '修改任务配置' : '创建一个新的定时任务' }}</p>
      </div>
      <el-button @click="$router.back()">返回</el-button>
    </div>

    <div class="content-card">
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
        style="max-width: 640px"
      >
        <!-- Common fields -->
        <el-form-item label="任务名称" prop="name">
          <el-input v-model="form.name" placeholder="输入任务名称" maxlength="100" />
        </el-form-item>

        <el-form-item label="任务类型" prop="type">
          <el-select v-model="form.type" placeholder="选择任务类型" style="width: 100%">
            <el-option label="提醒" value="remind" />
            <el-option label="脚本" value="script" />
            <el-option label="Agent" value="agent" />
          </el-select>
        </el-form-item>

        <!-- Type-specific target field -->
        <el-form-item v-if="form.type === 'remind'" label="提醒内容" prop="target">
          <el-input
            v-model="form.target"
            type="textarea"
            :rows="4"
            placeholder="输入提醒消息内容"
          />
        </el-form-item>
        <el-form-item v-else-if="form.type === 'script'" label="脚本路径" prop="target">
          <el-input v-model="form.target" placeholder="如: /path/to/script.sh" />
        </el-form-item>
        <el-form-item v-else-if="form.type === 'agent'" label="Agent 指令" prop="target">
          <el-input
            v-model="form.target"
            type="textarea"
            :rows="4"
            placeholder="输入 Agent 执行指令"
          />
        </el-form-item>

        <el-divider />

        <!-- Schedule -->
        <el-form-item label="调度类型" prop="schedule_type">
          <el-select v-model="form.schedule_type" placeholder="选择调度类型" style="width: 100%">
            <el-option label="Cron 表达式" value="cron" />
            <el-option label="固定间隔" value="interval" />
            <el-option label="一次性" value="once" />
          </el-select>
        </el-form-item>

        <el-form-item label="调度表达式" prop="schedule_expr">
          <el-input v-model="form.schedule_expr" placeholder="如: */5 * * * * 或 30m 或 2024-12-01T10:00:00">
            <template #append>
              <el-tooltip content="cron: 标准cron表达式; interval: 如 30m, 1h; once: ISO时间" placement="top">
                <el-icon><QuestionFilled /></el-icon>
              </el-tooltip>
            </template>
          </el-input>
        </el-form-item>

        <el-divider />

        <!-- Execution settings -->
        <el-form-item label="超时时间(秒)" prop="timeout">
          <el-input-number v-model="form.timeout" :min="0" :max="86400" :step="60" />
        </el-form-item>

        <el-form-item label="重试次数" prop="retry_count">
          <el-input-number v-model="form.retry_count" :min="0" :max="10" />
        </el-form-item>

        <el-form-item label="重试间隔(秒)" prop="retry_interval">
          <el-input-number v-model="form.retry_interval" :min="0" :max="3600" :step="10" />
        </el-form-item>

        <el-divider />

        <!-- Notification -->
        <el-form-item label="通知时机">
          <el-checkbox-group v-model="form.notify_on">
            <el-checkbox label="success">成功</el-checkbox>
            <el-checkbox label="failure">失败</el-checkbox>
            <el-checkbox label="start">开始</el-checkbox>
          </el-checkbox-group>
        </el-form-item>

        <el-form-item label="通知渠道" prop="notify_channel">
          <el-select v-model="form.notify_channel" placeholder="选择通知渠道" clearable style="width: 100%">
            <el-option label="飞书" value="feishu" />
            <el-option label="企业微信" value="wecom" />
            <el-option label="邮件" value="email" />
            <el-option label="Webhook" value="webhook" />
          </el-select>
        </el-form-item>

        <!-- Actions -->
        <el-form-item>
          <el-button type="primary" :loading="submitting" @click="handleSubmit">
            {{ isEdit ? '保存修改' : '创建任务' }}
          </el-button>
          <el-button @click="$router.back()">取消</el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { QuestionFilled } from '@element-plus/icons-vue'
import { getTask, createTask, updateTask } from '@/api/task'

const route = useRoute()
const router = useRouter()
const formRef = ref(null)
const submitting = ref(false)

const isEdit = computed(() => !!route.params.id)

const form = reactive({
  name: '',
  type: '',
  target: '',
  schedule_type: 'cron',
  schedule_expr: '',
  timeout: 300,
  retry_count: 0,
  retry_interval: 60,
  notify_on: ['failure'],
  notify_channel: '',
})

const rules = {
  name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择任务类型', trigger: 'change' }],
  target: [{ required: true, message: '请输入目标内容', trigger: 'blur' }],
  schedule_type: [{ required: true, message: '请选择调度类型', trigger: 'change' }],
  schedule_expr: [{ required: true, message: '请输入调度表达式', trigger: 'blur' }],
}

async function loadTask() {
  if (!isEdit.value) return
  try {
    const data = await getTask(route.params.id)
    Object.assign(form, {
      name: data.name || '',
      type: data.type || '',
      target: data.target || '',
      schedule_type: data.schedule_type || 'cron',
      schedule_expr: data.schedule_expr || '',
      timeout: data.timeout ?? 300,
      retry_count: data.retry_count ?? 0,
      retry_interval: data.retry_interval ?? 60,
      notify_on: data.notify_on || ['failure'],
      notify_channel: data.notify_channel || '',
    })
  } catch {
    ElMessage.error('加载任务失败')
  }
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    const payload = { ...form }
    if (isEdit.value) {
      await updateTask(route.params.id, payload)
      ElMessage.success('任务已更新')
    } else {
      await createTask(payload)
      ElMessage.success('任务已创建')
    }
    router.push('/tasks')
  } catch {
    // error handled by interceptor
  } finally {
    submitting.value = false
  }
}

onMounted(loadTask)
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
</style>
