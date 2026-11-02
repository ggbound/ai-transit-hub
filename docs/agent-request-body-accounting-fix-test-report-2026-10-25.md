# Agent 链路 request_body 落库缺失修复测试报告

- **日期**：2026-10-25
- **问题等级**：中（数据完整性缺陷：记账审计字段 request_body 持续丢失，不影响核心功能与计费金额）
- **影响组件**：[backend/internal/api/handlers/agent_task.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/agent_task.go)（Agent 任务记账链路）
- **修复文件**：[agent_task.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/agent_task.go)（16 处编辑，单文件）
- **验证环境**：backend 二进制重启于 14:02:05（`go vet` + `go build` 通过）；MySQL 127.0.0.1/ai_transit_hub
- **关联文档**：
  - 适配器统一化回归报告（同日）：[adapter-unification-test-report-2026-10-25.md](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/docs/adapter-unification-test-report-2026-10-25.md)

---

## 1. 执行摘要

用户反馈 `api_test_details.request_body` 近期数据未落库（值为字符串 `"null"`）。排查确认**缺陷仅存在于 Agent 任务链路**（`/v1/agent/tasks`、`/v1/agent/dispatch` 及 `:video` 子调用），根因是记账调用从未传入真实请求体（传的是透传参数 `options` 或硬编码 `nil`）。修复后 3 组实测全部通过，标准 /v1 链路无回归。

| 项目 | 结果 |
|---|---|
| 影响面 | Agent 链路 10 条 null 记录（2026-10-25 08:43:07 ~ 13:44:09）；标准 /v1 链路（chat/embeddings/images/videos）**不受影响** |
| 根因 | ① `agentRequestMeta` 快照缺 Body 字段；② 主任务记账误传 `options`（nil map）；③ 子调用记账硬编码 `nil`；`json.Marshal(nil)` 输出 `"null"` |
| 修复规模 | agent_task.go 单文件 16 处：新增 `captureRawBody`、meta 增加 `Body json.RawMessage`、8 个记账调用点统一改传 `meta.Body` |
| 静态验证 | `go vet` + `go build` 一次通过 |
| 实测验证 | **3/3 用例通过**（agent/tasks 无 options、agent/dispatch 语义路由、agent/tasks 带 options），`is_null` 全 0 |
| 副作用 | 无：`options` 仍正常透传给 Agent 运行时；`json.RawMessage` 落库保真不双重编码 |
| 遗留 | ① 历史 10 条 null 记录不可回填（原始请求体当时未保存）；② 本修复与同日其他改动（generate_audio 透传等）**尚未 commit** |

---

## 2. 问题描述

用户反馈：`api_test_details` 表的 `request_body` 字段最近的几次数据没有存进去。

落库值为 4 字节字符串 `"null"`（而非 NULL 或空串），该特征直接指向 `json.Marshal(nil)` 的输出——即记账调用方把 nil 传给了 requestBody 参数，而非数据库写入失败。

## 3. 排查过程与影响面

### 3.1 影响面收窄（detail ← request 正确 JOIN）

`api_test_details` 与 `api_test_requests` 是两套自增 id，必须以 `detail.request_id` 对齐。当日全部 Agent 链路记录：

| detail | request | endpoint | 请求时间 | request_body |
|---|---|---|---|---|
| 491 | 496 | agent/dispatch | 08:43:07 | `"null"` |
| 492 | 495 | agent/dispatch | 08:43:07 | `"null"` |
| 493 | 497 | agent/tasks | 08:43:29 | `"null"` |
| 494 | 498 | agent/tasks | 08:47:35 | `"null"` |
| 495 | 499 | agent/tasks | 08:53:04 | `"null"` |
| 496 | 500 | agent/tasks | 08:55:04 | `"null"` |
| 497 | 501 | agent/tasks | 09:02:48 | `"null"` |
| 498 | 502 | /v1/agent/tasks:video | 09:05:10 | `"null"` |
| 499 | 503 | agent/tasks | 13:42:13 | `"null"` |
| 500 | 504 | /v1/agent/tasks:video | 13:44:09 | `"null"` |
| 502 | 506 | agent/tasks | 14:02:21 | **正常**（修复后） |
| 503 | 508 | agent/tasks | 14:03:42 | **正常**（修复后） |
| 504 | 507 | agent/dispatch | 14:03:42 | **正常**（修复后） |

### 3.2 对照组（排除标准链路与二进制版本因素）

- detail 487-490（embeddings/videos/images，08:18-08:21，与最早 null 记录**同实例**）：request_body 正常；
- 修复前实时探针：`POST /v1/embeddings`（13:57:59）→ detail 501 正确保存 `{"input":"rb-probe-1025",...}`，证明当时运行的二进制标准链路记账无问题。

### 3.3 排查结论

1. **非全局回归**：只有 Agent 任务链路（agent/dispatch ×2、agent/tasks ×6、:video 子调用 ×2）为 "null"；
2. **自上线即存在**：最早 null 记录（08:43:15）产生于与正常记录同一服务实例，Agent 链路为新增功能，该缺陷大概率一直存在；
3. 请求体在 handler 层可得（中间件 `[API-REQUEST] Body` 日志有完整记录），是记账传参丢失而非采集失败。

## 4. 根因分析

三个缺陷点叠加（均在 [agent_task.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/agent_task.go)）：

