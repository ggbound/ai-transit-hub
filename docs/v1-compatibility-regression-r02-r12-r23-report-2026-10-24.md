# v1 兼容性改造回归测试报告（R02 / R23 / R12）

- **执行日期**：2026-10-24
- **测试对象**：AI Transit Hub v1 OpenAI 兼容层（`/v1` 与 `/api/v1` 双前缀）
- **对应改造**：P1 embeddings 数组输入修复（R02）、P2 视频响应别名与闭环（R12）、P4 错误信封统一（R23）
- **测试脚本**：[scripts/regression_r02_r12_r23.py](../scripts/regression_r02_r12_r23.py)
- **用例定义来源**：[回归测试建议](v1-compatibility-p1-p4-regression-plan-2026-10-24.md)
- **总体结论**：**3/3 用例通过**（R02 断言 8/8、R23 断言 7/7、R12 断言全部满足）。过程中发现脚本 R12 默认轮询上限不足（150s < seedance-2-0 实际生成时长 ~5 分钟），已修正为 360s。

---

## 一、执行环境

| 项 | 值 |
|---|---|
| 服务 | `/tmp/ath-server`（backend `./cmd/server` 编译产物），P1-P4 改造后版本 |
| 地址 | `http://127.0.0.1:8080`（执行前 `/health` 返回 200） |
| 测试 Key | ServBay MySQL `ai_transit_hub.user_api_keys`（status=1，用后即删） |
| R02 模型 | `doubao/doubao-embedding-vision-251215`（多模态嵌入，2048 维） |
| R12 模型 | `doubao/doubao-seedance-2-0-260128`（视频生成，真实计费） |
| Python | 系统 `python3`（urllib，无第三方依赖） |
| 脚本退出码约定 | 0 = 全部通过；1 = 存在失败断言 |

## 二、执行时间线

| 轮次 | 命令 | 结果 |
|---|---|---|
| 第 1 轮 | `python3 scripts/regression_r02_r12_r23.py --key <KEY>`（默认，R12 跳过） | R02 8/8、R23 7/7，退出码 0 |
| 第 2 轮 | 同上 + `--include-video` | R02 8/8、R23 7/7；R12 脚本判 FAIL（150s 轮询上限内未达终态），退出码 1 |
| 第 3 轮 | 对第 2 轮已有任务**延长轮询**（不重复创建计费任务） | 任务 `succeeded`，R12 全部断言人工确认满足 |
| 修正 | 脚本 `--poll-timeout` 默认 150 → 360，帮助文本同步 | `ast.parse` 语法检查通过 |

> 第 2 轮同时复跑了 R02/R23（每次脚本运行均完整执行三个用例的门控集合），结果与第 1 轮一致，可视为重复稳定性验证。

---

## 三、R02 · embeddings 数组输入返回 N 条向量（P1 核心修复）

### 用例定义

`POST /v1/embeddings`，`input=["你好","世界"]`。修复前豆包多模态端点把数组语义当作"多项内容拼 1 条向量"，仅返回 1 条；修复后逐条并发请求并按 index 合并，应返回 2 条。

### 请求

```json
{
  "model": "doubao/doubao-embedding-vision-251215",
  "input": ["你好", "世界"]
}
```

### 断言结果（8/8 通过）

| # | 断言 | 结果 | 实际值 |
|---|---|---|---|
| 1 | HTTP 状态 200 | PASS | 200 |
| 2 | 顶层 `object == "list"` | PASS | `"list"` |
| 3 | `data` 为数组 | PASS | list |
| 4 | `data` 长度 == 2（**修复前为 1**） | PASS | 2 |
| 5 | `index == [0, 1]` 且与输入顺序一致 | PASS | `[0, 1]` |
| 6 | 每项 `object == "embedding"` | PASS | — |
| 7 | 每项含非空向量 | PASS | 维度 `[2048, 2048]` |
| 8 | 两条向量内容不同（确实按输入拆分） | PASS | — |

### 结论

数组输入 → N 条向量契约完全恢复；index 顺序、object 字段、向量内容区分度全部符合 OpenAI 语义。

---

## 四、R23 · 未知 v1 URL 返回错误信封 404（P4）

### 用例定义

v1 前缀下未匹配路由应返回 OpenAI 风格对象信封（修复前为 gin 默认纯文本 `404 page not found`）。双场景：GET 未知路径 + POST 已知路径带多余尾段。

### 断言结果（7/7 通过）

| # | 断言 | 结果 | 实际值 |
|---|---|---|---|
| 1 | `GET /v1/nonexistent` → 404 | PASS | 404 |
| 2 | `error` 为对象信封（**修复前为字符串/纯文本**） | PASS | — |
| 3 | `error.code == "unknown_url"` | PASS | `"unknown_url"` |
| 4 | `error.type == "invalid_request_error"` | PASS | `"invalid_request_error"` |
| 5 | `error.message` 含请求方法与路径 | PASS | `"Invalid URL (GET /v1/nonexistent)"` |
| 6 | `POST /v1/chat/completions/extra` → 404 | PASS | 404 |
| 7 | 变体同为 unknown_url 信封 | PASS | — |

### 实际响应示例

```json
{
  "error": {
    "code": "unknown_url",
    "message": "Invalid URL (GET /v1/nonexistent)",
    "type": "invalid_request_error"
  }
}
```

### 结论

v1 未匹配路由统一 OpenAI 信封；非 v1 前缀（管理后台等）404 行为保持原样，未受影响（此点为回归计划 R24 防守项，本次未复测）。

---

## 五、R12 · 视频 prompt-only 闭环 + 响应别名不覆盖原字段（P2）

### 用例定义

