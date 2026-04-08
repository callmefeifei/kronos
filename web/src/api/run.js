import request from '@/utils/request'
import axios from 'axios'

// Get run detail
export function getRun(id) {
  return request.get(`runs/${id}`)
}

// Get run output (plain text)
export function getRunOutput(id) {
  const token = localStorage.getItem('kronos_access_token')
  return axios.get(`/api/v1/runs/${id}/output`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    responseType: 'text',
    transformResponse: [(data) => data],
  }).then((res) => res.data)
}
