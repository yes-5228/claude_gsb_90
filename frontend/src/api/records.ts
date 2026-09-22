import type { PageResult, RecordDetail, RecordListItem, RecordPayload } from '../types/domain';
import { buildQuery, http } from './client';

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

export const recordApi = {
  list: (query: RecordQuery) =>
    http.get<PageResult<RecordListItem>>(
      `/cleaning-records${buildQuery({
        keyword: query.keyword,
        taskId: query.taskId,
        segmentId: query.segmentId,
        districtIds: query.districtIds,
        roadIds: query.roadIds,
        method: query.method,
        weather: query.weather,
        dateFrom: query.dateFrom,
        dateTo: query.dateTo,
        page: query.page,
        pageSize: query.pageSize
      })}`
    ),
  detail: (id: number) => http.get<RecordDetail>(`/cleaning-records/${id}`),
  create: (payload: RecordPayload) => http.post<{ id: number }>('/cleaning-records', payload),
  update: (id: number, payload: RecordPayload) => http.put<{ id: number }>(`/cleaning-records/${id}`, payload),
  remove: (id: number) => http.del<{ id: number }>(`/cleaning-records/${id}`)
};
