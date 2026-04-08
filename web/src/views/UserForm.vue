<template>
  <el-dialog
    :title="isEdit ? '编辑用户' : '新建用户'"
    :model-value="visible"
    width="480px"
    @close="$emit('update:visible', false)"
    destroy-on-close
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-width="80px"
      @submit.prevent
    >
      <el-form-item label="用户名" prop="username">
        <el-input v-model="form.username" :disabled="isEdit" placeholder="请输入用户名" />
      </el-form-item>
      <el-form-item label="显示名称" prop="display_name">
        <el-input v-model="form.display_name" placeholder="请输入显示名称" />
      </el-form-item>
      <el-form-item label="密码" prop="password">
        <el-input
          v-model="form.password"
          type="password"
          show-password
          :placeholder="isEdit ? '留空则不修改密码' : '请输入密码'"
        />
      </el-form-item>
      <el-form-item label="角色" prop="role">
        <el-select v-model="form.role" style="width: 100%">
          <el-option label="管理员" value="admin" />
          <el-option label="普通用户" value="user" />
        </el-select>
      </el-form-item>
      <el-form-item label="状态" prop="status">
        <el-select v-model="form.status" style="width: 100%">
          <el-option label="启用" value="active" />
          <el-option label="禁用" value="disabled" />
        </el-select>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="$emit('update:visible', false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">
        确定
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { createUser, updateUser } from '@/api/user'

const props = defineProps({
  visible: Boolean,
  user: { type: Object, default: null },
})
const emit = defineEmits(['update:visible', 'success'])

const isEdit = computed(() => !!props.user?.id)
const formRef = ref(null)
const submitting = ref(false)

const form = reactive({
  username: '',
  display_name: '',
  password: '',
  role: 'user',
  status: 'active',
})

const rules = computed(() => ({
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  display_name: [{ required: true, message: '请输入显示名称', trigger: 'blur' }],
  password: isEdit.value
    ? []
    : [{ required: true, message: '请输入密码', trigger: 'blur' }],
  role: [{ required: true, message: '请选择角色', trigger: 'change' }],
  status: [{ required: true, message: '请选择状态', trigger: 'change' }],
}))

watch(
  () => props.visible,
  (val) => {
    if (val && props.user) {
      form.username = props.user.username || ''
      form.display_name = props.user.display_name || ''
      form.password = ''
      form.role = props.user.role || 'user'
      form.status = props.user.status || 'active'
    } else if (val) {
      form.username = ''
      form.display_name = ''
      form.password = ''
      form.role = 'user'
      form.status = 'active'
    }
  }
)

async function handleSubmit() {
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  submitting.value = true
  try {
    const data = { ...form }
    if (isEdit.value && !data.password) {
      delete data.password
    }
    if (isEdit.value) {
      await updateUser(props.user.id, data)
      ElMessage.success('用户已更新')
    } else {
      await createUser(data)
      ElMessage.success('用户已创建')
    }
    emit('update:visible', false)
    emit('success')
  } catch {
    // handled by interceptor
  } finally {
    submitting.value = false
  }
}
</script>
