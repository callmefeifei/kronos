import request from '@/utils/request'

// List users
export function listUsers(params) {
  return request.get('users', { params })
}

// Create user
export function createUser(data) {
  return request.post('users', data)
}

// Update user
export function updateUser(id, data) {
  return request.put(`users/${id}`, data)
}

// Delete user
export function deleteUser(id) {
  return request.delete(`users/${id}`)
}
