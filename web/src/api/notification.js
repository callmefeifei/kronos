import request from '@/utils/request'

// List notifications
export function listNotifications(params) {
  return request.get('notifications', { params })
}