1. **`agentRequestMeta` 快照缺 Body 字段**：goroutine 内记账可用的上下文只有 Method/Endpoint/Headers/ApiKeyID，从结构上就无法把请求体传入异步记账（修复前 L84-89）；
2. **主任务成功记账误传 `options`**：`UpdateSuccessRecords(..., options, model)` 把透传参数 `req.Options` 当作 requestBody 落库。语义错误 + 用户请求通常不带 options 字段（nil map）；
3. **子调用成功记账硬编码 `nil`**：`billSubModelCalls` 的成功/失败分支均传 nil，恒为 "null"。

**机制**：`json.Marshal(nil)`（nil map / nil interface{}）输出 4 字节字符串 `"null"`，被原样写入 detail.request_body——与观测现象完全吻合。同日 13:42 任务（detail 499/500）用户请求带了 `generate_audio` 之外的常规字段但无 options，故仍为 "null"。

标准 /v1 链路走 [v1_pipeline.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/v1_pipeline.go) 传完整重建的 `reqData`，因此一直正常。

## 5. 修复内容（16 处编辑）

| # | 改动 | 位置 |
|---|---|---|
| 1 | imports 增加 `bytes`、`io` | L3-15 |
| 2 | `agentRequestMeta` 新增 `Body json.RawMessage` 字段 | [L86-92](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/agent_task.go#L86-L92) |
| 3 | 新增 `captureRawBody`：`io.ReadAll` 读原始 body 后恢复 `c.Request.Body`（避免被 `ShouldBindJSON` 消费丢失），返回 `json.RawMessage` | [L95-102](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/agent_task.go#L95-L102) |
| 4/5 | `Create` / `Dispatch` 入口绑定前捕获 `rawBody` | [L105](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/agent_task.go#L105) / [L169](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/agent_task.go#L169) |
| 6/7 | 两处 `launchAgentTask` 调用透传 `rawBody` | L152 / L253 |
| 8/9 | `launchAgentTask` 签名增加参数；meta 构造赋值 `Body: rawBody` | [L620](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/agent_task.go#L620) / L680 |
| 10-17 | **8 个记账调用点统一改传 `meta.Body`**：panic 分支 L340、4 类错误分支 L354/364/378/425、主任务成功 L432、子调用失败 L478、子调用成功 L492 | [runAgentTask](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/agent_task.go#L332-L440) / [billSubModelCalls](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/agent_task.go#L441-L493) |

**保真说明**：`json.RawMessage` 的 Marshal 直接输出原始字节，落库即用户原始请求（不双重编码、不丢字段）；仅在读 body 失败的极端情况返回 nil（行为与修复前一致）。`options` 参数仍正常透传给 `runtime.RunTask`，不受影响。

## 6. 测试验证

编译通过（`go vet ./internal/api/handlers/` + `go build`）后于 14:02:05 重启服务，执行 3 组实测用例：

| # | 用例 | 任务 ID | detail | 验证点 | 结果 |
|---|---|---|---|---|---|
| 1 | agent/tasks，不带 options，quick-note 技能 | agt_20261025140221_8eed0c86d8b66259 | 502 | 修复主场景（旧逻辑必为 "null"） | ✅ completed，完整请求体落库 |
| 2 | agent/dispatch，语义路由（无显式 skills/model） | agt_20261025140341_58e7fa9ca6884f9a | 504 | 路由入口（旧逻辑必为 "null"） | ✅ completed，正确路由至 quick-note |
| 3 | agent/tasks，**带 options**（temperature=0.5） | agt_20261025140342_1679b8be264fdbce | 503 | options 存在时请求体不退化为仅剩 options | ✅ completed |

**落库实际值**（3 条记录 `is_null` 全 0，response_body 亦正常）：

```jsonc
// detail 502（agent/tasks，无 options）
{"model":"doubao/doubao-seed-2-0-mini-260428","skills":["quick-note"],"input":"rb-probe-1025 记录一条备注：request_body 落库复测","max_iterations":2}

// detail 504（agent/dispatch，语义路由）
{"input":"rb-probe-dispatch 记一条备注：dispatch 链路 request_body 复测","max_iterations":2}

// detail 503（agent/tasks，带 options）—— options 成为请求体的一个字段，而非替代请求体
{"model":"doubao/doubao-seed-2-0-mini-260428","skills":["quick-note"],"input":"rb-probe-options 记一条备注：带 options 复测","max_iterations":2,"options":{"temperature":0.5}}
```

**修复前后对比**（以带 options 请求为例）：

| | 修复前 | 修复后 |
|---|---|---|
| 不带 options | `"null"` | 完整原始请求体 |
| 带 options | `{"temperature":0.5}`（丢失 model/skills/input/max_iterations） | 完整原始请求体（options 为其中一个字段） |

**回归确认**：标准 /v1 链路当日探针（detail 501，embeddings）正常，本次修复未触碰 v1_pipeline 路径；`:video` 子调用与主任务共用同一 `meta.Body` 快照机制（唯一入口 `launchAgentTask`），代码路径一致。

## 7. 结论与遗留事项

**结论**：根因确认为 Agent 链路记账调用丢失请求体（options 误传 + nil 硬编码 + meta 缺字段），修复后 3/3 实测通过，问题闭环。

**遗留事项**：

1. **历史数据不可回填**：10 条 "null" 记录的原始请求体在缺陷期间未被保存，中间件日志中的 `[API-REQUEST] Body` 行可作人工核对参考，但不建议自动化回填；
2. **待 commit**：本修复与同日 generate_audio 透传等改动均未入库（工作区存在多处修改，最后 commit 为 `dbee185`），需要提交请告知；
3. 可选改进（未实施）：`ApiTestDetail.ResponseHeaders` 仍硬编码 `"{}"`（[accounting_service.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/service/accounting_service.go) L252），如需请求头对等落库可另行处理。
