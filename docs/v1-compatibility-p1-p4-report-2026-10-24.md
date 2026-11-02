# AI Transit Hub — V1 兼容性改造总结报告（P1–P4）

| 项目 | 内容 |
|---|---|
| 改造日期 | 2026-10-24（P1 于同日早些完成，P2–P4 于 19:45 测试报告发布后依次完成） |
| 范围 | Go 后端（`backend`）v1 对外 OpenAI 兼容层（`/v1/*` 与 `/api/v1/*` 双前缀） |
| 依据 | [V1 对外接口测试报告](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/docs/v1-api-test-report-2026-10-24.md)暴露的兼容性问题 + 会话中确认的能力元数据方案 |
| 状态 | **P1–P4 全部完成并逐项验证通过**（真实上游：豆包 Ark / 火山方舟） |

---

## 一、改造总览

v1 对外层采用**三层兼容策略**，四项改造分别落在对应层：

| # | 改造项 | 层级 | 核心文件 | 状态 |
|---|---|---|---|---|
| P1 | Embeddings 数组输入修复（多向量语义归一化） | L1 双向翻译 + 出口归一化 | [doubao_adapter.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/doubao_adapter.go)、[embeddings.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/embeddings.go) | ✅ 完成 |
| P2 | Videos `prompt`/`content` 适配（请求侧转换 + 响应侧别名） | L1 适配 + L3 显式冲突 | [videos.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/videos.go)、[doubao_adapter.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/doubao_adapter.go) | ✅ 完成 |
| P3 | `/v1/models` 能力元数据（`compatibility` 块） | 契约可发现性 | [openai.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/openai.go)、[adapter_factory.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/adapter_factory.go) | ✅ 完成 |
| P4 | 错误信封统一（OpenAI `{"error":{...}}` 形状） | 一致性 | 6 个文件（见 §五） | ✅ 完成 |

> **编号说明**：测试报告中的 P3（请求形状与上游强耦合）与 P4（错误体系不统一）编号不同——本报告 P3 为能力元数据、P4 为错误信封统一（按会话确认的方案编号执行）。测试报告 P3 的 images 尺寸约束部分见 §七遗留事项。

**统一原则**（四项改造共同遵守）：

