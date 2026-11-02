# R12 阶梯轮询优化方案

| 项目 | 内容 |
|---|---|
| 编写日期 | 2026-10-24 |
| 级别 | 建议级优化（回归报告遗留事项，非阻塞） |
| 改动范围 | 仅 [scripts/regression_full_suite.py](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/scripts/regression_full_suite.py) R12 轮询段（L671–698）+ `--poll-timeout` 默认值 |
| 预期收益 | 单次视频闭环的上游查询数 **31 → 19（-39%）**，秒级失败发现 10s → 5s，闭环耗时数据落报告 |

---

## 一、现状与实测依据

### 1.1 现有实现（L671–698）

R12 以**固定 10s 间隔**轮询 `GET /v1/videos/generations/tasks/{task_id}`，上限 `--poll-timeout`（默认 360s）：

```python
deadline = time.time() + poll_timeout
while time.time() < deadline:
    st, b = req(base, "/v1/videos/generations/tasks/%s" % task_id, key, timeout=60)
    ...
    time.sleep(10)   # 固定间隔
```

### 1.2 实测时序数据（回归全程取证）

| 事实 | 来源 |
|---|---|
| seedance-2-0 视频闭环 ≈ **300s**（两次独立确认，费用精确同价 ¥5.0094） | 计费轮 R08、造数轮 R09 |
| 创建后立即可查到进行中状态（queued/running/submitted） | R13 断言 |
| 终态前全程无状态变化，终态一旦生成即查即得 | R12 两轮轮询日志 |
| **轮询查询本身不计费**（GET 不产生 usage 记录，40+ 次轮询零增量） | R09 记账对账（usage 仅 1 条创建记录） |
| 每次轮询 = 网关向 Ark 上游转发 1 次查询，并在 `api_test_requests` 留 1 行日志 | 套件轮次日志与 DB 审计 |

### 1.3 现状问题

1. **均匀开销**：running 稳定期（30s–150s，实测无状态变化）与期望完成窗口用同样密度轮询，300s 闭环约打 31 次上游查询、留 31 行日志。
2. **秒级失败发现慢**：上游秒拒失败（如排队即 failed）也要等满第一个 10s。
3. **余量偏紧**：`--poll-timeout` 默认 360s，实测闭环 ~300s，余量仅 1.2 倍；上游偶发波动即误报超时。
4. **无量化输出**：日志只打 status，不记轮询时刻与闭环耗时，回归报告里的时序数据靠人工整理。

---

## 二、方案对比

| 方案 | 思路 | 300s 闭环请求数 | 终态滞后 | 评价 |
|---|---|---|---|---|
| **A. 三段阶梯（推荐）** | 5s（0–30s）→ 15s（30–150s）→ 30s（150s+） | **~19** | ≤30s | 参数不依赖任务时长分布；快速失败段更密；实现最小 |
| B. 期望窗口加密 | 10s（0–60s）→ 20s（60–240s）→ 10s（240s+） | ~24 | ≤10s | 依赖「seedance ≈5min」实测分布，换模型即失准 |
| C. 指数退避 + 抖动 | `min(5×1.5ⁿ, 30)` | ~15 | 无上界保证 | 更通用但滞后不可控、可读性差，对单用例过度设计 |
| D. 维持现状 | 固定 10s | 31 | ≤10s | 基线 |

回归套件的诉求是**稳定、少噪音、可复现**，不是基准测试——终态检测滞后从 ≤10s 放宽到 ≤30s 不影响任何断言（R12 只断言终态内容，不断言时刻）。选 **A**。

---

## 三、推荐方案 A 详细设计

### 3.1 阶梯参数

| 段 | 时间窗 | 间隔 | 目的 |
|---|---|---|---|
| 快速失败段 | 0–30s | **5s** | 上游秒拒（参数错/排队失败）尽早发现，6 次查询 |
| running 稳定期 | 30–150s | **15s** | 实测该段无状态变化，8 次查询 |
| 长尾/完成窗口 | 150s–超时 | **30s** | 覆盖 ~300s 完成窗口与更慢模型，7 次查询 |

