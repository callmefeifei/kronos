import request from '@/utils/request'

// List tasks with filters and pagination
export function listTasks(params) {
  return request.get('tasks', { params })
}

// Get task detail
export function getTask(id) {
  return request.get(`tasks/${id}`)
}

// Create task
export function createTask(data) {
  return request.post('tasks', data)
}

// Update task
export function updateTask(id, data) {
  return request.put(`tasks/${id}`, data)
}

// Delete task (soft)
export function deleteTask(id) {
  return request.delete(`tasks/${id}`)
}

// Run task now
export function runTask(id) {
  return request.post(`tasks/${id}/run`)
}

// Enable task
export function enableTask(id) {
  return request.post(`tasks/${id}/enable`)
}

// Disable task
export function disableTask(id) {
  return request.post(`tasks/${id}/disable`)
}

// Get task run history
export function listTaskRuns(id, params) {
  return request.get(`tasks/${id}/runs`, { params })
}
