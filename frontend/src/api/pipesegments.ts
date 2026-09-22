import type {
  PageResult,
  PipeSegment,
  SegmentDetail,
  SegmentHistoryItem,
  SegmentOptions,
  SegmentPayload
} from '../types/domain';
import { buildQuery, http } from './client';

export interface SegmentQuery {
  keyword?: string;
  districtIds?: number[];
  roadIds?: number[];
  pipeType?: string;
  status?: string;
  page?: number;
  pageSize?: number;
}

function buildSegmentQuery(query: SegmentQuery): string {
  const params = new URLSearchParams();
  Object.entries({
    keyword: query.keyword,
    pipeType: query.pipeType,
    status: query.status,
    page: query.page,
    pageSize: query.pageSize
  }).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      params.set(key, String(value));
    }
  });
  // 多选 ID 用重复参数传递，后端按集合过滤。
  query.districtIds?.forEach((id) => params.append('districtIds', String(id)));
  query.roadIds?.forEach((id) => params.append('roadIds', String(id)));
  const search = params.toString();
  return search ? `?${search}` : '';
}

export const segmentApi = {
  list: (query: SegmentQuery) => http.get<PageResult<PipeSegment>>(`/pipe-segments${buildSegmentQuery(query)}`),
  detail: (id: number) => http.get<SegmentDetail>(`/pipe-segments/${id}`),
  history: (id: number) => http.get<SegmentHistoryItem[]>(`/pipe-segments/${id}/cleaning-history`),
  options: (keyword?: string) => http.get<SegmentOptions>(`/pipe-segments/options${buildQuery({ keyword })}`),
  create: (payload: SegmentPayload) => http.post<PipeSegment>('/pipe-segments', payload),
  update: (id: number, payload: SegmentPayload) => http.put<PipeSegment>(`/pipe-segments/${id}`, payload),
  remove: (id: number) => http.del<{ id: number }>(`/pipe-segments/${id}`)
};
