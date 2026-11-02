# R12 阶梯轮询优化测试报告

- **日期**：2026-10-24
- **范围**：R12 视频任务轮询的阶梯化改造（方案 A：三段式 5s/15s/30s）的实施与三步验证
- **关联文档**：
  - 实施方案：[r12-polling-optimization-proposal-2026-10-24.md](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/docs/r12-polling-optimization-proposal-2026-10-24.md)
  - 上游回归报告：[v1-compatibility-regression-full-report-r01-r40-2026-10-24.md](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/docs/v1-compatibility-regression-full-report-r01-r40-2026-10-24.md)
- **改造对象**：[scripts/regression_full_suite.py](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/scripts/regression_full_suite.py)（仅测试套件，网关与数据库零改动）

---

## 1. 执行摘要

按方案文档完成 R12 阶梯轮询优化的**全部实施与验证**，三步验证（静态 → 冒烟 → 真实闭环）一次通过，无回滚事项。

| 项目 | 结果 |
|---|---|
| Patch 实施 | 4 处修改，全部落地于套件脚本 |
| 静态验证 | py_compile 通过；满窗 420s = 24 次唤醒（快速 6/稳定 8/长尾 10）；300s 闭环 = 19 次，与方案核算一致 |
| 快速失败冒烟 | 不存在任务首次查询即 break：poll#1、wall 0.23s、零 sleep、`task_not_found` |
| 真实闭环 | 套件 35 计数：**34 通过 / 0 失败 / 1 跳过（R36 未开启）**，SUITE_EXIT=0；R12 全断言 PASS |
| R12 实测 | 闭环 122s、13 次轮询，日志时刻表完整呈现 5s→15s→30s 阶梯 |
| 记账对账 | record_id=447 回填后 usage 439 = **108900 tokens / ¥5.0094**，与 R08/R09 历史精确同价（第三次计费确定性确认） |
| 本轮总成本 | 免费段 ¥0.000382 + 视频 ¥5.0094 ≈ **¥5.01** |

---

## 2. 背景与目标

R12 原实现为固定 10s 间隔轮询（`--poll-timeout` 默认 360s），存在三个问题：

1. **快速失败不敏感**：0–30s 内的上游秒级失败要等下一次 10s 轮询才发现；
2. **稳定期查询冗余**：running 状态下每 10s 一查，300s 闭环约 31 次轮询（每次轮询 = 网关转发 Ark 上游查询 1 次），对上游产生无谓压力；
3. **超时余量偏紧**：seedance-2-0 历史闭环 ≈300s，360s 上限余量仅 1.2×。

按方案选定**方案 A（三段式阶梯）**，目标：

- 300s 闭环轮询次数 31 → 19（**-39%**）；
- 0–30s 窗口内失败检测粒度 10s → 5s；
- 超时余量 1.2× → 1.4×（默认 420s）；
- 代价：终态滞后上限由 ≤10s 放宽到 ≤30s（设计接受，不影响任何断言）。

---

## 3. 实施内容（Patch 明细）

全部修改位于 [scripts/regression_full_suite.py](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/scripts/regression_full_suite.py)，共 4 处：

