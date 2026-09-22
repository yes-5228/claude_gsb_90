// 片区-道路多选级联：勾选片区联动其下道路，支持每个片区与整体的全选 / 半选。
//
// 选中片区后只列出该片区下的道路；勾选状态以道路集合为准，片区勾选框做三态联动。
import { useMemo, useState, useRef, useEffect } from 'react';
import type { DistrictNode } from '../types/domain';

interface Props {
  districts: DistrictNode[];
  selectedDistrictIds: number[];
  selectedRoadIds: number[];
  onChange: (next: { districtIds: number[]; roadIds: number[] }) => void;
  placeholder?: string;
  allowClear?: boolean;
}

function toggle(list: number[], id: number, on: boolean): number[] {
  return on ? Array.from(new Set([...list, id])) : list.filter((item) => item !== id);
}

export function HierarchyMultiSelect({
  districts,
  selectedDistrictIds,
  selectedRoadIds,
  onChange,
  placeholder = '选择片区 / 道路',
  allowClear = true
}: Props) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  // 只展示被选中片区下的道路；未选片区时展示全部片区（收起）。
  const visibleDistricts = useMemo(() => {
    if (selectedDistrictIds.length === 0) {
      return districts;
    }
    return districts.filter((d) => selectedDistrictIds.includes(d.id));
  }, [districts, selectedDistrictIds]);

  const selectedRoadSet = useMemo(() => new Set(selectedRoadIds), [selectedRoadIds]);

  const label = useMemo(() => {
    if (selectedRoadIds.length > 0) {
      return `已选 ${selectedRoadIds.length} 条道路`;
    }
    if (selectedDistrictIds.length > 0) {
      const names = districts.filter((d) => selectedDistrictIds.includes(d.id)).map((d) => d.name);
      return names.length <= 2 ? names.join('、') : `已选 ${names.length} 个片区`;
    }
    return placeholder;
  }, [selectedRoadIds, selectedDistrictIds, districts, placeholder]);

  // 勾选片区：联动勾选 / 取消其下全部道路。
  const toggleDistrict = (district: DistrictNode, on: boolean) => {
    const ids = district.roads.map((r) => r.id);
    const roadIds = on
      ? Array.from(new Set([...selectedRoadIds, ...ids]))
      : selectedRoadIds.filter((id) => !ids.includes(id));
    const districtIds = toggle(selectedDistrictIds, district.id, on);
    onChange({ districtIds, roadIds });
  };

  const toggleRoad = (district: DistrictNode, roadId: number, on: boolean) => {
    const roadIds = toggle(selectedRoadIds, roadId, on);
    const allOn = district.roads.length > 0 && district.roads.every((r) => roadIds.includes(r.id));
    const districtIds = toggle(selectedDistrictIds, district.id, allOn);
    onChange({ districtIds, roadIds });
  };

  const allRoadIds = useMemo(() => districts.flatMap((d) => d.roads.map((r) => r.id)), [districts]);
  const allSelected = allRoadIds.length > 0 && allRoadIds.every((id) => selectedRoadIds.includes(id));

  const toggleAll = (on: boolean) => {
    onChange({
      districtIds: on ? districts.map((d) => d.id) : [],
      roadIds: on ? allRoadIds : []
    });
  };

  const clear = () => onChange({ districtIds: [], roadIds: [] });

  const districtState = (district: DistrictNode): 'all' | 'some' | 'none' => {
    if (district.roads.length === 0) {
      return 'none';
    }
    const onCount = district.roads.filter((r) => selectedRoadSet.has(r.id)).length;
    if (onCount === 0) {
      return 'none';
    }
    return onCount === district.roads.length ? 'all' : 'some';
  };

  return (
    <div className="hier-select" ref={ref}>
      <button type="button" className={`select hier-select-trigger${open ? ' open' : ''}`} onClick={() => setOpen((v) => !v)}>
        <span className={selectedRoadIds.length || selectedDistrictIds.length ? '' : 'hier-placeholder'}>{label}</span>
        <span className="hier-caret">▾</span>
      </button>
      {open ? (
        <div className="hier-pop">
          <div className="hier-pop-toolbar">
            <label className="hier-check">
              <input type="checkbox" checked={allSelected} ref={(el) => { if (el) el.indeterminate = !allSelected && selectedRoadIds.length > 0; }} onChange={(e) => toggleAll(e.target.checked)} />
              <span>全选</span>
            </label>
            {allowClear && (selectedRoadIds.length > 0 || selectedDistrictIds.length > 0) ? (
              <button type="button" className="btn-link" onClick={clear}>
                清除
              </button>
            ) : null}
          </div>
          <div className="hier-pop-body">
            {visibleDistricts.length === 0 ? (
              <p className="form-note">暂无层级数据，请先在层级管理中维护片区与道路。</p>
            ) : (
              visibleDistricts.map((district) => {
                const state = districtState(district);
                return (
                  <div key={district.id} className="hier-group">
                    <label className="hier-check hier-district">
                      <input
                        type="checkbox"
                        checked={state === 'all'}
                        ref={(el) => { if (el) el.indeterminate = state === 'some'; }}
                        onChange={(e) => toggleDistrict(district, e.target.checked)}
                      />
                      <span className="hier-district-name">{district.name}</span>
                      <span className="hier-count">{district.roads.length} 路</span>
                    </label>
                    <div className="hier-roads">
                      {district.roads.map((road) => (
                        <label key={road.id} className="hier-check hier-road">
                          <input
                            type="checkbox"
                            checked={selectedRoadSet.has(road.id)}
                            onChange={(e) => toggleRoad(district, road.id, e.target.checked)}
                          />
                          <span>{road.name}</span>
                          {road.segmentCount > 0 ? <span className="hier-count">{road.segmentCount}</span> : null}
                        </label>
                      ))}
                      {district.roads.length === 0 ? <span className="form-note">该片区暂无道路</span> : null}
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>
      ) : null}
    </div>
  )
}
