# 排水管网清淤记录系统

面向市政排水管网养护单位的一套清淤全过程记录系统：以**管段台账**为基础档案，
通过**清淤任务**组织作业，用**清淤记录**记录每次现场作业数据，最后由**验收记录**给出结论并回写台账。

前端 React 单页应用按业务模块拆分页面，后端 Fiber 按模块拆分服务，前后端通过统一信封结构交互。

## 功能模块

| 模块 | 页面 | 说明 |
| --- | --- | --- |
| 运行看板 | `/` | 跨模块汇总管段、任务、清淤量、验收合格率，含待验收任务与分片区统计 |
| 片区 / 道路层级 | `/hierarchy` | 两级层级的新增、改名、归属调整、批量调整、删除与变更记录 |
| 管段台账 | `/segments`、`/segments/new`、`/segments/:id`、`/segments/:id/edit` | 管段档案维护、任务概况、清淤履历（计划 → 记录 → 验收），片区 / 道路从统一层级选择 |
| 清淤任务 | `/tasks`、`/tasks/new`、`/tasks/:id`、`/tasks/:id/edit` | 任务登记、开工 / 完工报验 / 取消、任务下清淤记录与验收结论 |
| 清淤记录 | `/records`、`/records/new`、`/records/:id`、`/records/:id/edit` | 按次录入现场数据（长度、清淤量、用水量、人数、设备、安全措施等） |
| 验收记录 | `/acceptances`、`/acceptances/new`、`/acceptances/:id` | 登记验收结论、整改跟踪、复验 |
| 枚举字典 | `/api/v1/meta/enums` | 后端统一下发全部枚举中文标签，前端不硬编码文案 |
| 层级树 | `/api/v1/hierarchy/tree` | 片区 / 道路两级层级，台账、任务、记录、看板共用同一套层级 |
| 层级变更记录 | `/api/v1/hierarchy/change-logs` | 新增、改名、归属调整、删除逐条留痕，同一次批量调整共用一个批次号 |

## 技术栈

- 后端：Go 1.27 + Fiber v2 + GORM + PostgreSQL 16（本地开发与单测可用 SQLite）
- 前端：React 19 + React Router 7 + TypeScript + Vite，样式为原生 CSS（无 UI 组件库）
- 部署：Docker Compose（PostgreSQL + 后端 + nginx 托管前端并反向代理 `/api`）

## 目录结构

```
.
├── backend/                         # Go 后端
│   ├── cmd/server/                  # 服务入口（配置、日志、优雅关闭）
│   ├── internal/
│   │   ├── config/                  # 环境变量配置
│   │   ├── database/                # 连接、迁移、演示数据
│   │   ├── httpx/                   # 统一信封、错误码、字段校验、分页
│   │   ├── middleware/              # 请求 ID、访问日志、Recover、CORS
│   │   ├── modules/                 # 业务模块（每个模块 model/dto/repository/service/handler/routes）
│   │   │   ├── hierarchy/           # 片区 / 道路层级与变更日志
│   │   │   ├── pipesegment/         # 管段台账
│   │   │   ├── cleaningtask/        # 清淤任务（含状态机）
│   │   │   ├── cleaningrecord/      # 清淤记录
│   │   │   ├── acceptance/          # 验收记录
│   │   │   ├── dashboard/           # 运行看板（只读汇总）
│   │   │   └── meta/                # 枚举字典
│   │   ├── router/                  # 路由装配
│   │   ├── shared/                  # 公共能力：date / option / num / refx
│   │   └── testsupport/             # 测试辅助（内存 SQLite + 断言）
│   └── Dockerfile
├── frontend/                        # React 前端
│   ├── src/
│   │   ├── api/                     # 接口封装（统一拆信封、抛 ApiError）
│   │   ├── components/              # 通用组件（布局、表格、分页、弹窗、层级多选筛选、Toast 等）
│   │   ├── features/                # 按业务模块拆分的页面
│   │   │   ├── hierarchy/  dashboard/  segments/  tasks/  records/  acceptances/
│   │   ├── hooks/                   # useAsync / useForm
│   │   ├── providers/               # 枚举字典 Provider + 层级树 Provider（共用同一套层级）
│   │   ├── styles/                  # 全局样式
│   │   ├── types/                   # 与后端一一对应的领域类型
│   │   └── utils/                   # 日期、数字、枚举标签格式化
│   ├── nginx.conf                   # SPA 回退 + /api 反向代理 + JSON 502
│   └── Dockerfile
├── docker-compose.yml
└── .env.example
```

