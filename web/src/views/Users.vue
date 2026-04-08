<template>
  <div class="users">
    <div class="page-header">
      <div>
        <h2 class="page-title">用户管理</h2>
        <p class="page-desc">管理系统用户</p>
      </div>
      <el-button type="primary" @click="openCreate">
        <el-icon><Plus /></el-icon>
        新建用户
      </el-button>
    </div>

    <div class="content-card">
      <el-table v-loading="loading" :data="users" stripe style="width: 100%">
        <el-table-column label="用户名" prop="username" min-width="120" />
        <el-table-column label="显示名称" prop="display_name" min-width="140" />
        <el-table-column label="角色" width="100">
          <template #default="{ row }">
            <el-tag :type="row.role === 'admin' ? 'danger' : ''" size="small" disable-transitions>
              {{ row.role === 'admin' ? '管理员' : '用户' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small" disable-transitions>
              {{ row.status === 'active' ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" prop="created_at" width="170">
          <template #default="{ row }">
            <span class="text-secondary">{{ row.created_at || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="openEdit(row)">编辑</el-button>
            <el-popconfirm
              title="确认删除此用户？"
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
    </div>

    <UserForm
      v-model:visible="formVisible"
      :user="editingUser"
      @success="fetchUsers"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { listUsers, deleteUser } from '@/api/user'
import UserForm from './UserForm.vue'

const loading = ref(false)
const users = ref([])
const formVisible = ref(false)
const editingUser = ref(null)

async function fetchUsers() {
  loading.value = true
  try {
    const res = await listUsers()
    users.value = res.items || res || []
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingUser.value = null
  formVisible.value = true
}

function openEdit(row) {
  editingUser.value = { ...row }
  formVisible.value = true
}

async function handleDelete(row) {
  try {
    await deleteUser(row.id)
    ElMessage.success('已删除')
    fetchUsers()
  } catch {
    // handled by interceptor
  }
}

onMounted(fetchUsers)
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
.text-secondary {
  color: #86909c;
  font-size: 13px;
}
</style>
