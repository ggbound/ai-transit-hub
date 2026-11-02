# Agent API 整体回归测试报告（2026-10-26）

> 范围：Agent API 全部 4 个接口 + 虚拟运行环境（managed CPython / Node）深入测试
> 方法：单元/集成测试回归 → HTTP 契约回归（正向 + 异常路径）→ 并发/隔离/失败注入/生命周期实证
> 结论：**契约回归 15/15 通过，环境深入测试 6 项全部通过，未发现功能性缺陷**

---

## 1. 测试环境与方法

| 项 | 值 |
|---|---|
| 服务 | backend（127.0.0.1:8080），实时运行 |
| 数据库 | MySQL 127.0.0.1:3306 / ai_transit_hub |
| Go 工具链 | GOTOOLCHAIN=go1.26.9 本地覆盖（go.mod 要求 1.25.0） |
| 鉴权 | API Key（`Authorization: Bearer sk_...`），v1 组中间件 |
| 路由前缀 | `/v1` 与 `/api/v1` 双前缀等价 |
| 测试技能 | 12 `pyenv-e2e`（python3 + pip:requests）、13 `nodeenv-e2e`（npm:dayjs）、14 `nodeenv-e2e-bad`（npm 404 失败注入，测试后已恢复禁用） |

方法分四层：

1. **静态回归**：`go build` / `go vet` / `go test ./...` 全量
2. **契约回归**：15 个 HTTP 用例覆盖鉴权、参数校验、技能/模型解析、任务与资产、前缀等价
3. **环境实证**：真实派发任务，验证 Python venv / Node 环境构建、执行、失败路径、并发隔离
4. **生命周期核查**：任务级 env 创建与清理、共享缓存、数据库 step trace、资产下载

---

## 2. 结论总览

| 测试层 | 结果 |
|---|---|
| go build / vet / test | 全绿 |
| 契约回归（15 用例） | **15/15 PASS** |
| Python venv 环境（创建→执行→隔离→清理） | PASS |
| Node 环境（宿主回退→npm 安装→执行→清理） | PASS |
| 并发三任务隔离 | PASS |
| max_iterations 钳制（100→20） | PASS |
| 语义路由（dispatch 仅传 input） | PASS |
| 非零退出码传播（exit 3 → 循环继续） | PASS |
| 失败注入（npm 404 包 → failed） | PASS（预期失败路径） |
| env 即时清理（成功/失败路径） | PASS |

---

## 3. Agent API 面清单