## 快速开始

### 方式一：Docker Compose（推荐）

```bash
cp .env.example .env      # 可选，用于覆盖端口与数据库账号
docker compose up -d --build
```

启动完成后：

- 前端页面：http://localhost:8081
- 后端健康检查：http://localhost:8080/healthz
- 接口根路径：http://localhost:8080/api/v1

首次启动会自动建表并写入演示数据（管段、任务、清淤记录、验收记录覆盖全部状态）。
数据表非空时不会重复写入；把 `APP_SEED_ENABLED` 设为 `false` 可关闭。

停止与清理：

```bash
docker compose down          # 停止容器，保留数据卷
docker compose down -v       # 连同 PostgreSQL 数据卷一起删除
```

### 方式二：本地开发

后端（默认使用 PostgreSQL，切到 SQLite 可以零依赖启动）：

```bash
cd backend
DB_DRIVER=sqlite DB_PATH=data/dev.db APP_PORT=8080 go run ./cmd/server
```

前端（Vite 开发服务器会把 `/api` 代理到 `http://localhost:8080`）：

```bash
cd frontend
npm install
npm run dev            # http://localhost:5173
```

## 环境变量

后端全部配置项都有默认值，不加任何变量也能启动。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_ENV` | `development` | 运行环境标识，会出现在 `/healthz` |
| `APP_PORT` | `8080` | 后端监听端口 |
| `APP_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `APP_SEED_ENABLED` | `true` | 是否写入演示数据（仅当数据表为空） |
| `DB_DRIVER` | `postgres` | `postgres` 或 `sqlite` |
| `DB_PATH` | `data/desilting.db` | SQLite 数据文件路径 |
| `DB_HOST` / `DB_PORT` | `localhost` / `5432` | PostgreSQL 地址 |
| `DB_NAME` / `DB_USER` / `DB_PASSWORD` | `desilting` / `desilting` / `desilting123456` | PostgreSQL 库与账号 |
| `DB_SSLMODE` | `disable` | PostgreSQL sslmode |
| `DB_TIMEZONE` | `Asia/Shanghai` | 连接时区，同时用于进程时区 |

## 接口一览

所有接口返回统一信封，`code = 0` 表示成功：

