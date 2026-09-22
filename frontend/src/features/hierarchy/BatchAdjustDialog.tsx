// 批量层级调整弹窗：一次提交多个片区改名与道路改名 / 归属调整，
// 后端在单个事务内批量生效，任一校验失败则整体回滚，不会部分成功。
import { useEffect, useState } from 'react';
import { toErrorMessage } from '../../api/client';
import { hierarchyApi } from '../../api/hierarchy';
import { FormField } from '../../components/FormField';
import { Modal } from '../../components/Modal';
import { useToast } from '../../components/Toast';
import type { DistrictNode } from '../../types/domain';

interface BatchAdjustDialogProps {
  open: boolean;
  districts: DistrictNode[];
  onClose: () => void;
  onDone: () => void;
}

interface RoadEdit {
  id: number;
  name: string;
  districtId: number;
  originalName: string;
  originalDistrictId: number;
}

interface DistrictEdit {
  id: number;
  name: string;
  originalName: string;
}

export function BatchAdjustDialog({ open, districts, onClose, onDone }: BatchAdjustDialogProps) {
  const toast = useToast();
  const [districtEdits, setDistrictEdits] = useState<DistrictEdit[]>([]);
  const [roadEdits, setRoadEdits] = useState<RoadEdit[]>([]);
  const [reason, setReason] = useState('');
  const [operator, setOperator] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!open) {
      return;
    }
    setDistrictEdits(districts.map((district) => ({
      id: district.id,
      name: district.name,
      originalName: district.name
    })));
    setRoadEdits(
      districts.flatMap((district) =>
        district.roads.map((road) => ({
          id: road.id,
          name: road.name,
          districtId: road.districtId,
          originalName: road.name,
          originalDistrictId: road.districtId
        }))
      )
    );
    setReason('');
    setOperator('');
  }, [open, districts]);

  const changedDistricts = districtEdits.filter((item) => item.name.trim() && item.name !== item.originalName);
  const changedRoads = roadEdits.filter(
    (item) => (item.name.trim() && item.name !== item.originalName) || item.districtId !== item.originalDistrictId
  );

  const submit = async () => {
    if (changedDistricts.length === 0 && changedRoads.length === 0) {
      toast.error('没有检测到任何改动');
      return;
    }
    setBusy(true);
    try {
      const result = await hierarchyApi.batch({
        reason: reason.trim() || undefined,
        operator: operator.trim() || undefined,
        districts: changedDistricts.map((item) => ({ id: item.id, name: item.name.trim() })),
        roads: changedRoads.map((item) => {
          const road: { id: number; name?: string; districtId?: number } = { id: item.id };
          if (item.name.trim() && item.name !== item.originalName) {
            road.name = item.name.trim();
          }
          if (item.districtId !== item.originalDistrictId) {
            road.districtId = item.districtId;
          }
          return road;
        })
      });
      toast.success(`层级调整已全部生效（片区 ${result.districtChanges} 项，道路 ${result.roadChanges} 项）`);
      onDone();
      onClose();
    } catch (cause: unknown) {
      toast.error(toErrorMessage(cause));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal
      open={open}
      title="批量调整层级"
      onClose={onClose}
      width={720}
      footer={
        <>
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy}>
            取消
          </button>
          <button type="button" className="btn btn-primary" onClick={submit} disabled={busy}>
            {busy
              ? '提交中…'
              : `全部生效（片区 ${changedDistricts.length} / 道路 ${changedRoads.length}）`}
          </button>
        </>
      }
    >
      <p className="form-note">
        所有改动在同一事务内提交，任何一项不合法（重名、目标片区不存在等）都会整体回滚。
        调整道路归属时，引用该道路的管段会一并迁移。
      </p>

      <FormField label="片区改名">
        <div className="hierarchy-tree">
          {districtEdits.map((item, index) => (
            <div key={item.id} className="hierarchy-road">
              <span className="hierarchy-hint">
                {item.name !== item.originalName && item.name.trim() ? '（已改名）' : '片区'}
              </span>
              <input
                className="input"
                value={item.name}
                onChange={(event) => {
                  const next = [...districtEdits];
                  next[index] = { ...next[index], name: event.target.value };
                  setDistrictEdits(next);
                }}
              />
            </div>
          ))}
        </div>
      </FormField>

      <FormField label="道路改名 / 归属调整">
        <div className="hierarchy-tree">
          {roadEdits.length === 0 ? <p className="form-note">暂无道路</p> : null}
          {roadEdits.map((item, index) => {
            const changed =
              (item.name.trim() && item.name !== item.originalName) ||
              item.districtId !== item.originalDistrictId;
            return (
              <div key={item.id} className="hierarchy-road" style={{ paddingLeft: 14 }}>
                <input
                  className="input"
                  style={{ flex: 1 }}
                  value={item.name}
                  onChange={(event) => {
                    const next = [...roadEdits];
                    next[index] = { ...next[index], name: event.target.value };
                    setRoadEdits(next);
                  }}
                />
                <select
                  className="select"
                  style={{ width: 180 }}
                  value={item.districtId}
                  onChange={(event) => {
                    const next = [...roadEdits];
                    next[index] = { ...next[index], districtId: Number(event.target.value) };
                    setRoadEdits(next);
                  }}
                >
                  {districts.map((district) => (
                    <option key={district.id} value={district.id}>
                      {district.name}
                    </option>
                  ))}
                </select>
                {changed ? <span className="chip chip-active">已调整</span> : null}
              </div>
            );
          })}
        </div>
      </FormField>

      <FormField label="调整原因" hint="可选，会记录在变更日志中">
        <input className="input" value={reason} maxLength={255} onChange={(e) => setReason(e.target.value)} />
      </FormField>
      <FormField label="操作人" hint="可选">
        <input className="input" value={operator} maxLength={64} onChange={(e) => setOperator(e.target.value)} />
      </FormField>
    </Modal>
  );
}
