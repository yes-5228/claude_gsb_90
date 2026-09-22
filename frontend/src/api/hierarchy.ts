import type { DistrictNode, HierarchyChangeLog, PageResult } from '../types/domain';
import { buildQuery, http } from './client';

export interface ChangeLogQuery {
  nodeType?: string;
  nodeId?: number;
  action?: string;
  batchId?: string;
  page?: number;
  pageSize?: number;
}

export interface MoveRoadsPayload {
  items: { roadId: number; districtId: number }[];
  operator?: string;
  remark?: string;
}

export interface ActorPayload {
  operator?: string;
  remark?: string;
}

export const hierarchyApi = {
  tree: () => http.get<DistrictNode[]>('/hierarchy/tree'),
  changeLogs: (query: ChangeLogQuery) =>
    http.get<PageResult<HierarchyChangeLog>>(`/hierarchy/change-logs${buildQuery({ ...query })}`),

  createDistrict: (name: string, extra: ActorPayload = {}) =>
    http.post<DistrictNode>('/hierarchy/districts', { name, ...extra }),
  renameDistrict: (id: number, name: string, extra: ActorPayload = {}) =>
    http.put<DistrictNode>(`/hierarchy/districts/${id}/rename`, { name, ...extra }),
  deleteDistrict: (id: number, extra: ActorPayload = {}) =>
    http.del<{ id: number }>(`/hierarchy/districts/${id}`, extra),

  createRoad: (districtId: number, name: string, extra: ActorPayload = {}) =>
    http.post<{ id: number }>('/hierarchy/roads', { districtId, name, ...extra }),
  renameRoad: (id: number, name: string, extra: ActorPayload = {}) =>
    http.put<{ id: number }>(`/hierarchy/roads/${id}/rename`, { name, ...extra }),
  moveRoad: (id: number, districtId: number, extra: ActorPayload = {}) =>
    http.put<{ id: number }>(`/hierarchy/roads/${id}/move`, { districtId, ...extra }),
  batchMoveRoads: (payload: MoveRoadsPayload) => http.post<{ count: number }>('/hierarchy/roads/batch-move', payload),
  deleteRoad: (id: number, extra: ActorPayload = {}) => http.del<{ id: number }>(`/hierarchy/roads/${id}`, extra)
};
