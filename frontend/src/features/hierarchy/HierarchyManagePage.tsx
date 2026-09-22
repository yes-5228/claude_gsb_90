// 片区 / 道路层级管理：新增、改名、归属调整、删除与批量调整，并查看变更记录。
//
// 名称与归属调整全部调用后端层级接口，调整成功后刷新层级树；台账、任务、记录、
// 看板通过同一个 HierarchyProvider 拿到刷新后的层级，历史数据仍按登记当时展示。
import { useEffect, useState } from 'react';
import { toErrorMessage } from '../../api/client';
import { hierarchyApi } from '../../api/hierarchy';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { DataTable, type Column } from '../../components/DataTable';
import { FormField } from '../../components/FormField';
import { Modal } from '../../components/Modal';
import { PageHeader } from '../../components/PageHeader';
import { Pagination } from '../../components/Pagination';
import { SectionCard } from '../../components/SectionCard';
import { StateBlock } from '../../components/StateBlock';
import { useToast } from '../../components/Toast';
import { useAsync } from '../../hooks/useAsync';
import { useHierarchy } from '../../providers/HierarchyProvider';
import type { District, HierarchyAction, HierarchyChangeLog, Road } from '../../types/domain';
import { formatDateTime } from '../../utils/format';
import { BatchAdjustDialog } from './BatchAdjustDialog';

type DialogKind =
  | { kind: 'new-district' }
  | { kind: 'new-road' }
  | { kind: 'rename-district'; district: District }
  | { kind: 'rename-road'; road: Road }
  | { kind: 'move-road'; road: Road }
  | { kind: 'batch' }
  | { kind: 'delete-district'; district: District }
  | { kind: 'delete-road'; road: Road }
  | null;

interface PendingDelete {
  type: 'district' | 'road';
  id: number;
  name: string;
}

const ACTION_LABELS: Record<HierarchyAction, string> = {
  create_district: '新增片区',
  create_road: '新增道路',
  rename_district: '片区改名',
  rename_road: '道路改名',
  move_road: '道路归属调整',
  delete_district: '删除片区',
  delete_road: '删除道路'
};

const LOG_PAGE_SIZE = 10;