配套调整：`--poll-timeout` 默认 **360 → 420**（对实测 300s 留 1.4 倍余量）。

### 3.2 代码改动（可直接套用的 patch）

模块级常量（放在 `--poll-timeout` 参数定义附近）：

```python
# --- R12 阶梯轮询参数：快速失败段 → running 稳定期 → 长尾/完成窗口 ---
VIDEO_POLL_FAST_S, VIDEO_POLL_FAST_UNTIL = 5, 30    # 0-30s：秒级失败尽早发现
VIDEO_POLL_MID_S,  VIDEO_POLL_MID_UNTIL  = 15, 150  # 30-150s：running 稳定期
VIDEO_POLL_SLOW_S                        = 30       # 150s+：长尾与期望完成窗口
```

R12 轮询循环（替换 L671–684 的 `deadline`/`while`/`c.check` 块，断言部分不变）：

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

argparse 默认值：

```python
ap.add_argument("--poll-timeout", type=int, default=420)
```

### 3.3 预期收益核算（300s 闭环）

| 指标 | 现状 | 方案 A |
|---|---|---|
| 上游查询次数 | 31（t=0,10,…,300） | **19**（6+8+5），-39% |
| `api_test_requests` 日志行 | 31 | 19 |
| 秒级失败发现 | ≤10s | **≤5s** |
| 终态检测滞后 | ≤10s | ≤30s（仅影响 note 精度，不影响断言） |
| 超时余量（实测 300s） | 1.2× | **1.4×** |
| 报告时序数据 | 人工整理 | `c.note` 自动落表（闭环耗时/轮询次数/滞后上界） |

---

## 四、验证方案

按成本递增三步：

1. **静态时刻表核对（零成本）**：模拟 interval 逻辑打印查询时刻，核对拐点。

```bash
python3 -c "
t, n = 0, 0
while t <= 420:
    print('t=%3ds poll#%d' % (t, n)); n += 1
    t += 5 if t < 30 else 15 if t < 150 else 30
" | head -25
```

预期：0,5,…,25 后跳 15s 步进至 150，之后 30s 步进；420s 内共 22 次。

2. **快速失败路径冒烟（零成本）**：将循环入口的 task_id 临时换为 `cgt-nonexistent`，确认首次查询即 404 `task_not_found` → 立即 break、不进入任何 sleep（验证终态 break 路径与 `c.note` 输出），验证后还原。

3. **真实闭环一次（¥5.0094）**：

```bash
python3 scripts/regression_full_suite.py \
  --key sk_hrIHgJC4i0ZW0W87ZFBi7dzOzA1tlHRtGhwuRShe --include-video
```

通过标准：日志时间戳 `[  x s]` 呈 5→15→30 阶梯；R12 全部断言通过（终态 succeeded、双位置 video_url 一致）；note 输出闭环耗时 ≈300s、轮询次数 ≈19；R08 费用与历史同价 ¥5.0094（计费确定性复核）。

---

## 五、风险与回滚

| 风险 | 评估 | 缓解 |
|---|---|---|
| 慢模型（>420s）误报超时 | 低——超时提示已指引增大 `--poll-timeout` | 命令行可调，阶梯段自动外推 |
| 终态滞后变大影响后续断言 | 无——R12 后无依赖时刻的用例 | 不适用 |
| 改动引入回归 | 低——仅轮询节奏与日志，断言语义零改动 | 第 2/3 步验证；`git revert` 单文件即回滚 |

---

## 六、范围外说明：费用回填不纳入本次改动

「终态后补一次 `?record_id=X` 查询触发 usage 费用回填」已记录为执行路径知识（[回归报告 §5.3](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/docs/v1-compatibility-regression-full-report-r01-r40-2026-10-24.md)）。不纳入套件的原因：`record_id` 取自 `api_test_requests` 表，套件内启用需引入 mysql CLI 依赖、破坏脚本无 DB 依赖的自包含性。维持现状——套件外 SQL 对账（回归报告 §5.2 已示范），零改动。
