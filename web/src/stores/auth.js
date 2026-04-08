import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import request from '@/utils/request'

export const useAuthStore = defineStore('auth', () => {
  const accessToken = ref(localStorage.getItem('kronos_access_token') || '')
  const refreshToken = ref(localStorage.getItem('kronos_refresh_token') || '')
  const user = ref(null)

  const isLoggedIn = computed(() => !!accessToken.value)

  function setTokens(access, refresh) {
    accessToken.value = access
    refreshToken.value = refresh
    localStorage.setItem('kronos_access_token', access)
    localStorage.setItem('kronos_refresh_token', refresh)
  }

  function clearTokens() {
    accessToken.value = ''
    refreshToken.value = ''
    user.value = null
    localStorage.removeItem('kronos_access_token')
    localStorage.removeItem('kronos_refresh_token')
  }

  async function login(username, password) {
    const res = await request.post('/auth/login', { username, password })
    setTokens(res.access_token, res.refresh_token)
    return res
  }

  async function logout() {
    try {
      await request.post('/auth/logout')
    } catch {
      // ignore logout API errors
    }
    clearTokens()
  }

  async function refresh() {
    try {
      const res = await request.post('/auth/refresh', {
        refresh_token: refreshToken.value,
      })
      setTokens(res.access_token, res.refresh_token || refreshToken.value)
      return true
    } catch {
      clearTokens()
      return false
    }
  }

  async function fetchUser() {
    try {
      const res = await request.get('/auth/me')
      user.value = res
      return res
    } catch {
      return null
    }
  }

  return {
    accessToken,
    refreshToken,
    user,
    isLoggedIn,
    login,
    logout,
    refresh,
    fetchUser,
    clearTokens,
  }
})
