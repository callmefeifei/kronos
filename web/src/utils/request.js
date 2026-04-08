import axios from 'axios'
import { ElMessage } from 'element-plus'

const request = axios.create({
  baseURL: '/api/v1/',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor — attach JWT token
request.interceptors.request.use(
  (config) => {
    // Import dynamically to avoid circular dependency
    const token = localStorage.getItem('kronos_access_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

// Response interceptor — unwrap unified response, handle errors
request.interceptors.response.use(
  (response) => {
    const res = response.data
    // Unified response: { code: 0, message: "ok", data: {...} }
    if (res.code !== undefined) {
      if (res.code === 0) {
        return res.data
      }
      // Business error
      ElMessage.error(res.message || 'Request failed')
      return Promise.reject(new Error(res.message || 'Request failed'))
    }
    // Non-unified response, return as-is
    return res
  },
  (error) => {
    const status = error.response?.status
    if (status === 401) {
      // Token expired or invalid — clear and redirect to login
      localStorage.removeItem('kronos_access_token')
      localStorage.removeItem('kronos_refresh_token')
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
      return Promise.reject(error)
    }

    const msg =
      error.response?.data?.message || error.message || 'Network error'
    ElMessage.error(msg)
    return Promise.reject(error)
  }
)

export default request