```json
{ "code": 0, "message": "操作成功", "data": {}, "timestamp": 1710000000000 }
```

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/healthz` | 健康检查（环境、数据库、运行时长） |
| GET | `/api/v1/meta/enums` | 全部枚举字典 |
| GET | `/api/v1/hierarchy/tree` | 片区 / 道路层级树 |
| POST | `/api/v1/hierarchy/districts` · `/hierarchy/roads` | 新增片区 / 道路 |
| PUT | `/api/v1/hierarchy/districts/:id/rename` · `/hierarchy/roads/:id/rename` | 片区 / 道路改名（写变更记录） |
| PUT | `/api/v1/hierarchy/roads/:id/move` | 道路归属调整，引用该道路的管段一并迁移（单事务） |
| DELETE | `/api/v1/hierarchy/districts/:id` · `/hierarchy/roads/:id` | 删除层级（被引用时拒绝，返回 409） |
| GET | `/api/v1/hierarchy/districts/:id/refs` · `/hierarchy/roads/:id/refs` | 层级被管段 / 任务 / 记录 / 验收引用的计数 |
| POST | `/api/v1/hierarchy/adjustments` | 批量层级调整：一个事务内全部生效，任一失败整体回滚 |
| GET | `/api/v1/hierarchy/change-logs` | 层级变更记录（`nodeType` `action` `batchId` 分页） |
| GET | `/api/v1/dashboard/overview` | 看板总览指标 |
| GET | `/api/v1/dashboard/district-stats` | 分片区统计（按当前层级口径） |
| GET | `/api/v1/dashboard/pending-acceptance?limit=8` | 待验收任务 |
| GET | `/api/v1/dashboard/recent-records?limit=8` | 最近清淤记录 |
| GET | `/api/v1/pipe-segments` | 管段列表（`keyword` `districtIds` `roadIds` `pipeType` `status`，层级参数支持多选 / 全选） |
| GET | `/api/v1/pipe-segments/options` | 管段下拉选项（名称含当前片区 / 道路） |
| GET | `/api/v1/pipe-segments/:id` | 管段详情（含任务统计与最近任务） |
| GET | `/api/v1/pipe-segments/:id/cleaning-history` | 管段清淤履历 |
| POST/PUT/DELETE | `/api/v1/pipe-segments[/:id]` | 新增 / 修改 / 删除管段（片区 / 道路必须来自层级树） |
| GET | `/api/v1/cleaning-tasks` | 任务列表（`keyword` `status` `districtIds` `roadIds` `priority` `source` `pipeSegmentId` `planFrom` `planTo`，按登记当时的层级快照过滤） |
| GET | `/api/v1/cleaning-tasks/:id` | 任务详情（含清淤汇总、验收结论、可执行动作、层级名称快照） |
| POST/PUT/DELETE | `/api/v1/cleaning-tasks[/:id]` | 登记 / 修改 / 删除任务（登记时固化层级快照） |
| POST | `/api/v1/cleaning-tasks/:id/start` | 开工（待开工 → 清淤中） |
| POST | `/api/v1/cleaning-tasks/:id/complete` | 完工报验（清淤中 → 待验收，需至少 1 条清淤记录） |
| POST | `/api/v1/cleaning-tasks/:id/cancel` | 取消任务（需填写取消原因） |
| GET | `/api/v1/cleaning-records` | 记录列表（`keyword` `taskId` `segmentId` `districtIds` `roadIds` `method` `weather` `dateFrom` `dateTo`，按登记当时快照过滤） |
| GET | `/api/v1/cleaning-records/:id` | 记录详情（含所属任务、管段与层级名称快照） |
| POST/PUT/DELETE | `/api/v1/cleaning-records[/:id]` | 录入 / 修改 / 删除清淤记录（登记时固化层级快照） |
| GET | `/api/v1/acceptances` | 验收列表（`keyword` `taskId` `segmentId` `districtIds` `roadIds` `result` `inspectorName` `dateFrom` `dateTo` `pendingRectify`） |
| GET | `/api/v1/acceptances/:id` | 验收详情（含任务、清淤汇总与层级名称快照） |
| POST | `/api/v1/acceptances` | 登记验收结论（登记时固化层级快照） |
| POST | `/api/v1/acceptances/:id/rectify` | 登记整改完成 |
| DELETE | `/api/v1/acceptances/:id` | 删除验收记录（合格结论不允许删除） |

## 数据模型与状态流转

业务依赖方向单向：**清淤记录 → 清淤任务 → 管段台账 → 片区 / 道路层级**；验收记录同时依赖任务、管段、层级与清淤记录。
跨模块的反向引用检查与跨表统计统一收敛在 `internal/shared/refx`，避免模块间循环依赖。

### 片区 / 道路层级

- 片区（District）与道路（Road）组成两级层级，管段通过 `district_id` / `road_id` 引用，是台账、任务、记录、看板共用的唯一层级来源。
- 新增、改名、归属调整、删除全部收敛在 `internal/modules/hierarchy`，逐条写入 `hierarchy_change_logs`（同一次批量调整共用一个 `batch_id`，含操作人与原因、前后 JSON 快照）。
- 批量调整在单个数据库事务内完成，按主键顺序加锁，任何一项不合法都整体回滚，不会部分成功；道路归属调整时引用该道路的管段会一并迁移。
- 被管段、任务、清淤记录或验收记录引用的层级不允许直接删除（返回 409，并给出各类引用计数）；片区下仍有道路时也拒绝删除。
- 任务 / 清淤记录 / 验收记录在登记当时固化层级名称快照（`district_snapshot` / `road_snapshot`），片区改名或道路归属调整后历史数据仍按登记当时的层级展示；登记快照与层级调整通过行锁互斥，调整期间正在录入的数据只会取调整前或调整后其中一个确定口径。
- 升级旧版本（自由文本片区 / 道路）时，迁移逻辑会按同名归并自动建立层级并回填各表快照，迁移幂等。

清淤任务状态机（`internal/modules/cleaningtask/status.go` 唯一定义）：

```
pending ──开工──▶ in_progress ──完工报验──▶ completed ──验收合格──▶ accepted
   │                   │                        │
   └──取消──▶ cancelled └──取消──▶ cancelled    └──验收需整改──▶ in_progress
