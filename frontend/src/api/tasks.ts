import type { PageResult, TaskDetail, TaskListItem, TaskPayload } from '../types/domain';
import { http } from './client';

export interface TaskQuery {
  keyword?: string;
  status?: string;
  districtIds?: number[];
  roadIds?: number[];
  priority?: string;
  source?: string;
  pipeSegmentId?: number;
  planFrom?: string;
  planTo?: string;
  page?: number;
  pageSize?: number;
}

function buildTaskQuery(query: TaskQuery): string {
  const params = new URLSearchParams();
  Object.entries({
    keyword: query.keyword,
    status: query.status,
    priority: query.priority,
    source: query.source,
    pipeSegmentId: query.pipeSegmentId,
    planFrom: query.planFrom,
    planTo: query.planTo,
    page: query.page,
    pageSize: query.pageSize
  }).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      params.set(key, String(value));
    }
  });
  // 任务按登记当时的层级快照多选过滤。
  query.districtIds?.forEach((id) => params.append('districtIds', String(id)));
  query.roadIds?.forEach((id) => params.append('roadIds', String(id)));
  const search = params.toString();
  return search ? `?${search}` : '';
}

export const taskApi = {
  list: (query: TaskQuery) => http.get<PageResult<TaskListItem>>(`/cleaning-tasks${buildTaskQuery(query)}`),
  detail: (id: number) => http.get<TaskDetail>(`/cleaning-tasks/${id}`),
  create: (payload: TaskPayload) => http.post<{ id: number }>('/cleaning-tasks', payload),
  update: (id: number, payload: TaskPayload) => http.put<{ id: number }>(`/cleaning-tasks/${id}`, payload),
  remove: (id: number) => http.del<{ id: number }>(`/cleaning-tasks/${id}`),
  start: (id: number) => http.post<{ id: number }>(`/cleaning-tasks/${id}/start`),
  complete: (id: number) => http.post<{ id: number }>(`/cleaning-tasks/${id}/complete`),
  cancel: (id: number, reason: string) => http.post<{ id: number }>(`/cleaning-tasks/${id}/cancel`, { reason })
};
