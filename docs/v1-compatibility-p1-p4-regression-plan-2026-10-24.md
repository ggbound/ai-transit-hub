# AI Transit Hub — P1–P4 兼容性改造回归测试建议

| 项目 | 内容 |
|---|---|
| 编写日期 | 2026-10-24 |
| 依据 | [P1–P4 改造总结报告](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/docs/v1-compatibility-p1-p4-report-2026-10-24.md)、[V1 测试报告基线（46 用例）](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/docs/v1-api-test-report-2026-10-24.md) |
| 用例编号 | 既有基线用例沿用 `Txx` 编号；新增回归用例使用 `Rxx` 编号 |
| 分级 | **必测**（合并/上线前必须通过）、建议（强烈推荐）、可选（条件允许时） |

---

## 一、风险面与回归策略

P1–P4 改动的落点分两类，回归策略不同：

| 落点类型 | 位置 | 风险 | 策略 |
|---|---|---|---|
| **共享管线** | `respondV1Success` / `respondPassthroughErr` / `respondV1Error`（[v1_pipeline.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/v1_pipeline.go)、[base.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/base.go)） | 被 14 条 v1 路由共用；P4 改写了错误分支 | 全量正向回归（所有 200 路径）+ 错误信封矩阵逐项断言 |
| **类型特定路径** | embeddings / videos / models 列表 | P1/P2/P3 的行为变化 | 该类型专项用例 + 相邻类型冒烟 |

**核心风险点**（回归时重点盯）：

1. `respondV1Success` 新增的 error 对象判断分支——必须确认所有正常 2xx 响应不受影响（分支仅在 `IsErrorResponse` 命中时才进入）。
2. `respondPassthroughErr` 兜底 code 从数字改为字符串——上游错误体透传（`ue.Body != nil` 主路径）未动，仅无 body 兜底变化；重点回归上游 4xx 透传保真。
3. NoRoute 拦截——管理后台 / 非 v1 前缀的 404 行为必须保持原样。
4. P1 的多模态逐条请求——并发合并的 index 顺序、usage 累加、任一子请求失败的错误路径。
5. P2 的双向适配——`content` 原生透传必须零改动（escape hatch 不被污染）。

---

## 二、P1 · Embeddings 回归（必测）

| 用例 | 场景 | 请求要点 | 期望断言 |
|---|---|---|---|
| R01 | 单文本输入（回归 T18） | `input="你好"`，通用嵌入模型 | 200；`data` 为**数组**且长度 1；`data[0].object=="embedding"`、`data[0].index==0`；顶层 `object=="list"`、`model` 存在 |
| R02 | **数组多输入**（核心修复，回归 T19） | `input=["你好","世界"]` | 200；`data` 长度 **2**（修复前为 1）；`index` 依次 0/1 且向量维度 2048；两条向量内容不同 |
| R03 | 多模态模型数组输入（核心修复） | `model=doubao/doubao-embedding-vision-251215`，`input=["a","b","c"]` | 200；`data` 长度 3、index 顺序与输入一致（并发合并正确性）；`usage.prompt_tokens` ≈ 3 倍单条用量（累加生效） |
| R04 | 多模态模型单输入 | 同上，`input="单条"` | 200；单请求路径（不合并），`data` 长度 1 |
| R05 | 上游错误透传（回归 T20） | `input=""`（空串） | 400；**上游 MissingParameter 原文透传**（P4 未改透传保真主路径的证明） |
| R06（建议） | 较大数组 | `input` 10 条 | 200；10 条向量、index 0–9 无乱序（并发上限 8 下的合并稳定性） |
| R07（建议） | 记账 | R02 执行后查 `usage_logs` | 1 条成功记录，token 计入正常 |

---

## 三、P2 · Videos 回归（必测）

