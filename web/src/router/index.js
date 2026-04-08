import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    component: () => import('@/layout/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/Dashboard.vue'),
        meta: { title: '仪表盘', icon: 'Odometer' },
      },
      {
        path: 'tasks',
        name: 'Tasks',
        component: () => import('@/views/Tasks.vue'),
        meta: { title: '任务管理', icon: 'List' },
      },
      {
        path: 'tasks/create',
        name: 'TaskCreate',
        component: () => import('@/views/TaskForm.vue'),
        meta: { title: '新建任务', hidden: true },
      },
      {
        path: 'tasks/:id',
        name: 'TaskDetail',
        component: () => import('@/views/TaskDetail.vue'),
        meta: { title: '任务详情', hidden: true },
      },
      {
        path: 'tasks/:id/edit',
        name: 'TaskEdit',
        component: () => import('@/views/TaskForm.vue'),
        meta: { title: '编辑任务', hidden: true },
      },
      {
        path: 'users',
        name: 'Users',
        component: () => import('@/views/Users.vue'),
        meta: { title: '用户管理', icon: 'User', adminOnly: true },
      },
      {
        path: 'notifications',
        name: 'Notifications',
        component: () => import('@/views/Notifications.vue'),
        meta: { title: '通知记录', icon: 'Bell' },
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/dashboard',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Auth guard
router.beforeEach((to, _from, next) => {
  const auth = useAuthStore()
  if (to.meta.public) {
    // Public route — redirect to dashboard if already logged in
    if (auth.isLoggedIn && to.name === 'Login') {
      next({ path: '/dashboard' })
    } else {
      next()
    }
  } else {
    // Protected route
    if (!auth.isLoggedIn) {
      next({ path: '/login', query: { redirect: to.fullPath } })
    } else {
      next()
    }
  }
})

export default router