export function HierarchyManagePage() {
  const toast = useToast();
  const { tree, loading, error, reload, districtById } = useHierarchy();
  const [dialog, setDialog] = useState<DialogKind>(null);
  const [busy, setBusy] = useState(false);
  const [name, setName] = useState('');
  const [newRoadDistrict, setNewRoadDistrict] = useState('');
  const [targetDistrict, setTargetDistrict] = useState('');
  const [reason, setReason] = useState('');
  const [operator, setOperator] = useState('');
  const [pendingDelete, setPendingDelete] = useState<PendingDelete | null>(null);
  const [logPage, setLogPage] = useState(1);

  const logs = useAsync(
    () =>
      hierarchyApi.logs({ page: logPage, pageSize: LOG_PAGE_SIZE }).then((page) => page),
    [logPage]
  );

  useEffect(() => {
    if (!dialog) {
      return;
    }
    setName(
      dialog.kind === 'rename-district'
        ? dialog.district.name
        : dialog.kind === 'rename-road'
          ? dialog.road.name
          : ''
    );
    if (dialog.kind === 'new-road') {
      setNewRoadDistrict(String(dialog ? firstDistrictId(tree?.districts) : ''));
    }
    if (dialog.kind === 'move-road') {
      setTargetDistrict('');
    }
    setReason('');
  }, [dialog, tree]);

  const districts = tree?.districts ?? [];

  const close = () => setDialog(null);

  const run = async (action: () => Promise<unknown>, success: string) => {
    setBusy(true);
    try {
      await action();
      toast.success(success);
      close();
      reload();
      logs.reload();
    } catch (cause: unknown) {
      toast.error(toErrorMessage(cause));
    } finally {
      setBusy(false);
    }
  };

  const submit = () => {
    if (!dialog) {
      return;
    }
    const meta = { reason: reason.trim() || undefined, operator: operator.trim() || undefined };
    switch (dialog.kind) {
      case 'new-district':
        if (!name.trim()) {
          toast.error('片区名称不能为空');
          return;
        }
        void run(() => hierarchyApi.createDistrict({ name: name.trim(), ...meta }), '片区已新增');
        break;
      case 'new-road': {
        const districtId = Number(newRoadDistrict);
        if (!districtId) {
          toast.error('请先选择所属片区');
          return;
        }
        if (!name.trim()) {
          toast.error('道路名称不能为空');
          return;
        }
        void run(
          () => hierarchyApi.createRoad({ districtId, name: name.trim(), ...meta }),
          '道路已新增'
        );
        break;
      }
      case 'rename-district':
        if (!name.trim()) {
          toast.error('片区名称不能为空');
          return;
        }
        void run(
          () => hierarchyApi.renameDistrict(dialog.district.id, { name: name.trim(), ...meta }),
          '片区已改名'
        );
        break;
      case 'rename-road':
        if (!name.trim()) {
          toast.error('道路名称不能为空');
          return;
        }
        void run(
          () => hierarchyApi.renameRoad(dialog.road.id, { name: name.trim(), ...meta }),
          '道路已改名'
        );
        break;
      case 'move-road': {
        const districtId = Number(targetDistrict);
        if (!districtId) {
          toast.error('请选择目标片区');
          return;
        }
        if (districtId === dialog.road.districtId) {
          toast.error('道路已属于该片区，无需调整');
          return;
        }
        void run(
          () => hierarchyApi.moveRoad(dialog.road.id, { districtId, ...meta }),
          '道路归属已调整，引用该道路的管段已一并迁移'
        );
        break;
      }
    }
  };

  const confirmDelete = async () => {
    if (!pendingDelete) {
      return;
    }
    setBusy(true);
    try {
      if (pendingDelete.type === 'district') {
        await hierarchyApi.deleteDistrict(pendingDelete.id);
      } else {
        await hierarchyApi.deleteRoad(pendingDelete.id);
      }
      toast.success(`${pendingDelete.name} 已删除`);
      setPendingDelete(null);
      reload();
      logs.reload();
    } catch (cause: unknown) {
      toast.error(toErrorMessage(cause));
      setPendingDelete(null);
    } finally {
      setBusy(false);
    }
  };

  const logColumns: Column<HierarchyChangeLog>[] = [
    {
      key: 'createdAt',
      title: '时间',
      width: '160px',
      render: (row) => formatDateTime(row.createdAt)
    },
    {
      key: 'nodeType',
      title: '层级',
      width: '80px',
      render: (row) => (row.nodeType === 'district' ? '片区' : '道路')
    },
    {
      key: 'action',
      title: '动作',
      width: '120px',
      render: (row) => ACTION_LABELS[row.action] ?? row.action
    },
    {
      key: 'name',
      title: '名称 / 批次',
      render: (row) => (
        <>
          <span className="cell-main">{row.name}</span>
          <span className="cell-sub">{row.batchId}</span>
        </>
      )
    },
    {
      key: 'reason',
      title: '原因 / 操作人',
      width: '200px',
      render: (row) => (
        <>
          <span>{row.reason || '—'}</span>
          <span className="cell-sub">{row.operator || '未填写'}</span>
        </>
      )
    }
  ];

  const dialogTitle: Record<string, string> = {
    'new-district': '新增片区',
    'new-road': '新增道路',
    'rename-district': '片区改名',
    'rename-road': '道路改名',
    'move-road': '调整道路归属',
    batch: '批量调整层级'
  };

  return (
    <div className="page">
      <PageHeader
        title="片区 / 道路层级"
        description="管段台账、任务、记录与看板共用同一套片区与道路；新增、改名和归属调整都会留下记录，被引用的层级不允许删除。"
        actions={
          <>
            <button type="button" className="btn btn-ghost" onClick={() => setDialog({ kind: 'batch' })}>
              批量调整
            </button>
            <button type="button" className="btn btn-ghost" onClick={() => setDialog({ kind: 'new-road' })}>
              新增道路
            </button>
            <button type="button" className="btn btn-primary" onClick={() => setDialog({ kind: 'new-district' })}>
              新增片区
            </button>
          </>
        }
      />

      <StateBlock loading={loading} error={error} onRetry={reload}>
        <SectionCard title="层级树" subtitle="调整道路归属时，引用该道路的管段会一并迁移到新片区">
          <div className="card-body">
            {districts.length === 0 ? (
              <p className="form-note">还没有片区，请先新增片区。</p>
            ) : (
              <div className="hierarchy-tree">
                {districts.map((district) => (
                  <div key={district.id} className="hierarchy-district">
                    <div className="hierarchy-district-head">
                      <span>{district.name}</span>
                      <span className="hierarchy-actions">
                        <button
                          type="button"
                          className="btn btn-ghost btn-sm"
                          onClick={() => setDialog({ kind: 'new-road' })}
                        >
                          新增道路
                        </button>
                        <button
                          type="button"
                          className="btn btn-ghost btn-sm"
                          onClick={() => setDialog({ kind: 'rename-district', district })}
                        >
                          改名
                        </button>
                        <button
                          type="button"
                          className="btn btn-danger btn-sm"
                          onClick={() =>
                            setPendingDelete({ type: 'district', id: district.id, name: district.name })
                          }
                        >
                          删除
                        </button>
                      </span>
                    </div>
                    <div className="hierarchy-road-list">
                      {district.roads.length === 0 ? (
                        <p className="form-note" style={{ padding: '8px 32px' }}>
                          该片区暂无道路
                        </p>
                      ) : (
                        district.roads.map((road) => (
                          <div key={road.id} className="hierarchy-road">
                            <span>{road.name}</span>
                            <span className="hierarchy-actions">
                              <button
                                type="button"
                                className="btn btn-ghost btn-sm"
                                onClick={() => setDialog({ kind: 'move-road', road })}
                              >
                                调整归属
                              </button>
                              <button
                                type="button"
                                className="btn btn-ghost btn-sm"
                                onClick={() => setDialog({ kind: 'rename-road', road })}
                              >
                                改名
                              </button>
                              <button
                                type="button"
                                className="btn btn-danger btn-sm"
                                onClick={() =>
                                  setPendingDelete({ type: 'road', id: road.id, name: road.name })
                                }
                              >
                                删除
                              </button>
                            </span>
                          </div>
                        ))
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
            <p className="form-note" style={{ marginTop: 12 }}>
              提示：片区调整（改名 / 道路归属调整）支持批量生效；被管段、任务、清淤记录或验收记录引用的层级
              不允许直接删除。任务、清淤记录、验收记录会保留登记当时的层级名称，层级调整后历史数据口径不变。
            </p>
          </div>
        </SectionCard>
      </StateBlock>

      <SectionCard title="层级变更记录" subtitle="新增、改名、归属调整与删除都会逐条留痕，同一次批量调整共用一个批次号">
        <div className="card-body-flush">
          <DataTable
            columns={logColumns}
            rows={logs.data?.list ?? []}
            rowKey={(row) => row.id}
            loading={logs.loading}
            error={logs.error}
            onRetry={logs.reload}
            emptyText="暂无层级变更记录"
          />
          <Pagination
            total={logs.data?.total ?? 0}
            page={logPage}
            pageSize={logs.data?.pageSize ?? LOG_PAGE_SIZE}
            onChange={setLogPage}
          />
        </div>
      </SectionCard>

      <Modal
        open={
          dialog !== null &&
          !['delete-district', 'delete-road', 'batch'].includes(dialog.kind)
        }
        title={dialog ? dialogTitle[dialog.kind] ?? '' : ''}
        onClose={close}
        footer={
          <>
            <button type="button" className="btn btn-ghost" onClick={close} disabled={busy}>
              取消
            </button>
            <button type="button" className="btn btn-primary" onClick={submit} disabled={busy}>
              {busy ? '提交中…' : '确定'}
            </button>
          </>
        }
      >
        {dialog?.kind === 'new-road' ? (
          <FormField label="所属片区" required>
            <select
              className="select"
              value={newRoadDistrict}
              onChange={(event) => setNewRoadDistrict(event.target.value)}
            >
              <option value="">请选择片区</option>
              {districts.map((district) => (
                <option key={district.id} value={district.id}>
                  {district.name}
                </option>
              ))}
            </select>
          </FormField>
        ) : null}
        {dialog?.kind === 'move-road' ? (
          <>
            <FormField label="道路">
              <input className="input" value={dialog.road.name} disabled />
            </FormField>
            <FormField label="当前片区">
              <input
                className="input"
                value={districtById(dialog.road.districtId)?.name ?? ''}
                disabled
              />
            </FormField>
            <FormField label="目标片区" required>
              <select
                className="select"
                value={targetDistrict}
                onChange={(event) => setTargetDistrict(event.target.value)}
              >
                <option value="">请选择目标片区</option>
                {districts
                  .filter((district) => district.id !== dialog.road.districtId)
                  .map((district) => (
                    <option key={district.id} value={district.id}>
                      {district.name}
                    </option>
                  ))}
              </select>
            </FormField>
          </>
        ) : null}
        {dialog && dialog.kind !== 'move-road' && dialog.kind !== 'new-road' ? null : null}
        {dialog && ['new-district', 'new-road', 'rename-district', 'rename-road'].includes(dialog.kind) ? (
          <FormField label="名称" required>
            <input
              className="input"
              value={name}
              placeholder={dialog.kind.includes('district') ? '例如 城东片区' : '例如 中山北路'}
              onChange={(event) => setName(event.target.value)}
              autoFocus
            />
          </FormField>
        ) : null}
        <FormField label="调整原因" hint="可选，会记录在变更日志中">
          <input
            className="input"
            value={reason}
            maxLength={255}
            onChange={(event) => setReason(event.target.value)}
          />
        </FormField>
        <FormField label="操作人" hint="可选">
          <input
            className="input"
            value={operator}
            maxLength={64}
            onChange={(event) => setOperator(event.target.value)}
          />
        </FormField>
        {dialog?.kind === 'move-road' ? (
          <p className="form-note">归属调整会在一个事务内完成，引用该道路的管段会同步迁移，不会出现部分成功。</p>
        ) : null}
      </Modal>

      <BatchAdjustDialog
        open={dialog?.kind === 'batch'}
        districts={districts}
        onClose={close}
        onDone={() => {
          reload();
          logs.reload();
        }}
      />

      <ConfirmDialog
        open={pendingDelete !== null}
        title={pendingDelete?.type === 'district' ? '删除片区' : '删除道路'}
        danger
        busy={busy}
        confirmText="确认删除"
        message={
          <>
            <p>
              即将删除{pendingDelete?.type === 'district' ? '片区' : '道路'} <strong>{pendingDelete?.name}</strong>。
            </p>
            <p>被管段、清淤任务、清淤记录或验收记录引用的层级不允许直接删除；片区下仍有道路时也需先移走或删除道路。</p>
          </>
        }
        onConfirm={confirmDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </div>
  );
}

function firstDistrictId(districts: District[] | undefined): number {
  return districts && districts.length > 0 ? districts[0].id : 0;
}
