// 片区-道路层级在应用启动时拉取一次，各页面通过 useHierarchy 读取同一棵树，
// 保证管段台账、任务、记录与看板使用同一套层级与名称。
import { createContext, useCallback, useContext, useMemo, type ReactNode } from 'react';
import { hierarchyApi } from '../api/hierarchy';
import { useAsync } from '../hooks/useAsync';
import type { DistrictNode, RoadNode } from '../types/domain';

interface HierarchyValue {
  districts: DistrictNode[];
  loading: boolean;
  error: string;
  reload: () => void;
  /** 全部道路的扁平列表。 */
  roads: RoadNode[];
  /** 按道路 ID 查道路。 */
  roadById: (id: number) => RoadNode | undefined;
  /** 按片区 ID 查片区名。 */
  districtName: (id: number) => string;
  /** 按道路 ID 取 “片区 / 道路” 完整路径。 */
  roadPath: (roadId: number, districtId?: number) => { district: string; road: string };
}

const HierarchyContext = createContext<HierarchyValue | null>(null);

export function HierarchyProvider({ children }: { children: ReactNode }) {
  const { data, loading, error, reload } = useAsync<DistrictNode[]>(() => hierarchyApi.tree(), []);

  const districts = useMemo(() => data ?? [], [data]);
  const roads = useMemo(() => districts.flatMap((d) => d.roads), [districts]);

  const roadMap = useMemo(() => {
    const map = new Map<number, RoadNode>();
    roads.forEach((road) => map.set(road.id, road));
    return map;
  }, [roads]);

  const districtMap = useMemo(() => {
    const map = new Map<number, DistrictNode>();
    districts.forEach((d) => map.set(d.id, d));
    return map;
  }, [districts]);

  const roadById = useCallback((id: number) => roadMap.get(id), [roadMap]);
  const districtName = useCallback(
    (id: number) => districtMap.get(id)?.name ?? '—',
    [districtMap]
  );
  const roadPath = useCallback(
    (roadId: number) => {
      const road = roadMap.get(roadId);
      if (!road) {
        return { district: '—', road: '—' };
      }
      return { district: districtMap.get(road.districtId)?.name ?? '—', road: road.name };
    },
    [roadMap, districtMap]
  );

  const value = useMemo<HierarchyValue>(
    () => ({ districts, loading, error, reload, roads, roadById, districtName, roadPath }),
    [districts, loading, error, reload, roads, roadById, districtName, roadPath]
  );

  return <HierarchyContext.Provider value={value}>{children}</HierarchyContext.Provider>;
}

export function useHierarchy(): HierarchyValue {
  const value = useContext(HierarchyContext);
  if (!value) {
    throw new Error('useHierarchy 必须在 HierarchyProvider 内部使用');
  }
  return value;
}
