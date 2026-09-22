// 片区-道路层级管理：维护片区与道路两级层级，查看变更记录。
//
// 支持片区 / 道路的新增、改名，道路归属调整（单条与批量），
// 以及删除保护提示；所有操作都会在后端留下变更记录。
import { useMemo, useState } from 'react';
import { toErrorMessage } from '../../api/client';
import { hierarchyApi } from '../../api/hierarchy';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { DataTable, type Column } from '../../components/DataTable';
import { Modal } from '../../components/Modal';
import { PageHeader } from '../../components/PageHeader';
import { Pagination } from '../../components/Pagination';
import { SectionCard } from '../../components/SectionCard';
import { useToast } from '../../components/Toast';
import { useAsync } from '../../hooks/useAsync';
import { useHierarchy } from '../../providers/HierarchyProvider';
import type { DistrictNode, HierarchyAction, HierarchyChangeLog, RoadNode } from '../../types/domain';

const PAGE_SIZE = 10;

interface TextModalState {
  title: string;
  label: string;
  initial: string;
  submit: (value: string) => Promise<void>;
}

interface MoveState {
  road: RoadNode;
}

const ACTION_LABEL: Record<HierarchyAction, string> = {
  create: '新增',
  rename: '改名',
  move: '归属调整',
  delete: '删除'
};