| 用例 | 场景 | 请求要点 | 期望断言 |
|---|---|---|---|
| R08 | **prompt-only 创建**（核心修复，回归 T23） | `POST /v1/videos/generations/tasks`，仅 `{"model","prompt":"一只猫在草地上奔跑"}` | 200；返回任务 ID（未转换时 Ark 因缺 content 拒绝 → 200 即证明适配生效） |
| R09 | content-only 透传（回归 T23b） | 仅 `{"model","content":[{"type":"text","text":"..."}]}` | 200；请求体除 prompt/content 逻辑外**零改动**透传。**已执行（2026-10-24）**：200 + id，响应与 R08 同构，闭环 succeeded（108900 tokens / ¥5.0094，与 R08 精确同价）；费用回填发生在带 record_id 的查询路径 |
| R10 | prompt+content 冲突（L3） | 两者并存 | 400；信封 `code=="prompt_content_conflict"`、`type=="invalid_request_error"`；**无 usage 记录**（校验在 prepareV1Request 之前） |
| R11 | prompt 类型错误 | `prompt: 123` | 400；`code=="invalid_prompt_type"` |
| R12 | prompt-only 任务完整闭环 | R08 创建的任务轮询至终态 | `status=="succeeded"`；顶层 `video_url` 与 `content.video_url` **同时存在且一致**（别名不覆盖原字段） |
| R13 | running 任务查询 | 创建后立即查询 | 200；响应无 `content` 时归一化安全跳过，顶层无多余的 `video_url:null` |
| R14 | 任务不存在（回归 T24） | `GET .../tasks/cgt-nonexistent` | 404；信封 `code=="task_not_found"`（P4 后由 status/error 格式变为标准信封——注意此用例断言的是**新信封**） |
| R15（建议） | 记账 | R08/R09 后查 `usage_logs` | 仅成功创建各 1 条记录 |

---

## 四、P3 · 能力元数据回归（必测）

| 用例 | 场景 | 请求要点 | 期望断言 |
|---|---|---|---|
| R16 | 全量模型元数据完整性 | `GET /v1/models` | 每个 model 均含 `compatibility`，`endpoints`/`request` 恒有；`response` 仅声明于网关有响应保证的类型（embedding/video），text/image 无该段属设计；**原有字段（capabilities、supports_*、价格）一个不少** |
| R17 | text 类动态端点 | 抽查豆包 text 与 ZhiyunTB text | 豆包系：有 `chat`、无 `messages`（SupportsMessages=false）；ZhiyunTB：含 `responses` 端点 |
| R18 | 多模态嵌入判定（核心） | 模型 14 `doubao-embedding-vision-251215` | `request.input` 含 `"multimodal_content[]"`；通用嵌入模型（如 doubao-embedding-text）不含 |
| R19 | video 类契约 | 任一豆包视频模型 | `endpoints.create/query` 存在；`exclusive_fields==["prompt","content"]`；`response.video_url=="top_level_alias"` |
| R20 | Anthropic 列表不受影响（回归 T07） | `GET /v1/anthropic/models` | 保持 Anthropic 极简格式，**无** compatibility 块 |
| R21（建议） | 缓存一致性 | 60s 内二次请求 | 两次 compatibility 输出一致（缓存序列化无损） |

---

## 五、P4 · 错误信封矩阵回归（必测）

**统一断言模板**：每个错误响应 body 满足 `error` 为**对象**，含非空 `message` 与 `type`，`code` 为字符串；HTTP 状态与语义匹配。

| 用例 | 场景 | 请求要点 | 期望 |
|---|---|---|---|
| R22 | 401 三连（回归 T01–T04） | 无认证头 / 坏 Bearer key / 坏 X-Api-Key | 401；`code` 分别为 `missing_api_key` / `invalid_api_key` / `invalid_api_key`；错误对象信封（修复前为 error 字符串） |
| R23 | 未知 URL | `GET /v1/nonexistent` 及 `POST /v1/chat/completions/extra` | 404；`code=="unknown_url"`、message 含 `Invalid URL (METHOD /path)` |
| R24 | **非 v1 前缀 404 不变** | `GET /nonexistent`（或管理后台路由） | 404；body 为 `{"error":"404 page not found"}`（gin 默认行为保持） |
| R25 | 透传保真（回归 T11/T16） | chat 缺 `messages`、messages 缺 `max_tokens` | chat 必 400 且**上游错误原文原样透传**（Ark 格式，未被包装/改写）；messages 缺 `max_tokens` 双态接受——强制型上游 400→Anthropic error 信封，宽松型上游（如 ZhiyunTB deepseek）200→合法 message（透传原则下网关不拦截） |
| R26 | 模型路由（回归 T10/T43） | `nope/none`、不存在的 provider | 404；`provider_not_found` / `model_not_found`（回归确认信封未变） |
| R27 | 413 | 构造 >10MB body（api_logger maxRequestBodySize = 10MB） | 413；`code=="request_too_large"` |
| R28 | 双前缀一致性 | 上述核心错误用例在 `/api/v1/*` 重跑（至少 R22/R23/R26） | 信封与 `/v1` 完全一致 |
| R29（可选） | 过期 key | 需造一条 `expires_at` 已过期的 key | 401；`code=="api_key_expired"`。**已执行（2026-10-24）**：401 + `api_key_expired`，零记账记录，造数 key 已删除 |
| R30（可选） | 越权 record 查询 | 用 B 账号 key 查 A 账号 record | 404；`code=="record_not_found"`。**已执行（2026-10-24）**：404 + `record_not_found` + `param=record_id`，零记账记录（注意：record_id 必须真实存在，不存在时 fall through 到默认模型） |

