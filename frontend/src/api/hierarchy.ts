import type {
  BatchAdjustPayload,
  BatchAdjustResult,
  HierarchyChangeLog,
  HierarchyRefCounts,
  HierarchyTree,
  PageResult
} from '../types/domain';
import { buildQuery, http } from './client';

export interface ChangeLogQuery {
  nodeType?: string;
  action?: string;
  batchId?: string;
  page?: number;
  pageSize?: number;
}

export const hierarchyApi = {
  tree: () => http.get<HierarchyTree>('/hierarchy/tree'),
  createDistrict: (payload: { name: string; reason?: string; operator?: string }) =>
    http.post<unknown>('/hierarchy/districts', payload),
  renameDistrict: (id: number, payload: { name: string; reason?: string; operator?: string }) =>
    http.put<unknown>(`/hierarchy/districts/${id}/rename`, payload),
  deleteDistrict: (id: number, payload?: { reason?: string; operator?: string }) =>
    http.del<{ id: number }>(`/hierarchy/districts/${id}`, payload),
  districtRefs: (id: number) => http.get<HierarchyRefCounts>(`/hierarchy/districts/${id}/refs`),
  createRoad: (payload: { districtId: number; name: string; reason?: string; operator?: string }) =>
    http.post<unknown>('/hierarchy/roads', payload),
  renameRoad: (id: number, payload: { name: string; reason?: string; operator?: string }) =>
    http.put<unknown>(`/hierarchy/roads/${id}/rename`, payload),
  moveRoad: (id: number, payload: { districtId: number; reason?: string; operator?: string }) =>
    http.put<unknown>(`/hierarchy/roads/${id}/move`, payload),
  deleteRoad: (id: number, payload?: { reason?: string; operator?: string }) =>
    http.del<{ id: number }>(`/hierarchy/roads/${id}`, payload),
  roadRefs: (id: number) => http.get<HierarchyRefCounts>(`/hierarchy/roads/${id}/refs`),
  batch: (payload: BatchAdjustPayload) => http.post<BatchAdjustResult>('/hierarchy/adjustments', payload),
  logs: (query: ChangeLogQuery) =>
    http.get<PageResult<HierarchyChangeLog>>(`/hierarchy/change-logs${buildQuery({ ...query })}`)
};