export function HierarchyPage() {
  const toast = useToast();
  const { districts, reload, roads } = useHierarchy();
  const [busy, setBusy] = useState(false);

  const [districtModal, setDistrictModal] = useState<TextModalState | null>(null);
  const [roadModal, setRoadModal] = useState<{ district: DistrictNode } | null>(null);
  const [roadName, setRoadName] = useState('');
  const [moveState, setMoveState] = useState<MoveState | null>(null);
  const [moveTarget, setMoveTarget] = useState('');

  // 批量归属调整。
  const [batchMode, setBatchMode] = useState(false);
  const [checked, setChecked] = useState<Set<number>>(new Set());
  const [batchTarget, setBatchTarget] = useState('');

  const [pendingDeleteDistrict, setPendingDeleteDistrict] = useState<DistrictNode | null>(null);
  const [pendingDeleteRoad, setPendingDeleteRoad] = useState<RoadNode | null>(null);

  const [logPage, setLogPage] = useState(1);
  const logs = useAsync(
    () => hierarchyApi.changeLogs({ page: logPage, pageSize: PAGE_SIZE }),
    [logPage]
  );

  const run = async (fn: () => Promise<void>, success: string) => {
    setBusy(true);
    try {
      await fn();
      toast.success(success);
      reload();
      logs.reload();
    } catch (cause: unknown) {
      toast.error(toErrorMessage(cause));
    } finally {
      setBusy(false);
    }
  };

  const openRenameDistrict = (d: DistrictNode) =>
    setDistrictModal({
      title: `片区改名：${d.name}`,
      label: '片区名称',
      initial: d.name,
      submit: async (value) => {
        await hierarchyApi.renameDistrict(d.id, value);
      }
    });

  const openCreateRoad = (d: DistrictNode) => {
    setRoadModal({ district: d });
    setRoadName('');
  };

  const toggleCheck = (id: number) => {
    setChecked((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const checkedRoads = useMemo(() => roads.filter((r) => checked.has(r.id)), [roads, checked]);

  const submitBatchMove = () => {
    const targetID = Number(batchTarget);
    if (!targetID) {
      toast.error('请选择目标片区');
      return;
    }
    if (checked.size === 0) {
      toast.error('请先勾选需要调整归属的道路');
      return;
    }
    const movable = checkedRoads.filter((r) => r.districtId !== targetID);
    if (movable.length === 0) {
      toast.error('所选道路都已在目标片区下，无需调整');
      return;
    }
    void run(async () => {
      await hierarchyApi.batchMoveRoads({
        items: movable.map((r) => ({ roadId: r.id, districtId: targetID }))
      });
      setBatchMode(false);
      setChecked(new Set());
      setBatchTarget('');
    }, `已批量调整 ${movable.length} 条道路的归属（整组生效，未部分成功）`);
  };

  const submitMove = () => {
    if (!moveState) {
      return;
    }
    const targetID = Number(moveTarget);
    if (!targetID) {
      toast.error('请选择目标片区');
      return;
    }
    if (targetID === moveState.road.districtId) {
      toast.error('道路已在该片区下');
      return;
    }
    void run(async () => {
      await hierarchyApi.moveRoad(moveState.road.id, targetID);
      setMoveState(null);
    }, '道路归属已调整');
  };

  const logColumns: Column<HierarchyChangeLog>[] = [
    { key: 'createdAt', title: '时间', width: '170px', render: (row) => new Date(row.createdAt).toLocaleString('zh-CN') },
    {
      key: 'nodeType',
      title: '层级',
      width: '80px',
      render: (row) => (row.nodeType === 'district' ? '片区' : '道路')
    },
    { key: 'nodeName', title: '名称', width: '140px', render: (row) => row.nodeName },
    {
      key: 'action',
      title: '动作',
      width: '90px',
      render: (row) => <span className="tag tag-info">{ACTION_LABEL[row.action] ?? row.action}</span>
    },
    {
      key: 'change',
      title: '变更内容',
      render: (row) => (
        <span>
          {row.fromValue || '—'} <span className="hier-arrow">→</span> {row.toValue || '—'}
          {row.remark ? <span className="cell-sub">{row.remark}</span> : null}
        </span>
      )
    },
    { key: 'operator', title: '操作人', width: '100px', render: (row) => row.operator || '—' },
    { key: 'batchId', title: '调整批次', width: '190px', render: (row) => <span className="cell-sub">{row.batchId}</span> }
  ];

  return (
    <div className="page">
      <PageHeader
        title="片区 / 道路层级"
        description="统一维护片区与道路两级层级，管段台账、任务、记录与看板共用同一套层级与名称；调整记录全程留痕，历史数据按登记当时的层级展示。"
        actions={
          <>
            <button
              type="button"
              className={`btn ${batchMode ? 'btn-primary' : 'btn-ghost'}`}
              onClick={() => {
                setBatchMode((v) => !v);
                setChecked(new Set());
              }}
            >
              {batchMode ? '退出批量调整' : '批量调整归属'}
            </button>
            <button
              type="button"
              className="btn btn-primary"
              onClick={() =>
                setDistrictModal({
                  title: '新增片区',
                  label: '片区名称',
                  initial: '',
                  submit: async (value) => {
                    await hierarchyApi.createDistrict(value);
                  }
                })
              }
            >
              新增片区
            </button>
          </>
        }
      />

      {batchMode ? (
        <SectionCard title="批量归属调整" subtitle="勾选道路后选择目标片区，整组在一个事务内生效，任一不合法则全部回滚">
          <div className="batch-move-bar">
            <span>已选 {checked.size} 条道路</span>
            <select className="select" value={batchTarget} onChange={(e) => setBatchTarget(e.target.value)}>
              <option value="">选择目标片区…</option>
              {districts.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </select>
            <button type="button" className="btn btn-primary btn-sm" disabled={busy} onClick={submitBatchMove}>
              应用到所选道路
            </button>
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => setChecked(new Set())}>
              清空选择
            </button>
          </div>
        </SectionCard>
      ) : null}

      <div className="hier-tree">
        {districts.map((district) => (
          <SectionCard
            key={district.id}
            title={district.name}
            subtitle={`${district.roadCount} 条道路 · ${district.segmentCount} 个管段`}
            extra={
              <div className="row-actions">
                <button type="button" className="btn-link" onClick={() => openCreateRoad(district)}>
                  新增道路
                </button>
                <button type="button" className="btn-link" onClick={() => openRenameDistrict(district)}>
                  改名
                </button>
                <button type="button" className="btn-link btn-danger-link" onClick={() => setPendingDeleteDistrict(district)}>
                  删除
                </button>
              </div>
            }
          >
            <div className="card-body-flush">
              <table className="table hier-road-table">
                <thead>
                  <tr>
                    {batchMode ? <th style={{ width: 40 }} /> : null}
                    <th>道路</th>
                    <th style={{ width: 120 }}>管段数</th>
                    <th style={{ width: 220 }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {district.roads.map((road) => (
                    <tr key={road.id}>
                      {batchMode ? (
                        <td>
                          <input type="checkbox" checked={checked.has(road.id)} onChange={() => toggleCheck(road.id)} />
                        </td>
                      ) : null}
                      <td>{road.name}</td>
                      <td>{road.segmentCount}</td>
                      <td>
                        <div className="row-actions">
                          <button
                            type="button"
                            className="btn-link"
                            onClick={() =>
                              setDistrictModal({
                                title: `道路改名：${road.name}`,
                                label: '道路名称',
                                initial: road.name,
                                submit: async (value) => {
                                  await hierarchyApi.renameRoad(road.id, value);
                                }
                              })
                            }
                          >
                            改名
                          </button>
                          <button
                            type="button"
                            className="btn-link"
                            onClick={() => {
                              setMoveState({ road });
                              setMoveTarget(String(road.districtId));
                            }}
                          >
                            归属调整
                          </button>
                          <button type="button" className="btn-link btn-danger-link" onClick={() => setPendingDeleteRoad(road)}>
                            删除
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                  {district.roads.length === 0 ? (
                    <tr>
                      <td colSpan={batchMode ? 4 : 3}>
                        <p className="form-note">该片区暂无道路，可点击右上角「新增道路」。</p>
                      </td>
                    </tr>
                  ) : null}
                </tbody>
              </table>
            </div>
          </SectionCard>
        ))}
      </div>

      <SectionCard title="层级变更记录" subtitle="新增、改名、归属调整与删除均在此留痕，批量调整共享同一批次号">
        <div className="card-body-flush">
          <DataTable
            columns={logColumns}
            rows={logs.data?.list ?? []}
            rowKey={(row) => row.id}
            loading={logs.loading}
            error={logs.error}
            onRetry={logs.reload}
            emptyText="暂无变更记录"
          />
          <Pagination
            total={logs.data?.total ?? 0}
            page={logPage}
            pageSize={logs.data?.pageSize ?? PAGE_SIZE}
            onChange={setLogPage}
          />
        </div>
      </SectionCard>

      {/* 通用文本输入弹窗：片区新增/改名、道路改名 */}
      <Modal
        open={districtModal !== null}
        title={districtModal?.title ?? ''}
        onClose={() => setDistrictModal(null)}
        footer={
          <>
            <button type="button" className="btn btn-ghost" onClick={() => setDistrictModal(null)}>
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary"
              disabled={busy}
              onClick={() => {
                if (!districtModal) {
                  return;
                }
                const value = (document.getElementById('hier-text-input') as HTMLInputElement | null)?.value.trim() ?? '';
                if (!value) {
                  toast.error(`${districtModal.label}不能为空`);
                  return;
                }
                void run(async () => {
                  await districtModal.submit(value);
                  setDistrictModal(null);
                }, '操作成功');
              }}
            >
              保存
            </button>
          </>
        }
      >
        <div className="form-field">
          <label className="form-label">{districtModal?.label}</label>
          <input id="hier-text-input" className="input" defaultValue={districtModal?.initial} autoFocus />
        </div>
      </Modal>

      {/* 新增道路弹窗 */}
      <Modal
        open={roadModal !== null}
        title={`在「${roadModal?.district.name}」下新增道路`}
        onClose={() => setRoadModal(null)}
        footer={
          <>
            <button type="button" className="btn btn-ghost" onClick={() => setRoadModal(null)}>
              取消
            </button>
            <button
              type="button"
              className="btn btn-primary"
              disabled={busy}
              onClick={() => {
                if (!roadModal) {
                  return;
                }
                const value = roadName.trim();
                if (!value) {
                  toast.error('道路名称不能为空');
                  return;
                }
                void run(async () => {
                  await hierarchyApi.createRoad(roadModal.district.id, value);
                  setRoadModal(null);
                }, '道路已新增');
              }}
            >
              保存
            </button>
          </>
        }
      >
        <div className="form-field">
          <label className="form-label">道路名称</label>
          <input
            className="input"
            value={roadName}
            autoFocus
            placeholder="例如 中山北路"
            onChange={(e) => setRoadName(e.target.value)}
          />
        </div>
      </Modal>

      {/* 单条道路归属调整 */}
      <Modal
        open={moveState !== null}
        title={`归属调整：${moveState?.road.name ?? ''}`}
        onClose={() => setMoveState(null)}
        footer={
          <>
            <button type="button" className="btn btn-ghost" onClick={() => setMoveState(null)}>
              取消
            </button>
            <button type="button" className="btn btn-primary" disabled={busy} onClick={submitMove}>
              确认调整
            </button>
          </>
        }
      >
        <div className="form-field">
          <label className="form-label">目标片区</label>
          <select className="select" value={moveTarget} onChange={(e) => setMoveTarget(e.target.value)}>
            {districts.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
                {d.id === moveState?.road.districtId ? '（当前片区）' : ''}
              </option>
            ))}
          </select>
          <p className="form-note">调整在事务内原子生效；历史任务与记录仍按登记当时的层级展示。</p>
        </div>
      </Modal>

      <ConfirmDialog
        open={pendingDeleteDistrict !== null}
        title="删除片区"
        danger
        busy={busy}
        confirmText="确认删除"
        message={
          <p>
            即将删除片区 <strong>{pendingDeleteDistrict?.name}</strong>。其下仍有道路的片区不允许删除，请先调整或删除道路。
          </p>
        }
        onConfirm={() =>
          pendingDeleteDistrict &&
          run(async () => {
            await hierarchyApi.deleteDistrict(pendingDeleteDistrict.id);
            setPendingDeleteDistrict(null);
          }, '片区已删除')
        }
        onCancel={() => setPendingDeleteDistrict(null)}
      />
      <ConfirmDialog
        open={pendingDeleteRoad !== null}
        title="删除道路"
        danger
        busy={busy}
        confirmText="确认删除"
        message={
          <p>
            即将删除道路 <strong>{pendingDeleteRoad?.name}</strong>。被管段台账引用的道路不允许删除，请先调整管段归属。
          </p>
        }
        onConfirm={() =>
          pendingDeleteRoad &&
          run(async () => {
            await hierarchyApi.deleteRoad(pendingDeleteRoad.id);
            setPendingDeleteRoad(null);
          }, '道路已删除')
        }
        onCancel={() => setPendingDeleteRoad(null)}
      />
    </div>
  );
}