**代码走查覆盖**（难以稳定触发，不进自动化）：

- `models_fetch_failed`（模型缓存查询失败）、`model_unresolved` / `adapter_creation_failed`（依赖 DB 数据缺失）、`task_query_failed`（上游 5xx）——走查 [videos.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/videos.go#L93-L187) 分支与 [openai.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/openai.go) 确认 param/code 传参正确。
- `respondV1Success` 的"error 为对象 → 原样透传"分支——确认包装分支不会误伤已是标准信封的上游错误。

---

## 六、共享管线正向回归（必测，P4 重点防守）

P4 动了所有 200 路径共用的响应函数，**改造前的正向用例必须全量重跑**：

| 用例 | 基线编号 | 期望 |
|---|---|---|
| R31 | T08 | chat 非流式 200，`choices[].message` / `usage` 完整 |
| R32 | T09 | chat 流式 SSE，增量块正常结束（`data: [DONE]`） |
| R33 | T13/T14 | `/v1/messages` 非流式（Bearer 与 X-Api-Key 各一），Anthropic 格式完整 |
| R34 | T15 | messages 流式事件序列完整 |
| R35 | T26/T28 | responses 非流式 + 流式 |
| R36 | T21b | images 成功返回 `data[].url` |
| R37 | T32/T33/T37 | agent 创建 → 轮询 → completed + 资产下载。**已执行（2026-10-24）**：202 + `agt_` task_id，13s 即 completed（quick-note 技能，1 轮迭代），note.md 下载 200 且 826 字节与 assets 元数据一致；usage ¥0.002754 / 5319 tokens 对账一致 |
| R38 | T41 + 备用前缀 | X-Api-Key 认证、`/api/v1/models` 200 |
| R39 | T42 | 记账对账：上述成功请求各产生 usage 记录（status=1）；R10/R11 等**网关层拦截**（校验 400/401/404/413）**不产生**记录；上游返回的 400（如 chat 缺 messages）产生 status=0、cost=0 的**零费用审计记录**（与 PHP createInitialRecords 先建后更行为一致，不产生费用） |
| R40（建议） | T05 | `/v1/models` 200 且 P3 元数据与 R16 断言一致（一条用例同时守 P3+管线） |

---

## 七、执行方式建议

1. **顺序**：先跑 §六（正向防守）→ §五（错误矩阵）→ §二/§三/§四（类型专项）。正向失败时先修管线再继续，避免误判。
2. **脚本化**：P0 用例（R01–R05、R08–R14、R16–R19、R22–R28、R31–R39）适合合并为一个 python 断言脚本（模式与本次验证一致：`urllib` 发请求 + json 断言 + 结果表输出）；R12 需轮询等待（seedance-2-0 实测约 5 分钟，轮询上限建议 ≥360s）放最后。
3. **测试数据**：真实激活 key 1 枚（用后清理）；R29/R30 需额外构造数据，建议放到最后单独执行。
4. **环境**：与本次改造验证一致——`backend` 目录启动（加载 .env）、端口 8080、真实上游（豆包 Ark）。**不使用 mock 上游跑信封矩阵**（透传保真断言依赖真实上游错误原文）。
5. **验收标准**：必测用例 100% 通过；建议用例允许记录后跟进；任何信封形状回归（error 非对象 / code 缺失 / 上游原文被改写）视为阻塞。

---

## 八、示例断言（关键用例）

```python
# R02：数组输入返回 N 条向量（P1 核心修复点）
st, body = req("POST", "/v1/embeddings", {"model": MODEL, "input": ["你好", "世界"]})
assert st == 200
assert isinstance(body["data"], list) and len(body["data"]) == 2
assert [d["index"] for d in body["data"]] == [0, 1]
assert all(d["object"] == "embedding" for d in body["data"])
assert body["object"] == "list"

# R23：未知 URL 信封（P4）
st, body = req("GET", "/v1/nonexistent")
assert st == 404
assert body["error"]["code"] == "unknown_url"
assert "/v1/nonexistent" in body["error"]["message"]

# R12：响应别名不覆盖原字段（P2）
# 任务 succeeded 后：
assert body["video_url"] == body["content"]["video_url"]   # 别名与嵌套一致
```
