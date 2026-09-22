// 片区 / 道路层级在应用启动后按需加载，各页面共用同一份层级树。
//
// 层级调整（新增 / 改名 / 归属调整 / 删除）后统一调用 reload，保证管段台账、
// 任务列表、记录列表与看板展示的是同一套层级与名称。
import { createContext, useCallback, useContext, useMemo, type ReactNode } from 'react';
import { hierarchyApi } from '../api/hierarchy';
import { useAsync } from '../hooks/useAsync';
import type { DistrictNode, HierarchyTree, Road } from '../types/domain';

interface HierarchyValue {
  tree: HierarchyTree | null;
  loading: boolean;
  error: string;
  reload: () => void;
  districtById: (id: number) => DistrictNode | undefined;
  roadById: (id: number | null | undefined) => Road | undefined;
  /** 按片区分组的道路索引，供"选中片区后只列该片区道路"联动使用。 */
  roadsOfDistrict: (districtId: number) => Road[];
}

const HierarchyContext = createContext<HierarchyValue | null>(null);

export function HierarchyProvider({ children }: { children: ReactNode }) {
  const { data, loading, error, reload } = useAsync<HierarchyTree>(() => hierarchyApi.tree(), []);

  const districts = data?.districts ?? [];
  const districtMap = useMemo(() => {
    const map = new Map<number, DistrictNode>();
    districts.forEach((district) => map.set(district.id, district));
    return map;
  }, [districts]);

  const roadMap = useMemo(() => {
    const map = new Map<number, Road>();
    districts.forEach((district) => district.roads.forEach((road) => map.set(road.id, road)));
    return map;
  }, [districts]);

  const districtById = useCallback((id: number) => districtMap.get(id), [districtMap]);
  const roadById = useCallback((id: number | null | undefined) =>
    id === null || id === undefined ? undefined : roadMap.get(id), [roadMap]);
  const roadsOfDistrict = useCallback(
    (districtId: number) => districtMap.get(districtId)?.roads ?? [],
    [districtMap]
  );

  const value = useMemo<HierarchyValue>(
    () => ({ tree: data, loading, error, reload, districtById, roadById, roadsOfDistrict }),
    [data, loading, error, reload, districtById, roadById, roadsOfDistrict]
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
