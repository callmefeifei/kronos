<template>
  <el-container class="main-layout">
    <!-- Sidebar -->
    <el-aside :width="isCollapsed ? '64px' : '220px'" class="sidebar">
      <div class="sidebar-logo" @click="$router.push('/dashboard')">
        <el-icon :size="24" color="#165dff"><Timer /></el-icon>
        <span v-show="!isCollapsed" class="logo-text">Kronos</span>
      </div>
      <el-menu
        :default-active="activeMenu"
        :collapse="isCollapsed"
        :collapse-transition="false"
        class="sidebar-menu"
        router
      >
        <template v-for="item in menuItems" :key="item.path">
          <el-menu-item
            v-if="!item.adminOnly || isAdmin"
            :index="item.path"
          >
            <el-icon><component :is="item.icon" /></el-icon>
            <template #title>{{ item.title }}</template>
          </el-menu-item>
        </template>
      </el-menu>
    </el-aside>

    <!-- Main area -->
    <el-container>
      <!-- Header -->
      <el-header class="header" height="56px">
        <div class="header-left">
          <el-icon
            class="collapse-btn"
            :size="20"
            @click="isCollapsed = !isCollapsed"
          >
            <Fold v-if="!isCollapsed" />
            <Expand v-else />
          </el-icon>
        </div>
        <div class="header-right">
          <el-dropdown trigger="click" @command="handleCommand">
            <span class="user-info">
              <el-avatar :size="32" class="user-avatar">
                {{ userInitial }}
              </el-avatar>
              <span class="username">{{ displayName }}</span>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">
                  <el-icon><SwitchButton /></el-icon>
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <!-- Content -->
      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const isCollapsed = ref(false)

const menuItems = [
  { path: '/dashboard', title: '仪表盘', icon: 'Odometer' },
  { path: '/tasks', title: '任务管理', icon: 'List' },
  { path: '/users', title: '用户管理', icon: 'User', adminOnly: true },
  { path: '/notifications', title: '通知记录', icon: 'Bell' },
]

const activeMenu = computed(() => {
  // Match /tasks/:id back to /tasks
  if (route.path.startsWith('/tasks/')) return '/tasks'
  return route.path
})

const isAdmin = computed(() => {
  return auth.user?.is_admin === true
})

const displayName = computed(() => {
  return auth.user?.display_name || auth.user?.username || 'User'
})

const userInitial = computed(() => {
  return displayName.value.charAt(0).toUpperCase()
})

async function handleCommand(cmd) {
  if (cmd === 'logout') {
    await auth.logout()
    router.push('/login')
  }
}

// Fetch user info on mount
auth.fetchUser()
</script>

<style scoped>
.main-layout {
  height: 100vh;
  overflow: hidden;
}

.sidebar {
  background: var(--color-white);
  border-right: 1px solid var(--color-border);
  overflow-y: auto;
  transition: width 0.2s;
}

.sidebar-logo {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  cursor: pointer;
  border-bottom: 1px solid var(--color-border);
}

.logo-text {
  font-size: 18px;
  font-weight: 700;
  color: var(--color-primary);
  letter-spacing: 1px;
}

.sidebar-menu {
  border-right: none;
}

.sidebar-menu .el-menu-item {
  height: 36px;
  line-height: 36px;
  margin: 4px 8px;
  border-radius: 4px;
}

.sidebar-menu .el-menu-item.is-active {
  background-color: var(--color-primary-light);
  color: var(--color-primary);
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--color-white);
  border-bottom: 1px solid var(--color-border);
  padding: 0 20px;
}

.header-left {
  display: flex;
  align-items: center;
}

.collapse-btn {
  cursor: pointer;
  color: var(--color-text-secondary);
  transition: color 0.2s;
}

.collapse-btn:hover {
  color: var(--color-primary);
}

.header-right {
  display: flex;
  align-items: center;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  color: var(--color-text);
}

.user-avatar {
  background-color: var(--color-primary);
  color: var(--color-white);
  font-size: 14px;
}

.username {
  font-size: 14px;
}

.main-content {
  background: var(--color-bg);
  overflow-y: auto;
  padding: 20px;
}

/* Fix Element Plus primary color in sidebar */
:deep(.el-menu-item:hover) {
  background-color: #f2f3f5;
}
:deep(.el-menu-item.is-active) {
  background-color: #e8f3ff !important;
  color: #165dff !important;
}
</style>
