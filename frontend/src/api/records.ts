import type { PageResult, RecordDetail, RecordListItem, RecordPayload } from '../types/domain';
import { http } from './client';

export interface RecordQuery {
  keyword?: string;
  taskId?: number;
  segmentId?: number;
  districtIds?: number[];
  roadIds?: number[];
  method?: string;
  weather?: string;
  dateFrom?: string;
  dateTo?: string;
  page?: number;
  pageSize?: number;
}

function buildRecordQuery(query: RecordQuery): string {
  const params = new URLSearchParams();
  Object.entries({
    keyword: query.keyword,
    taskId: query.taskId,
    segmentId: query.segmentId,
    method: query.method,
    weather: query.weather,
    dateFrom: query.dateFrom,
    dateTo: query.dateTo,
    page: query.page,
    pageSize: query.pageSize
  }).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      params.set(key, String(value));
    }
  });
  // 记录按录入当时的层级快照多选过滤。
  query.districtIds?.forEach((id) => params.append('districtIds', String(id)));
  query.roadIds?.forEach((id) => params.append('roadIds', String(id)));
  const search = params.toString();
  return search ? `?${search}` : '';
}

export const recordApi = {
  list: (query: RecordQuery) => http.get<PageResult<RecordListItem>>(`/cleaning-records${buildRecordQuery(query)}`),
  detail: (id: number) => http.get<RecordDetail>(`/cleaning-records/${id}`),
  create: (payload: RecordPayload) => http.post<{ id: number }>('/cleaning-records', payload),
  update: (id: number, payload: RecordPayload) => http.put<{ id: number }>(`/cleaning-records/${id}`, payload),
  remove: (id: number) => http.del<{ id: number }>(`/cleaning-records/${id}`)
};
