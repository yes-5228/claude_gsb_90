// 层级多选筛选器：片区多选（支持全选），选中片区后道路下拉只列出这些片区下的道路，
// 道路同样支持多选与全选。供管段台账、任务列表、记录列表、验收列表统一使用。
import { useMemo } from 'react';
import { useHierarchy } from '../providers/HierarchyProvider';

interface HierarchyFilterProps {
  districtIds: number[];
  roadIds: number[];
  onChange: (next: { districtIds: number[]; roadIds: number[] }) => void;
  /** 是否展示"全选"快捷项。 */
  selectAll?: boolean;
}

function toggle(set: number[], id: number): number[] {
  return set.includes(id) ? set.filter((item) => item !== id) : [...set, id];
}

export function HierarchyFilter({ districtIds, roadIds, onChange, selectAll = true }: HierarchyFilterProps) {
  const { tree } = useHierarchy();
  const districts = tree?.districts ?? [];

  const visibleRoads = useMemo(() => {
    const scoped = districtIds.length > 0 ? districts.filter((d) => districtIds.includes(d.id)) : districts;
    return scoped.flatMap((district) => district.roads);
  }, [districts, districtIds]);

  const allDistrictSelected = districts.length > 0 && districtIds.length === districts.length;
  const allRoadSelected = visibleRoads.length > 0 && roadIds.length === visibleRoads.length;

  const toggleDistrict = (id: number) => {
    const wasSelected = districtIds.includes(id);
    const nextDistricts = toggle(districtIds, id);
    // 取消勾选片区时，该片区下已选的道路筛选一并移除，避免残留隐藏条件。
    let nextRoads = roadIds;
    if (wasSelected) {
      const removedRoadIds = visibleRoads
        .filter((road) => road.districtId === id)
        .map((road) => road.id);
      nextRoads = roadIds.filter((roadId) => !removedRoadIds.includes(roadId));
    }
    onChange({ districtIds: nextDistricts, roadIds: nextRoads });
  };

  const toggleAllDistricts = () => {
    if (allDistrictSelected) {
      onChange({ districtIds: [], roadIds: [] });
    } else {
      onChange({ districtIds: districts.map((d) => d.id), roadIds: [] });
    }
  };

  const toggleRoad = (id: number) => {
    onChange({ districtIds, roadIds: toggle(roadIds, id) });
  };

  const toggleAllRoads = () => {
    if (allRoadSelected) {
      onChange({ districtIds, roadIds: [] });
    } else {
      onChange({ districtIds, roadIds: visibleRoads.map((road) => road.id) });
    }
  };

  return (
    <>
      <div className="filter-item">
        <span className="filter-label">所属片区</span>
        <select
          className="select"
          value=""
          onChange={(event) => {
            const value = event.target.value;
            if (value === '__all__') {
              toggleAllDistricts();
            } else {
              const id = Number(value);
              if (id > 0) {
                toggleDistrict(id);
              }
            }
            event.target.value = '';
          }}
        >
          <option value="">
            {districtIds.length > 0 ? `已选 ${districtIds.length} 个片区（继续选择）` : '全部片区'}
          </option>
          {selectAll ? (
            <option value="__all__">{allDistrictSelected ? '取消全选片区' : '全选片区'}</option>
          ) : null}
          {districts.map((district) => (
            <option key={district.id} value={district.id}>
              {districtIds.includes(district.id) ? '✓ ' : ''}
              {district.name}
            </option>
          ))}
        </select>
        {districtIds.length > 0 ? (
          <div className="chip-row">
            {districts
              .filter((district) => districtIds.includes(district.id))
              .map((district) => (
                <button
                  type="button"
                  key={district.id}
                  className="chip chip-active"
                  onClick={() => toggleDistrict(district.id)}
                  title="点击移除该片区筛选"
                >
                  {district.name} ✕
                </button>
              ))}
            {selectAll ? (
              <button type="button" className="chip" onClick={toggleAllDistricts}>
                {allDistrictSelected ? '取消全选' : '全选片区'}
              </button>
            ) : null}
          </div>
        ) : null}
      </div>
      <div className="filter-item">
        <span className="filter-label">所在道路</span>
        <select
          className="select"
          value=""
          onChange={(event) => {
            const value = event.target.value;
            if (value === '__all__') {
              toggleAllRoads();
            } else {
              const id = Number(value);
              if (id > 0) {
                toggleRoad(id);
              }
            }
            event.target.value = '';
          }}
          disabled={visibleRoads.length === 0}
        >
          <option value="">
            {roadIds.length > 0
              ? `已选 ${roadIds.length} 条道路（继续选择）`
              : districtIds.length > 0
                ? '所选片区的全部道路'
                : '全部道路'}
          </option>
          {selectAll && visibleRoads.length > 0 ? (
            <option value="__all__">{allRoadSelected ? '取消全选道路' : '全选道路'}</option>
          ) : null}
          {visibleRoads.map((road) => {
            const district = districts.find((item) => item.id === road.districtId);
            return (
              <option key={road.id} value={road.id}>
                {roadIds.includes(road.id) ? '✓ ' : ''}
                {district ? `${district.name} / ` : ''}
                {road.name}
              </option>
            );
          })}
        </select>
        {roadIds.length > 0 ? (
          <div className="chip-row">
            {visibleRoads
              .filter((road) => roadIds.includes(road.id))
              .map((road) => (
                <button type="button" key={road.id} className="chip chip-active" onClick={() => toggleRoad(road.id)}>
                  {road.name} ✕
                </button>
              ))}
            {selectAll && visibleRoads.length > 0 ? (
              <button type="button" className="chip" onClick={toggleAllRoads}>
                {allRoadSelected ? '取消全选' : '全选道路'}
              </button>
            ) : null}
          </div>
        ) : null}
      </div>
    </>
  );
}