路由定义：[routes.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/routes.go#L259-L262)；处理器：[agent_task.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/agent_task.go)

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/v1/agent/tasks` | Create：`model` + `skills` + `input` 全必填 |
| POST | `/v1/agent/dispatch` | Dispatch：仅 `input` 必填；无 skills 时 LLM 语义路由 |
| GET | `/v1/agent/tasks/:taskId` | 轮询：状态 + steps trace |
| GET | `/v1/agent/tasks/:taskId/assets/*assetPath` | 资产下载（防路径穿越） |

Dispatch 模型优先级：路由模型 `req.model > AGENT_ROUTER_MODEL`；执行模型四级回退 `req.model > 技能声明 > 路由建议 > AGENT_ROUTER_MODEL`。

---

## 4. 基础回归（ra-2）

```
GOTOOLCHAIN=go1.26.9 go build ./...
GOTOOLCHAIN=go1.26.9 go vet ./...
GOTOOLCHAIN=go1.26.9 go test ./...
```

全部通过，无新增告警。

---

## 5. 契约回归明细（ra-3，15/15）

| # | 用例 | 期望 | 实测 | 结果 |
|---|---|---|---|---|
| A1 | Create 无 API Key | 401 `missing_api_key` | 一致 | PASS |
| A2 | Create 无效 API Key | 401 | 一致 | PASS |
| A3 | Dispatch 无 API Key | 401 | 一致 | PASS |
| A4 | Create 非法 JSON | 400 `invalid_json` | 一致 | PASS |
| A5 | Create 缺 model | 400 `model_required` | 一致 | PASS |
| A6 | Create 缺 skills | 400 `skills_required` | 一致 | PASS |
| A7 | Create 缺 input / 空 skills 数组 | 400 `input_required` / `skills_required` | 一致 | PASS |
| A8 | 未知技能 | 400 `skills_not_found` | 一致 | PASS |
| A9 | 禁用技能（nodeenv-e2e-bad） | 400 `skills_not_found`（消息含禁用技能名） | 一致 | PASS |
| A10 | embedding 模型 | 400 `model_not_supported`（非 text 模型拒绝） | 一致 | PASS |
| A11 | 未知模型 | **404 `provider_not_found`** | 一致 | PASS |
| A12a | 资产路径穿越（`../../`） | 400 `invalid_asset_name` | 一致 | PASS |
| A12b | 资产正常下载 | 200 + 文件内容 | `text/plain` 2683 字节，内容首行 `# Treatment — 山雾 SHANWU 15s` | PASS |
| A13 | 任务不存在 / 资产不存在 | 404 `task_not_found` / `asset_not_found` | 一致 | PASS |
| A14 | `/api/v1` 前缀等价 | 与 `/v1` 行为一致 | 一致 | PASS |

错误契约实测确认（与代码一致）：未知模型返回 **404**（非 400），由 handler 分类；401 由中间件层统一返回 `missing_api_key`。

---

## 6. 虚拟环境深入测试（ra-4，6 项全过）

### 6.1 并发三任务隔离（14:23:23 同时派发）

| 任务 | 技能 | 结果 | 耗时 | iterations |
|---|---|---|---|---|
| `agt_20261026142323_d0de1c1ec58dbe50` (PY) | pyenv-e2e | **completed** | 18s | 3 |
| `agt_20261026142323_8869576ff3dc2a20` (ND) | nodeenv-e2e | **completed** | 14s | 3 |
| `agt_20261026142323_2d29227206cc3ffe` (BD) | nodeenv-e2e-bad | **failed**（预期） | 3s | 0 |

三个任务并发构建各自独立 env，互不干扰；PY 的 `max_iterations` 传 100，落库为 **20**（钳制生效，上限见 [agent_task.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/agent_task.go#L31)）。

### 6.2 Python venv 隔离（PY 资产 result.txt 实证）

```
executable: .../storage/agent_tasks/_pyenvs/agt_.../bin/python3   ← venv 隔离解释器
python: 3.12.15                                                    ← managed CPython 3.12
requests: 2.34.2                                                   ← pip 安装到任务 env site-packages
```

- venv 创建于任务专属目录，解释器指向任务 env 而非宿主
- `pip install requests` 成功且版本可查询 → pip 可用性验证通过

### 6.3 Node 环境（ND 资产 result.txt 实证）

```
node: v22.23.2                                                     ← 宿主 Node 22（自托管回退链正常）
dayjs_location: .../agt_..._node/node_modules/dayjs/dayjs.min.js  ← npm 安装到任务 env
```

- 宿主无 node 时按 `resolveInterpreter` 回退链取自托管 node（[agent_sandbox.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/service/agent_sandbox.go#L156-L169)）
- `npm install dayjs` 装入任务级 `_node` 目录，require 路径正确

### 6.4 失败注入：npm 404 包（BD）

技能 14 声明 `@no-such-org-e2e/no-such-pkg-404`，npm registry 返回 404。任务 3 秒快速失败，错误链精确保留到 API 响应：

```
task node environment build failed: install npm packages
[@no-such-org-e2e/no-such-pkg-404]: exit status 1:
error 404 '@no-such-org-e2e/no-such-pkg-404@*' is not in this registry.
```

失败发生在 env build 阶段，iterations=0，无脏资产残留。

### 6.5 语义路由（RT = `agt_20261026142406_fde9745ff5374ff2`）

Dispatch 仅传 `input`（「用户需要验证Python venv隔离环境...」），未传 skills/model：

- **completed**，iterations=3
- 响应含 `routed: {skills:["pyenv-e2e"], model:"doubao/doubao-seed-2-0-mini-260428", reason:"..."}`
- DB `agent_task_steps` seq=0 落库 `skill_router` step（ok=1），trace 可审计
- 执行模型正确兜底为 `AGENT_ROUTER_MODEL`

### 6.6 非零退出码传播（EX = `agt_20261026142406_2e7bc959992a53e9`）

- **completed**，iterations=6
- seq=2 `execute_script`（check_venv.py）：`exit code: 0`，venv 四行输出正常
- seq=5 `execute_script`（fail.py）：**`exit code: 3`、`stdout: boom`** —— 非零退出码 + stdout 完整回传给模型，agent 循环继续而非中断
- 资产 `fail_result.txt`（内容 `boom`）可正常下载（200）

---

## 7. env 生命周期与清理

| 项 | 实测 |
|---|---|
| 任务级 env 即时清理 | BD 14:23:26、ND 14:23:37、PY 14:23:42 各自 `removed task env`（含 `_node` 后缀目录）——**成功与失败路径均清理** |
| wave-2 收尾核查 | `_pyenvs/` 下已无任何 `agt_20261024*`/`agt_20261026*` 任务目录，仅剩 `_npm-cache`、`_uv` 共享缓存（设计内常驻） |
| 工作区布局 | 任务工作区根 = `storage/agent_tasks/agt_xxx/`，资产文件直接在根（`AssetFilePath` = `workspaceRoot/taskID/name`） |

---

## 8. 现场恢复

- 测试用技能 13（nodeenv-e2e）、14（nodeenv-e2e-bad）已恢复禁用（`enabled=0`，与测试前一致）
- 恢复后复验：Create 携带 `["pyenv-e2e","nodeenv-e2e"]` → 400 `skills_not_found: [nodeenv-e2e]`（仅禁用项被拒，pyenv-e2e 正常解析）✅
- 测试期间产生的任务记录与工作区目录保留（可供追溯）

---

## 9. 发现与建议

无功能性缺陷。两点备忘：

1. **未知模型返回 404**（`provider_not_found`）而非 400——与 handler 的 provider 查找逻辑一致，属有意设计；若前端按「参数错误=400」处理需注意区分。
2. **`max_iterations` 静默钳制**（100→20）——行为正确但对调用方不可见（响应中无提示）。如需显式反馈，可在 Create/Dispatch 响应中附带 `max_iterations` 生效值。

---

## 附录：测试任务 ID 清单

| 任务 ID | 场景 | 终态 |
|---|---|---|
| agt_20261026142323_d0de1c1ec58dbe50 | Python venv 并发隔离 + 钳制 | completed |
| agt_20261026142323_8869576ff3dc2a20 | Node 环境并发 | completed |
| agt_20261026142323_2d29227206cc3ffe | npm 404 失败注入 | failed（预期） |
| agt_20261026142406_fde9745ff5374ff2 | 语义路由 | completed |
| agt_20261026142406_2e7bc959992a53e9 | 非零退出码传播 | completed |
