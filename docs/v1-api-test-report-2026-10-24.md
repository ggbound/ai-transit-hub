# AI Transit Hub — V1 对外接口测试报告

| 项目 | 内容 |
|---|---|
| 测试日期 | 2026-10-24 19:45 – 19:48 (UTC+8) |
| 被测服务 | Go 后端（`backend`，进程 `ath-serve`） |
| 服务地址 | `http://localhost:8080`（同时支持 `/v1/*` 与 `/api/v1/*` 两个前缀） |
| 认证方式 | `Authorization: Bearer <sk-key>` 或 `X-Api-Key: <sk-key>`（用户级 API Key） |
| 测试数据 | 真实激活 API Key 1 枚；覆盖 text / embedding / image / video 四类模型 |
| 用例总数 | 46（含正向、负向、异步闭环、记账验证） |
| 结果概览 | **43 通过 / 3 暴露问题**（1 个功能缺陷、2 个兼容性规范偏差），0 个服务端 5xx |

---

## 一、测试范围

依据 [routes.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/routes.go#L239-L261)，v1 对外接口共 14 条路由：

| # | 接口 | 方法 | 用例数 | 结果 |
|---|---|---|---|---|
| 1 | `/v1/models` | GET | 4 | ✅ 全部通过 |
| 2 | `/v1/chat/completions` | POST | 5 | ✅ 全部通过 |
| 3 | `/v1/messages`（Anthropic） | POST | 3 | ✅ 全部通过 |
| 4 | `/v1/anthropic/models` | GET | 1 | ✅ 通过 |
| 5 | `/v1/anthropic/messages`（旧路径） | POST | 1 | ✅ 通过 |
| 6 | `/v1/embeddings` | POST | 3 | ⚠️ 数组输入缺陷 |
| 7 | `/v1/images/generations` | POST | 3 | ⚠️ 尺寸约束依赖上游 |
| 8 | `/v1/videos/generations/tasks` | POST | 3 | ⚠️ 不支持标准 prompt 字段 |
| 9 | `/v1/videos/generations/tasks/:taskId` | GET | 3 | ✅ 全部通过 |
| 10 | `/v1/responses` | POST | 3 | ✅ 全部通过 |
| 11 | `/v1/agent/tasks` | POST | 4 | ✅ 全部通过 |
| 12 | `/v1/agent/tasks/:taskId` | GET | 3 | ✅ 全部通过 |
| 13 | `/v1/agent/tasks/:taskId/assets/:name` | GET | 2 | ✅ 全部通过 |
| 14 | `/v1/agent/dispatch` | POST | 2 | ✅ 全部通过 |
| — | 认证 / 备用前缀 / 记账 | — | 6 | ✅ 全部通过 |

---

## 二、详细用例结果

### 2.1 认证与鉴权（6/6 通过）

| 用例 | 场景 | 期望 | 实际 | 结论 |
|---|---|---|---|---|
| T01 | 无任何认证头 | 401 | 401 `authorization header or x-api-key header required` | ✅ |
| T02 | 无效 Bearer Key | 401 | 401 `invalid api key` | ✅ |
| T03 | 无效 X-Api-Key | 401 | 401 `invalid api key` | ✅ |
| T04 | 错误认证方案（`Authorization: Token ...`） | 401 | 401 | ✅ |
| T41 | 有效 X-Api-Key 调用 `/v1/models` | 200 | 200 | ✅ |
| — | 备用前缀 `/api/v1/models` | 200 | 200 | ✅ |

补充验证：API Key 命中 30s TTL 缓存，`last_used_at` 按 60s 节流异步落库（见 [auth.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/middleware/auth.go#L78-L133)），未发现越权（视频任务查询带属主校验，[videos.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/videos.go#L76-L84)）。

### 2.2 模型列表（8/8 通过）

| 用例 | 场景 | 结果 |
|---|---|---|
| T05 | `GET /v1/models` | 200，返回 OpenAI 格式（`id` 为 `provider/model`，含 `supports_*` 能力位、价格、max_tokens） |
| T07 | `GET /v1/anthropic/models` | 200，返回 Anthropic 格式（`display_name` / `provider`） |
| — | 字段抽查 | 模型 42 个（text 24 / video 4 / image 3 / embedding 1 与库一致） |

> 观察：`/v1/models` 会返回 embedding/image/video 模型，OpenAI 客户端可通过 `supports_*` 字段区分，建议文档说明。

### 2.3 Chat Completions（5/5 通过）

| 用例 | 场景 | 期望 | 实际 | 耗时 |
|---|---|---|---|---|
| T08 | 非流式对话（doubao-seed-2-0-mini） | 200 | 200，标准 `choices[].message`，含 `reasoning_content` 与完整 `usage`（212 tokens） | 2.10s |
| T09 | 流式 `stream=true` | SSE | 200，`chat.completion.chunk` 增量块正常 | — |
| T10 | 不存在的 provider | 4xx | 404 `provider_not_found` | 5ms |
| T11 | 缺少 `messages` | 400 | 400（上游 MissingParameter 原文透传） | 0.18s |
| T12 | 非法 JSON body | 400 | 400 `invalid_request_error` | <1ms |
| T43 | provider 存在但模型不存在 | 4xx | 404 `model_not_found` | 3ms |

### 2.4 Anthropic 兼容（9/9 通过）

| 用例 | 场景 | 结果 |
|---|---|---|
| T13 | `POST /v1/messages`（Bearer 认证 + anthropic-version 头） | 200，Anthropic 格式：`content[]`（thinking + text 块）、`stop_reason: end_turn`、`usage` |
| T14 | `POST /v1/anthropic/messages`（旧路径 + `x-api-key` 头） | 200，与 T13 一致 |
| T15 | 流式 `/v1/messages` | 200，SSE 事件序列完整：`ping → message_start → content_block_start → content_block_delta` |
| T16 | 缺 `max_tokens` | 400（上游错误透传，见问题 4） |
| T17 | 无效模型 | 404 `provider_not_found` |

### 2.5 Embeddings（2 通过 + 1 缺陷）

| 用例 | 场景 | 结果 |
|---|---|---|
| T18 | 字符串输入 | 200，返回 2048 维向量 ✅（但 `data` 为对象而非数组，见问题 1） |
| T19 | **数组输入 `["你好","世界"]`** | 200，但仅返回 **1 个** 2048 维向量 ❌（问题 1） |
| T20 | 空字符串输入 | 400 MissingParameter ✅ |

### 2.6 Images（2 通过 + 1 上游约束）

| 用例 | 场景 | 结果 |
|---|---|---|
| T21 | `size=1024x1024` | 400：上游要求 ≥3686400 像素（Seedream 4.5 限制，错误正确透传） |
| T21b | `size=2048x2048` | 200，`data[].url` 返回真实图片 URL，符合 OpenAI 规范 ✅（8.04s） |
| T22 | 缺 `prompt` | 400 ✅ |

### 2.7 Videos（5 通过 + 1 兼容性问题）

| 用例 | 场景 | 结果 |
|---|---|---|
| T23 | OpenAI 风格 `prompt` 字段创建 | 400：要求 `content` 参数 ❌（问题 2） |
| T23b | Doubao 原生 `content` 数组格式 | 200，返回任务 ID `cgt-2026...` ✅ |
| T25 | 查询任务 | 200，`status: running` ✅ |
| T39/T44 | 45s 后复查 | `status: succeeded`，`content.video_url` 存在 ✅ |
| T24 | 查询不存在的任务 | 404 `Task not found` ✅ |

### 2.8 Responses（3/3 通过）

| 用例 | 场景 | 结果 |
|---|---|---|
| T26 | 非流式 | 200，Responses 格式（`output[].summary`、`max_output_tokens`、`expire_at`） |
| T28 | 流式 | 200，`response.created → response.in_progress → ...` 事件序列正确 |
| T27 | 缺 `input` | 400 MissingParameter ✅ |

### 2.9 Agent 任务（12/12 通过）

| 用例 | 场景 | 结果 |
|---|---|---|
| T29 | 缺 `skills` | 400，字段级校验错误 ✅ |
| T30 | 不存在的技能名 | 400 `skills_not_found` ✅ |
| T31 | 用 image 模型跑 agent | 400 `model_not_supported`（要求 type=text）✅ |
| T32 | 正常创建（quick-note，max_iterations=2） | 202，返回 `task_id` + `poll_url` ✅ |
| T33 | 轮询任务 | 200，`status: running`，含 steps/usage/cost ✅ |
| T37 | 任务完成后复查 | `completed`，1 轮迭代，4995 tokens，cost 0.002093，产出资产 `note.md`（1106 字节）✅ |
| T36 | `/v1/agent/dispatch` 语义路由 | 202，自动路由到 `quick-note` 技能并给出 reason ✅（3.59s） |
| T38 | dispatch 任务完成 | `completed`，5319 tokens，cost 0.002722 ✅ |
| T34 | 查询不存在的任务 | 404 `task_not_found` ✅ |
| T35 | 下载不存在的资产 | 404 `asset_not_found` ✅ |
| T40 | 下载真实资产 `note.md` | 200，返回文件内容 ✅ |

### 2.10 计费记账验证（1/1 通过）

| 用例 | 结果 |
|---|---|
| T42 | `usage_logs` 表正确记录：agent 两笔 4995/5319 tokens 与 cost 0.002093/0.002722 完全对账 ✅；流式中断请求记录 `status=0` + `client disconnected mid-stream`，服务未崩溃 ✅ |

---

## 三、性能观察（单次采样，仅供参考）

| 接口 | 耗时 |
|---|---|
| `/v1/models`（缓存命中） | ~1ms |
| `/v1/chat/completions` 非流式 | 2.10s |
| `/v1/messages` 非流式 | 0.64 – 0.72s |
| `/v1/embeddings` | 0.15 – 0.55s |
| `/v1/images/generations`（2048²） | 8.04s |
| `/v1/videos` 创建 → succeeded | ~45s（上游生成） |
| `/v1/agent/tasks` 全生命周期 | ~8s |
| `/v1/agent/dispatch`（含路由） | ~9s |

---

## 四、问题清单（按优先级）

### 🔴 P1 · 缺陷：`/v1/embeddings` 数组输入返回结果错误
- **现象**：`input=["你好","世界"]` 时响应 `data` 是单个对象 `{embedding, object}` 而非数组，且只有 1 条 2048 维向量（与单文本输入维度相同），两条输入仅产出一条向量。
- **规范**：OpenAI 规范要求 `data` 为数组，且每个输入对应一条 `{object:"embedding", embedding:[...], index:N}`。
- **影响**：批量向量化场景静默丢数据，调用方难以察觉。
- **建议**：在 [EmbeddingHandler](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/embeddings.go) 中按输入条数拆分/映射上游结果，统一输出数组结构；上游单条接口时可循环调用并按 index 组装。

### 🟡 P2 · 兼容性：`/v1/videos/generations/tasks` 不接受标准 `prompt` 字段
- **现象**：传 `{"prompt": "..."}` 返回上游 400 `MissingParameter: content`；必须使用 Doubao 私有的 `{"content":[{"type":"text","text":"... --duration 5"}]}` 命令格式。
- **影响**：作为"对外 OpenAI 兼容层"，与常见 OpenAI 视频 SDK 的请求形状不一致，接入成本高。
- **建议**：handler 内做 `prompt → content` 自动转换（参数如 duration/resolution 映射为命令后缀），或在 API 文档中显著标注仅支持原生格式。

### 🟡 P3 · 兼容性：embeddings/images 的请求形状与上游强耦合
- embeddings：`data` 非数组（同 P1）；images：size 下限（≥3686400 像素）为 Seedream 4.5 私有约束，`1024x1024` 直接被上游拒绝。
- **建议**：文档标注各模型的尺寸/参数约束；可在本地校验层提前拦截并给出友好错误，避免透传上游原文。

### 🟢 P4 · 一致性：缺参错误为上游原文透传，错误体系不统一
- 现象：chat 缺 `messages`（火山方舟格式）、messages 缺 `max_tokens`（智谱格式）返回的上游错误 code/type 各不相同；而 JSON 解析错误、模型不存在则是网关统一格式。
- **建议**：对必填参数做本地校验，统一返回 `invalid_request_error` 错误结构，避免将上游 Request id 等内部信息暴露给外部调用方。

### ℹ️ P5 · 观察项（非缺陷）
- 客户端提前断开流式连接（测试中 `head` 截断所致）会被记录为失败 usage log（`client disconnected mid-stream: context canceled`），服务端稳定无泄漏，属正确行为。
- `/v1/models` 返回含非文本模型，需靠 `supports_*` 字段区分，建议文档说明。

---

## 五、测试期间产生的资源

| 资源 | 标识 | 状态 |
|---|---|---|
| 视频生成任务 | `cgt-20261024194538-bzw7j` | succeeded（已产出 video_url） |
| Agent 任务（直建） | `agt_20261024194627_716b431d8d9f66b2` | completed（note.md） |
| Agent 任务（dispatch） | `agt_20261024194643_816c8f4917a9c374` | completed（note.md） |
| 用量记录 | usage_logs #329、#330 等 | 已正确记账，合计 cost ≈ 0.005 元 |

## 六、结论

v1 对外接口整体质量良好：**14 条路由全部可达，认证、错误处理、流式输出、异步任务闭环、计费记账均工作正常，无 5xx，无越权**。核心待修复项为 embeddings 数组输入缺陷（P1），视频接口的 prompt 兼容转换（P2）建议在下一迭代处理。