| # | 位置 | 内容 |
|---|---|---|
| 1 | [L738-741](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/scripts/regression_full_suite.py#L738-L741) | 新增三段阶梯模块级常量 |
| 2 | [L671-696](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/scripts/regression_full_suite.py#L671-L696) | `section_f_video_paid` 内 R12 轮询循环替换为按已耗时分段 sleep |
| 3 | [L754](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/scripts/regression_full_suite.py#L754) | `--poll-timeout` 默认值 360 → 420 |
| 4 | [L18-19](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/scripts/regression_full_suite.py#L18-L19) | 文件头注释同步（阶梯 5/15/30s，上限默认 420s） |

### 3.1 阶梯常量（Edit 1）

```python
# --- R12 阶梯轮询参数：快速失败段 → running 稳定期 → 长尾/完成窗口 ---
VIDEO_POLL_FAST_S, VIDEO_POLL_FAST_UNTIL = 5, 30    # 0-30s：秒级失败尽早发现
VIDEO_POLL_MID_S,  VIDEO_POLL_MID_UNTIL  = 15, 150  # 30-150s：running 稳定期
VIDEO_POLL_SLOW_S                        = 30       # 150s+：长尾与期望完成窗口
```

### 3.2 R12 阶梯轮询循环（Edit 2，替换原固定 10s 循环）

```python
# R12 闭环 + 别名（阶梯轮询 5/15/30s，方案见 docs/r12-polling-optimization-proposal-2026-10-24.md）
c = Case("R12", "轮询至终态 + 别名不覆盖原字段（真实生成，最长 %ds，阶梯 5/15/30s）" % poll_timeout)
started = time.time()
deadline = started + poll_timeout
final = None
polls = 0
while time.time() < deadline:
    st, b = req(base, "/v1/videos/generations/tasks/%s" % task_id, key, timeout=60)
    polls += 1
    status = b.get("status") if isinstance(b, dict) else "?"
    print("    [%3ds] status=%s" % (time.time() - started, status))
    if status not in ("running", "queued", "submitted"):
        final = b
        break
    elapsed = time.time() - started
    if elapsed < VIDEO_POLL_FAST_UNTIL:
        time.sleep(VIDEO_POLL_FAST_S)
    elif elapsed < VIDEO_POLL_MID_UNTIL:
        time.sleep(VIDEO_POLL_MID_S)
    else:
        time.sleep(VIDEO_POLL_SLOW_S)
c.check("在时限内到达终态", final is not None,
        "超时（%ds），建议增大 --poll-timeout；seedance-2-0 实测约 5 分钟" % poll_timeout)
if final is not None:
    c.note("闭环耗时=%ds 轮询次数=%d 终态滞后<=%ds" % (
        time.time() - started, polls, VIDEO_POLL_SLOW_S))
```

关键设计点：

- **先查询后判断再 sleep**：终态（非 running/queued/submitted）在 sleep 前即 break，保证快速失败路径零等待；
- **分段判据用已耗时**（`elapsed`），而非轮询计数，天然适应网络抖动；
- **超时提示带决策信息**：提示增大 `--poll-timeout` 并注明历史均值；
- **note 记录三个关键观测量**：闭环耗时、轮询次数、终态滞后上限，直接落入套件报告输出。

后续 succeeded 断言块（顶层 video_url、content.video_url、两处一致、duration/ratio/resolution/fps/usage）未改动。

---

## 4. 验证过程与结果（三步验证法）

### 4.1 步骤一：静态时刻表核对（零成本）

- `python3 -m py_compile scripts/regression_full_suite.py` → **SYNTAX_OK**；
- 以纯数学推演轮询时刻表（查询即时、忽略网络抖动）：

| 阶段 | 时间窗 | 间隔 | 唤醒时刻 | 次数 |
|---|---|---|---|---|
| 快速失败段 | 0–30s | 5s | 0, 5, 10, 15, 20, 25 | 6 |
| running 稳定期 | 30–150s | 15s | 30, 45, 60, 75, 90, 105, 120, 135 | 8 |
| 长尾/完成窗口 | 150s+ | 30s | 150, 180, 210, 240, 270, 300, 330, 360, 390, (420 边界) | 10 |
| **合计（满窗 420s）** | | | | **24** |

- **300s 历史闭环场景**：6 + 8 + 5（150/180/210/240/270）= **19 次**，与方案核算值一致（原固定 10s 为 31 次，-39%）。

### 4.2 步骤二：快速失败路径冒烟（零成本）

- 方法：临时脚本导入套件模块（复用 `VIDEO_POLL_*` 常量与 `req()`），以不存在的 `task_id='cgt-nonexistent-smoke'` 复刻 R12 循环，验证改造后循环的早退语义；
- 结果（**SMOKE_OK**）：

| 观测项 | 期望 | 实测 |
|---|---|---|
| break 发生时机 | 首次查询即终态 break | **poll#1** |
| wall 耗时 | 零 sleep | **0.23s** |
| 错误渲染 | 404 `task_not_found` | ✅ |
| note 输出格式 | 正常 | ✅ |

- 结论：重构未破坏「终态先于 sleep 判断」的快速退出语义；临时脚本验证后已删除。

### 4.3 步骤三：真实闭环验证（--include-video，真实计费）

执行 `python3 scripts/regression_full_suite.py --include-video`（真实创建 seedance-2-0 视频）。

**套件整体**：35 计数，**34 通过 / 0 失败 / 1 跳过（R36，--include-image 未开启）**，SUITE_EXIT=0。其中 A–E 段 31 项全绿，F 计费段 R08/R13/R12 三用例全 PASS。

**R12 阶梯时刻表实测**（套件日志逐条打印，task_id=`cgt-20261024221845-4vjzq`，22:18:45 创建）：

| 阶段 | 实测轮询时刻（秒） | 次数 | 与设计对照 |
|---|---|---|---|
| 快速段（5s） | 0, 5, 10, 15, 20, 26, 31 | 7 | 20→26 的 1s 偏移为请求耗时代入 elapsed 的自然抖动（设计允许） |
| 稳定期（15s） | 46, 61, 76, 91, 107 | 5 | 间隔严格 15s |
| 终态 | **122**（succeeded，break） | 1 | 终态在稳定期 15s 节奏内即时捕获，未进入长尾段 |
| **合计** | | **13** | |

- R12 note：`闭环耗时=122s 轮询次数=13 终态滞后<=30s`；
- 视频规格断言：720p / 16:9 / 5s / 24fps；
- 说明：本轮上游快于历史均值（122s vs ≈300s），13 次轮询与固定 10s 在该时长下相当；阶梯的轮询节省主要兑现于稳定期与长尾段（>30s 场景，静态核算 300s 闭环 31→19 次）。

---

## 5. R12 断言明细

| 断言 | 结果 | 证据 |
|---|---|---|
| 在时限内到达终态（< 420s） | PASS | 闭环 122s |
| 终态 status == succeeded | PASS | 日志 [122s] status=succeeded |
| 顶层 `video_url` 存在 | PASS | 响应含直链 |
| `content.video_url` 存在 | PASS | content 段含直链 |
| 两处 video_url 一致 | PASS | 逐字符比对 |
| duration = 5s | PASS | note 记录 |
| ratio = 16:9 | PASS | note 记录 |
| resolution = 720p | PASS | note 记录 |
| fps = 24 | PASS | note 记录 |
| usage 上报（completion/total = 108900） | PASS | `usage={"completion_tokens": 108900, "total_tokens": 108900}` |

---

## 6. 记账对账

### 6.1 增量基线与套件落账

验证前基线：usage_logs max=**423**、api_test_requests max=**431**。套件运行后：

| 空间 | 新增区间 | 内容 |
|---|---|---|
| usage_logs | 424–438（15 行） | 免费段（embeddings/messages 等），其中 431、437 为 status=0 零费用审计行；合计 ¥0.000382 |
| usage_logs | 439 | 视频创建行（status=1、0 tokens、0 cost，22:18:45）——费用待 record_id 查询回填（已知设计） |
| api_test_requests | 432–447（16 行） | 与 usage 424–439 一一对应（两空间 id 不同步，存在偏移） |

### 6.2 费用回填（record_id 路径）

回填路径依据 [backend/internal/api/handlers/videos.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/api/handlers/videos.go)：

1. 查得视频创建行：`api_test_requests.id=447`（api_type=videos，usage_log_id=439）；
2. 带回填参数查询：
   `GET /v1/videos/generations/tasks/cgt-20261024221845-4vjzq?record_id=447` → 返回 `succeeded` + usage 108900；
3. 复查 usage_logs 439：

| 字段 | 回填前 | 回填后 |
|---|---|---|
| status | 1 | 1 |
| completion_tokens | 0 | **108900** |
| total_tokens | 0 | **108900** |
| cost | 0.000000 | **5.009400** |
| request_time | 22:18:45 | 22:22:44（回填更新时刻） |

### 6.3 计费确定性与不计费语义

- **¥5.0094 与 R08/R09 历史精确同价**——seedance-2-0 同参数（720p/16:9/5s/24fps）第三次计费确定性确认；
- **轮询不计费语义再次验证**：本轮 13 次轮询 GET + 1 次回填 GET，usage_logs 终态 max 仍为 439（回填为原行 update，非新增），GET 查询路径零新增计费记录；
- 本轮总成本：免费段 ¥0.000382 + 视频 ¥5.0094 = **¥5.009782 ≈ ¥5.01**。

---

## 7. 收益核算：方案预期 vs 实测

| 指标 | 改造前（固定 10s） | 方案预期 | 实测 |
|---|---|---|---|
| 300s 闭环轮询次数 | 31 | 19（-39%） | **19**（静态推演核对一致） |
| 快速失败检测粒度（0–30s） | ≤10s | ≤5s | ≤5s（本轮实测段间隔 5s，抖动 +1s） |
| 超时余量（对 ≈300s 闭环） | 1.2×（360s） | 1.4×（420s） | 420s 默认已生效 |
| 终态滞后上限 | ≤10s | ≤30s（接受） | ≤30s（本轮终态即时捕获，未触上限） |
| 快速失败路径等待 | 终态需先遇一次 sleep 检查 | 终态即 break | **poll#1 / 0.23s / 零 sleep** |

如实说明：本轮实际闭环 122s（上游快于历史 ≈300s），该时长下阶梯与固定 10s 的轮询次数相当；**阶梯收益主要兑现于 >30s 的稳定期与长尾场景**（如 300s 闭环的 -39%），且以 1 处常量可调的三段参数保留了向上游波动适配的空间。

---

## 8. 风险评估与回滚状态

| 风险项（方案识别） | 验证结论 |
|---|---|
| 终态滞后放宽至 ≤30s 影响断言 | 不影响：R12 断言只要求时限内到达终态，未对滞后设阈；本轮终态即时捕获 |
| 上游波动（更快/更慢） | 更快：本轮 122s 即时捕获；更慢：420s 上限 + 超时提示含决策信息 |
| 快速失败语义被重构破坏 | 冒烟排除：poll#1 即 break、零 sleep |
| 记账完整性 | 对账闭合：447→439 回填精确同价 |

**回滚状态**：未触发。如需回滚，仅还原套件脚本 4 处编辑（常量、循环、argparse 默认值、头注释）即可，网关与数据库无改动。

---

## 9. 遗留事项

1. **git 分组提交**（未要求）：P1–P4 兼容层代码、回归文档、本次阶梯改动建议分组提交；
2. **费用回填不纳入套件**（方案 §范围外，维持）：回填依赖 record_id 查询路径，套件保持免费回填外的零侵入，记账以 SQL 对账为准；
3. **R36（--include-image）维持默认跳过**：历史决策不变，本轮 1 个跳过项即来源于此。

---

## 10. 附录

### 10.1 测试环境

| 项 | 值 |
|---|---|
| 网关 | AI Transit Hub，Go(Gin)+PHP，127.0.0.1:8080 |
| 套件 | scripts/regression_full_suite.py（Python 3，六段式） |
| 数据库 | MySQL（ServBay），ai_transit_hub |
| 视频模型 | doubao/doubao-seedance-2-0-260128 |
| 测试密钥 | A key（user_id=1），本报告脱敏为 `sk_hrIH****` |

### 10.2 关键命令记录

```bash
# 静态验证
python3 -m py_compile scripts/regression_full_suite.py

# 真实闭环（计费）
python3 scripts/regression_full_suite.py --include-video

# 费用回填（record_id 路径）
curl -s "http://127.0.0.1:8080/v1/videos/generations/tasks/cgt-20261024221845-4vjzq?record_id=447" \
  -H "Authorization: Bearer $ATH_KEY"
```

### 10.3 产物与清理

- 交付物：本报告、实施方案文档、R01-R40 全量回归报告（均存 docs/）；
- 已清理：/tmp/ath_ladder_verify.log（闭环日志，取证后删除）、/tmp/ath_r12_smoke.py（冒烟脚本，验证后删除）。