1. **只做结构适配，不改语义**：L1 标准字段双向翻译；L2 未知字段原样透传（escape hatch）；L3 语义冲突显式 400，**不猜**。
2. **只声明实际保证**：能力元数据仅描述网关代码真实执行的行为，不重复 admin 配置。
3. **透传保真**：上游错误体（2xx/4xx body）原样透传，不重写、不吞字段。
4. **客户端输入校验放 handler 层**（可返回 400）；provider 特定转换放适配器层——因非 UpstreamError 错误会被映射为 500（[base.go passthroughErrStatus](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/base.go#L185-L201)）。

---

## 二、P1 · Embeddings 数组输入修复

### 问题（测试报告 T18/T19，🔴 缺陷）

`input=["你好","世界"]` 时响应 `data` 是单个对象而非数组，且仅返回 1 条向量——**批量向量化静默丢数据**。

### 根因

豆包多模态嵌入端点（`/embeddings/multimodal`）的 `input` 语义是"**一个向量的内容项列表**"（N 项内容共同构成 1 条向量），与 OpenAI 的"N 个独立文本 → N 条向量"语义直接冲突。

### 方案与实现

**适配层**（[doubao_adapter.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/doubao_adapter.go#L86-L186)）：

- `multimodalEmbeddings`：多文本输入时**逐条单独请求**（并发上限 8，防打爆上游），按 `index` 合并为标准 `data[]`，`usage` 逐条累加；单文本直连单请求
- `extractEmbeddingVector`：兼容上游 `data` 为单对象/数组两种形状
- `mergeUsageTokens`：合并 `prompt_tokens` / `completion_tokens` / `total_tokens`

**出口归一化**（[embeddings.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/embeddings.go#L121-L170)）：

- `normalizeEmbeddingsResponse`：`data` 为单对象时包装为 `[{...index:0}]`；数组项缺失 `index`/`object` 时按顺序补齐；顶层缺失 `object:"list"`、`model` 时补充
- 对已是标准结构的响应**幂等无害**，通用路径（非豆包多模态）也获得兜底保证

### 保证（已写入 P3 能力元数据）

- `data` 恒为数组，每项含 `object:"embedding"` 与 `index`
- N 个输入 → N 条向量（`multi_input: one_vector_per_input`）

---

## 三、P2 · Videos `prompt`/`content` 适配

### 问题（测试报告 T23，🟡 兼容性）

OpenAI 风格 `{"prompt": "..."}` 被上游 400 拒绝（`MissingParameter: content`）——Ark 会忽略顶层 `prompt`，提示词必须放 `content` 数组。

### 方案（用户确认原文）

> 请求侧做 L1 适配——只有 `prompt` → 自动转 `content` 数组；只有 `content` → 原生透传；两者都有 → 400 显式报"参数冲突"，不猜。响应侧在顶层补 `video_url`/`status` 标准别名。

### 实现分三层

| 层 | 文件 | 行为 |
|---|---|---|
| Handler 校验（L3） | [videos.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/videos.go#L43-L57) | `prompt`+`content` 并存 → 400 `prompt_content_conflict`；`prompt` 非 string → 400 `invalid_prompt_type` |
| 适配器转换（L1） | [doubao_adapter.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/doubao_adapter.go#L384-L413) | `PassthroughRequest` video 类型：仅 `prompt` → 自动转 `content` 数组（`[{"type":"text","text":prompt}]`）；有 `content` 保持原样；`ratio`/`duration`/`seed`/`watermark`/`generate_audio` 等顶层参数 Ark 原生接受，不动 |
| 响应别名 | [videos.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/videos.go#L192-L215) | `GetTask` 调用 `normalizeVideoTaskResponse`：`content.video_url`/`content.status` 缺顶层时提升为**别名**（不覆盖上游原始字段；无嵌套结构时安全跳过） |

### 验证（真实 Ark 上游，S1–S5 全通过）

| 场景 | 结果 |
|---|---|
| S1 仅 `prompt` | 200，任务 `cgt-20261024200756-nqbd4`（未转换时 Ark 会因缺 `content` 拒绝 → 200 证明转换生效） |
| S2 仅 `content`（回归） | 200，任务 `cgt-20261024200756-df8g4` |
| S3 两者并存 | 400 `prompt_content_conflict`（标准 OpenAI 信封） |
| S3b `prompt` 非 string | 400 `invalid_prompt_type` |
| S4 历史任务查询（`cgt-20261024194538-bzw7j`） | 顶层补出 `video_url` 别名，嵌套原字段保留 |
| S5 running 中任务（响应无 `content`） | 归一化安全跳过，200 |
| 记账 | 仅 S1/S2 建 usage 记录（#334/#335）；400 场景无记录（校验在 `prepareV1Request` 之前）✅ |

---

## 四、P3 · 能力元数据（`/v1/models` compatibility 块)

### 目标

客户端可**程序化发现**端点、请求形状与网关响应保证，而非靠文档或试错。

### Schema

每个模型对象新增 `compatibility` 块，三段式：

```json
{
  "id": "doubao/doubao-seedance-2-0-260128",
  "capabilities": ["vision"],          // 原有自由格式字段，原样保留
  "compatibility": {
    "endpoints": { "create": "POST /v1/videos/generations/tasks",
                   "query":  "GET /v1/videos/generations/tasks/{task_id}" },
    "request":  { "prompt": "string",
                  "content": "multimodal_content[]",
                  "exclusive_fields": ["prompt", "content"] },
    "response": { "status": "top_level", "video_url": "top_level_alias" }
  }
}
```

### 推导规则

- 由 **model type + provider 适配器家族**推导，**不新增 admin 配置**
- 适配器家族归一化：[adapter_factory.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/adapter_factory.go#L43-L53) 新增导出方法 `NormalizedAdapterClass`（兼容 PHP 命名空间格式，`App\Services\AI\Adapters\DoubaoAdapter` → `doubao`；volcengine 视为 doubao 同族，与 `CreateAdapterWithApiKey` 分支一致）
- 多模态嵌入判定**镜像适配器真实解析逻辑**（[adapter.go buildUrl](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/adapter.go#L96-L119)）：生效端点 = `custom_endpoint` 优先、`embeddings_endpoint` 兜底（修复了初版只查 EmbeddingsEndpoint 的 bug——DB 实证模型 14 的 multimodal 端点在 custom_endpoint）

### 各类型元数据（验证于 32 个模型全量）

| 类型 | endpoints | request | response |
|---|---|---|---|
| text | `chat` 恒有；`responses`（SupportsResponse）；`messages`（SupportsMessages） | `messages: array`、`stream: bool` | — |
| embedding | `create: POST /v1/embeddings` | `input: ["string","string[]"]`；多模态模型加 `"multimodal_content[]"` | `data: array`、`multi_input: one_vector_per_input` |
| image | `create: POST /v1/images/generations` | `prompt: string` | — |
| video | `create` + `query`（task_id 占位） | `prompt: string`；豆包系加 `content: multimodal_content[]`、`exclusive_fields: [prompt, content]` | `status: top_level`、`video_url: top_level_alias` |

### 验证结果

- doubao-embedding-vision-251215：`input` 含 `multimodal_content[]` ✅
- text 类按 DB 动态：豆包系无 `messages` 端点、ZhiyunTB deepseek 有 `responses` ✅
- image/video 全部正确；自由格式 `capabilities` 原样并存 ✅

---

## 五、P4 · 错误信封统一

### 目标形状

```json
{"error": {"message": "...", "type": "invalid_request_error|api_error", "param": "...?", "code": "..."}}
```

### 修复的 7 处不一致点

| # | 位置 | 原状 | 修复 |
|---|---|---|---|
| 1 | [videos.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/videos.go#L93-L187) GetTask 5 处 | `{"status":"error","message":...}`（内部风格泄漏） | OpenAI 信封：`record_not_found` / `model_unresolved`（400→500，网关配置问题）/ `adapter_creation_failed` / `task_not_found` / `task_query_failed` |
| 2 | [auth.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/middleware/auth.go#L96-L131) ApiKeyAuthMiddleware 3 处 401 | `{"error":"string"}`（error 为字符串） | `missing_api_key` / `invalid_api_key` / `api_key_expired`（admin 用 JWT 中间件不动） |
| 3 | [api_logger.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/middleware/api_logger.go#L38-L45) 413 | 同上 | `request_too_large`（该中间件仅挂 v1 路径） |
| 4 | [openai.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/openai.go) / [anthropic.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/anthropic.go) ListModels 500 | `{"error":"string"}` | `models_fetch_failed` |
| 5 | [v1_pipeline.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/v1_pipeline.go#L204-L216) respondV1Success | 上游错误响应体**原样透传**（非标形状泄漏） | `error` 已是对象 → 原样保真；其余形状（error 字符串 / success:false / status:error）→ 包装标准信封 `upstream_error` |
| 6 | [base.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/base.go#L208-L215) respondPassthroughErr 兜底 | `code` 为数字（ue.StatusCode） | 字符串 `"upstream_error"`（对齐 OpenAI 惯例） |
| 7 | [routes.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/routes.go#L263-L279) NoRoute | gin 默认纯文本 `404 page not found` | /v1 与 /api/v1 前缀 → `unknown_url` 信封（含 `Invalid URL (METHOD /path)`）；其他前缀保持默认（管理后台不受影响） |

### 边界（保持不变）

- **上游错误体透传保真**：`ue.Body != nil` 时原样透传（上游契约保真是正确行为，如 Ark 的 400 原文）
- **必填参数本地校验未引入**：透传架构下保留上游校验，避免双份校验漂移（见 §七）
- 流式已开始后的错误（SSE 无法改信封）维持原记账行为

### 验证（9 场景全通过）

401 无效 key / 404 未知 v1 URL（GET、POST 各一）/ 非 v1 路径 404 保持原样 / 上游 400 透传保真 / 404 model_not_found（回归）/ 400 prompt_content_conflict（回归）/ videos task_not_found（新信封）/ 200 models 正常且 P3 元数据完好。

---

## 六、v1 客户端契约速查（改造后）

**调用契约**：统一 OpenAI 形状；网关自产错误恒为 `{"error":{message,type,param?,code?}}`；上游错误体原样透传。

**错误码总表**（网关自产）：

| HTTP | code | type | 场景 |
|---|---|---|---|
| 401 | `missing_api_key` / `invalid_api_key` / `api_key_expired` | invalid_request_error | 认证 |
| 400 | `invalid_body`、`prompt_content_conflict`、`invalid_prompt_type` | invalid_request_error | 请求解析 / L3 冲突 |
| 404 | `provider_not_found` / `model_not_found` | invalid_request_error | 模型路由 |
| 404 | `unknown_url` | invalid_request_error | 未匹配的 v1 路径 |
| 404 | `record_not_found` / `task_not_found` | invalid_request_error | 视频任务查询（越权 / 上游不存在） |
| 413 | `request_too_large` | invalid_request_error | 请求体超限 |
| 500 | `upstream_error`、`model_unresolved`、`adapter_creation_failed`、`task_query_failed`、`models_fetch_failed` | api_error | 网关侧 / 上游异常 |

**能力发现**：`GET /v1/models` → 每 model 的 `compatibility.endpoints/request/response`（契约）+ `capabilities`（自由格式标签）+ `supports_*`（能力位）。

---

## 七、遗留事项与建议

1. **images 尺寸约束**（测试报告 P3 残留）：Seedream 4.5 要求 ≥3686400 像素，`1024x1024` 被上游拒绝——目前靠透传报错；可在 P3 元数据的 `request` 段补充约束描述或做本地预校验。
2. **必填参数本地校验**（测试报告 P4 残留）：chat 缺 `messages`、messages 缺 `max_tokens` 仍透传上游原文——透传架构下的取舍，如需统一可加 handler 层预校验。
3. **`/v1/messages` 信封形状**：网关自产错误当前为 OpenAI 形状（Anthropic SDK 会 fallback 展示原始 body，不致崩）；如需严格 Anthropic 信封（`{"type":"error",...}`）可在 `respondV1Error` 加端点分支。
4. **git 提交**：本批 P1–P4 改动与其他开发线（agent 工具环 tool_calls、SKILL 管理等）混在同一工作区，建议按功能分组提交。
5. **回归建议**：P1–P4 涉及共享管线（respondV1Success/respondPassthroughErr），合并前跑一遍既有 46 用例测试报告中的正向回归（尤其流式 chat/messages 与 agent 任务）。

---

## 八、改动文件清单（P1–P4 本批）

| 文件 | P 阶段 | 内容 |
|---|---|---|
| [internal/ai/doubao_adapter.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/doubao_adapter.go) | P1 + P2 | `multimodalEmbeddings` 逐条请求合并；`extractEmbeddingVector`/`mergeUsageTokens`；`VideosGenerations` content 格式；`PassthroughRequest` 视频 prompt→content 转换 |
| [internal/api/handlers/embeddings.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/embeddings.go) | P1 | `normalizeEmbeddingsResponse`/`normalizeEmbeddingItem` 出口归一化 |
| [internal/api/handlers/videos.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/videos.go) | P2 + P4 | prompt/content L3 校验；`normalizeVideoTaskResponse` 别名；GetTask 5 处信封 |
| [internal/ai/adapter_factory.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/adapter_factory.go) | P3 | 导出 `NormalizedAdapterClass` |
| [internal/api/handlers/openai.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/openai.go) | P3 + P4 | `modelCompatibility` + ListModels 接入；ListModels 500 信封 |
| [internal/api/handlers/anthropic.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/anthropic.go) | P4 | ListModels 500 信封 |
| [internal/api/handlers/v1_pipeline.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/v1_pipeline.go) | P4 | respondV1Success 非标形状包装 |
| [internal/api/handlers/base.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/base.go) | P4 | respondPassthroughErr 兜底 code 字符串化 |
| [internal/api/middleware/auth.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/middleware/auth.go) | P4 | ApiKeyAuthMiddleware 3 处 401 信封 |
| [internal/api/middleware/api_logger.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/middleware/api_logger.go) | P4 | 413 信封 |
| [internal/api/routes.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/routes.go) | P4 | v1 前缀 NoRoute 404 信封 |
