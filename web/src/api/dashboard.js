import request from '@/utils/request'

/** Get dashboard statistics */
export function getDashboardStats() {
  return request.get('dashboard/stats')
}

/** Get recent execution timeline */
export function getDashboardTimeline() {
  return request.get('dashboard/timeline')
}