```

关键业务规则：

- 首次录入清淤记录时，任务自动从「待开工」推进到「清淤中」。
- 完工报验要求任务下至少有 1 条清淤记录。
- 验收登记要求任务处于「待验收」，且该任务没有已判定合格的验收记录；
  若上一次结论为「需整改」，必须先登记整改完成才能复验。
- 验收合格会在**同一个事务**内把任务置为「已验收」、写入验收时间，并把管段清淤次数 +1、
  回写最近清淤日期与运行状态。
- 删除保护：被任务引用的管段、已产生清淤记录或验收记录的任务、被验收记录引用的清淤记录、
  以及已合格的验收记录，都不允许删除。

## 校验与错误码

字段校验基于 `validator`，错误文案通过结构体 `label` 标签直接输出中文，例如「验证评分不能大于 100」。
分页参数自动规整：`page` 最小 1，`pageSize` 默认 10、最大 100。

| 业务码 | HTTP | 含义 |
| --- | --- | --- |
| `0` | 200 / 201 | 成功 |
| `40000` | 400 | 请求参数错误（含日期格式、JSON 解析失败） |
| `40001` | 400 | 字段校验不通过 |
| `40002` | 400 | 当前业务状态不允许该操作 |
| `40400` | 404 | 资源不存在 |
| `40900` | 409 | 唯一性或业务冲突（如管段编号重复、存在引用不允许删除） |
| `50000` | 500 | 服务内部错误 |

## 测试与验证

后端：

```bash
cd backend
go build ./...
go vet ./...
go test ./...          # httpx 与四个业务模块的单测（内存 SQLite）
gofmt -l .             # 应无输出
```

前端：

```bash
cd frontend
npm run typecheck      # tsc --noEmit
npm run build          # 类型检查 + 生产构建
```

容器编排自检：

```bash
docker compose up -d --build
curl http://localhost:8080/healthz
curl http://localhost:8081/healthz            # 经 nginx 反代到后端
curl http://localhost:8081/api/v1/meta/enums  # 前端同源访问接口
```

## 常见问题

**前端页面能打开但接口报 502？**
nginx 在后端不可达时会返回 JSON 形式的 502（`code: 50000`），先确认 `docker compose ps` 中
`desilting-backend` 为 healthy，再查看 `docker compose logs backend`。

**清淤记录录入失败提示任务状态不允许？**
只有「待开工」「清淤中」的任务可以录入或修改记录；已完工报验、已验收、已取消的任务都会被拒绝。

**验收时提示评分过低？**
判定合格要求评分不低于 60 分，低于该阈值请改判为「需整改」或修正评分。

**如何重置演示数据？**
`docker compose down -v` 删除数据卷后重新 `up`，即会重新建表并写入演示数据。

**本地不想装 PostgreSQL？**
设置 `DB_DRIVER=sqlite` 即可，迁移与业务代码在两种驱动下共用同一套实现。

**前端镜像构建时 npm 拉包很慢或失败？**
`frontend/Dockerfile` 默认使用 `https://registry.npmmirror.com` 安装依赖；如需切回官方源：

```bash
docker compose build --build-arg NPM_REGISTRY=https://registry.npmjs.org frontend
```

后端构建同理使用 `GOPROXY=https://goproxy.cn,direct`，可在 `backend/Dockerfile` 中调整。