仅传 `prompt`（不传 content）创建视频任务 → 轮询至终态 → `status=="succeeded"`，顶层 `video_url` 与 `content.video_url` **同时存在且一致**（别名写入不得覆盖原字段）。

### 执行过程

| 阶段 | 内容 |
|---|---|
| 创建 | prompt-only 请求 → 200，返回任务 ID `cgt-20261024205424-p2d8t`（2026-10-24 20:54:24 创建） |
| 脚本轮询 | 5s 间隔 × 30 次 = 150s，全部 `running` → 脚本判 FAIL（轮询上限不足，见第六节） |
| 延长轮询 | 对同一任务以 10s 间隔继续轮询，第 14 次（约 +140s）到达 `succeeded` |
| 总耗时 | `created_at=1792846464` → `updated_at=1792846771`，**307 秒 ≈ 5 分 07 秒** |

### 终态响应（关键字段）

```json
{
  "id": "cgt-20261024205424-p2d8t",
  "model": "doubao-seedance-2-0-260128",
  "status": "succeeded",
  "video_url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-0/0217....mp4?X-Tos-Signature=...",
  "content": {
    "video_url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-0/0217....mp4?X-Tos-Signature=..."
  },
  "duration": 5,
  "ratio": "16:9",
  "resolution": "720p",
  "framespersecond": 24,
  "generate_audio": true,
  "output_format": "mp4",
  "seed": 40566,
  "service_tier": "default",
  "execution_expires_after": 172800,
  "usage": { "completion_tokens": 108900, "total_tokens": 108900 }
}
```

> `video_url` 为带签名的 TOS 临时地址（X-Tos-Expires=86400），上表截断展示；两处 URL 逐字符一致。

### 断言结果（全部满足）

| # | 断言 | 结果 | 实际值 |
|---|---|---|---|
| 1 | prompt-only 创建 → 200 | PASS | 200，`{"id": "cgt-...p2d8t"}` |
| 2 | 返回任务 ID | PASS | `cgt-20261024205424-p2d8t` |
| 3 | 终态 `status == "succeeded"` | PASS | `succeeded`（延长轮询确认） |
| 4 | 顶层 `video_url` 存在 | PASS | 签名 TOS URL |
| 5 | `content.video_url` 存在（别名不覆盖原字段） | PASS | 同上 |
| 6 | 两处 URL 一致 | PASS | 逐字符一致 |
| 7 | 附带元数据完整可用 | PASS | duration/ratio/resolution/fps/usage 均在 |

### 结论

prompt-only → 任务创建 → 轮询闭环全链路可用；顶层字段与 `content` 别名同时存在且一致，别名写入逻辑未破坏原字段结构。**判定通过**（判定依据为延长轮询人工确认，脚本内该轮判 FAIL 的原因见下节，属脚本参数问题而非网关行为问题）。

---

## 六、发现的问题与修正

### 问题：R12 脚本默认轮询上限不足

- **现象**：第 2 轮脚本内 R12 在 150s（30 次 × 5s）内未达终态，判 FAIL，退出码 1；任务实际继续运行至 ~307s 才 `succeeded`。
- **根因**：回归计划撰写时按 seedance 常规生成时长估计 "~45s"，seedance-2-0 实测显著更长（本次 5 分 07 秒，720p/5s/含音频）。
- **影响面**：仅脚本参数，网关行为无问题。
- **修正**（[scripts/regression_r02_r12_r23.py](../scripts/regression_r02_r12_r23.py)）：
  - `--poll-timeout` 默认 150 → **360**
  - `--include-video` 与 docstring 帮助文本同步修正为 "seedance-2-0 实测约 5 分钟"
  - `ast.parse` 语法检查通过
- **未重跑说明**：修正后未再执行脚本级 R12 复跑，避免二次视频计费；终态断言已通过延长轮询确认。后续例行回归可直接使用新默认值。

### 顺带确认

- 脚本退出码语义符合设计：全通过 → 0；有失败 → 1（可挂 CI）。
- 第 2 轮 R02/R23 与第 1 轮结果完全一致，重复执行稳定性良好。
- 查询任务时误用 `GET /v1/videos/generations/{id}` 正确暴露出 unknown_url 信封（路径实为 `/v1/videos/generations/tasks/{taskId}`），反向验证了 R23 信封在真实误用场景下的表现。

---

## 七、汇总

| 用例 | 覆盖改造 | 断言 | 结果 | 成本 |
|---|---|---|---|---|
| R02 | P1 embeddings 数组输入 | 8/8 | **通过** | 少量 tokens |
| R23 | P4 未知 URL 错误信封 | 7/7 | **通过** | 免费 |
| R12 | P2 视频闭环 + 别名不覆盖 | 7/7 | **通过** | 1 次视频生成（108,900 tokens 计费） |

**整体判定：R02 / R23 / R12 全部通过。**

## 八、遗留与建议

1. **其余回归用例**（R01-R40 中的其余项）未在本轮执行，见[回归测试建议](v1-compatibility-p1-p4-regression-plan-2026-10-24.md)；其中建议级用例（R06/R07/R15/R21/R29/R30 等）可按需跟进。
2. **R12 轮询间隔建议**：脚本 5s 间隔对 5 分钟级任务偏密，可考虑改为 "前 30s 间隔 3s、之后间隔 15s" 的阶梯策略，减少无谓请求（可选优化）。
3. **回归计划文档更新**：建议将回归计划中 R12 的 "~45s" 时长估计修正为 "~5 分钟（seedance-2-0 实测）"，与脚本修正保持一致。
4. **URL 记忆点**：视频任务查询路径为 `/v1/videos/generations/tasks/:taskId`（带 `tasks` 段），易与创建路径 `/v1/videos/generations` 混淆——R23 信封对误用场景已给出可诊断的错误信息。
