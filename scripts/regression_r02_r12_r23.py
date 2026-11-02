#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
P1–P4 兼容性改造回归测试：关键用例 R02 / R12 / R23 可执行断言脚本

配套文档：
  docs/v1-compatibility-p1-p4-regression-plan-2026-10-24.md
  docs/v1-compatibility-p1-p4-report-2026-10-24.md

用例说明：
  R02  embeddings 数组输入返回 N 条向量（P1 核心修复点，消耗少量 embedding tokens）
  R23  未知 v1 URL 返回 OpenAI 错误信封（P4，无上游消耗）
  R12  视频 prompt-only 创建 → 轮询至终态 → 响应别名不覆盖原字段（P2，真实计费 ~45s）

用法：
  python3 scripts/regression_r02_r12_r23.py --key <sk-...>
  python3 scripts/regression_r02_r12_r23.py --key <sk-...> --include-video   # 追加 R12

选项：
  --base          服务地址（默认 http://127.0.0.1:8080）
  --model         R02 嵌入模型 id（默认 doubao/doubao-embedding-vision-251215）
  --video-model   R12 视频模型 id（默认 doubao/doubao-seedance-2-0-260128）
  --poll-timeout  R12 轮询上限秒数（默认 360；seedance-2-0 实测约 5 分钟）

API Key 也可用环境变量 ATH_API_KEY 传入。
从数据库取测试 key：
  /Applications/ServBay/bin/mysql -h127.0.0.1 -uroot -pServBay.dev ai_transit_hub \\
    -N -s -e "SELECT api_key FROM user_api_keys WHERE status=1 LIMIT 1"
"""

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request

# ---------------------------------------------------------------- 基础设施


class CaseResult:
    """单个用例的检查结果收集器"""

    def __init__(self, case_id, title):
        self.case_id = case_id
        self.title = title
        self.checks = []  # (name, ok, detail)

    def check(self, name, ok, detail=""):
        self.checks.append((name, bool(ok), detail))
        return ok

    @property
    def passed(self):
        return all(ok for _, ok, _ in self.checks)

    def print_report(self):
        print("\n=== %s · %s" % (self.case_id, self.title))
        for name, ok, detail in self.checks:
            mark = "PASS" if ok else "FAIL"
            line = "  [%s] %s" % (mark, name)
            if detail:
                line += "  (%s)" % detail
            print(line)
        print("  => %s" % ("通过" if self.passed else "未通过"))


def build_request(base, method, path, key, body=None):
    req = urllib.request.Request(base + path, method=method)
    req.add_header("Authorization", "Bearer " + key)
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        req.add_header("Content-Type", "application/json")
    return req, data


def call(base, method, path, key, body=None):
    """返回 (status, parsed_json)。非 JSON 响应返回 (status, raw_text)。"""
    req, data = build_request(base, method, path, key, body)
    try:
        with urllib.request.urlopen(req, data, timeout=60) as resp:
            return resp.status, json.loads(resp.read())
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, raw.decode(errors="replace")[:200]
    except urllib.error.URLError as e:
        raise SystemExit("服务不可达（%s%s）：%s" % (base, path, e.reason))

# ---------------------------------------------------------------- 用例


def r02_embeddings_array(base, key, model):
    """R02：数组输入 input=["你好","世界"] 返回 2 条向量（P1 核心修复点）。

    修复前：上游单对象语义导致 data 仅为 1 条向量且非标准数组（静默丢数据）。
    """
    r = CaseResult("R02", "embeddings 数组输入返回 N 条向量")
    st, body = call(base, "POST", "/v1/embeddings", key, {"model": model, "input": ["你好", "世界"]})

    r.check("HTTP 200", st == 200, "实际 %s" % st)
    if not isinstance(body, dict):
        r.check("响应为 JSON 对象", False, repr(body)[:120])
        return r
    r.check("顶层 object == list", body.get("object") == "list", repr(body.get("object")))
    data = body.get("data")
    r.check("data 为数组", isinstance(data, list), type(data).__name__)
    if not isinstance(data, list):
        return r
    r.check("data 长度 == 2（修复前为 1）", len(data) == 2, "实际 %d" % len(data))
    r.check("index == [0, 1] 顺序与输入一致",
            [d.get("index") for d in data] == [0, 1],
            repr([d.get("index") for d in data]))
    r.check("每项 object == embedding",
            all(d.get("object") == "embedding" for d in data))
    dims = [len(d.get("embedding") or []) for d in data]
    r.check("每项含非空向量", all(dim > 0 for dim in dims), "维度 %s" % dims)
    r.check("两条向量内容不同（确实按输入拆分）",
            len(data) == 2 and data[0].get("embedding") != data[1].get("embedding"))
    return r


def r23_unknown_url(base, key):
    """R23：未匹配的 v1 路径返回 OpenAI 信封 404（P4，NoRoute 改造）。

    修复前：gin 默认纯文本 "404 page not found"。
    """
    r = CaseResult("R23", "未知 v1 URL 返回错误信封 404")

    st, body = call(base, "GET", "/v1/nonexistent", key)
    r.check("GET /v1/nonexistent -> 404", st == 404, "实际 %s" % st)
    if isinstance(body, dict) and isinstance(body.get("error"), dict):
        err = body["error"]
        r.check("error 为对象信封（修复前为字符串）", True)
        r.check("code == unknown_url", err.get("code") == "unknown_url", repr(err.get("code")))
        r.check("type == invalid_request_error",
                err.get("type") == "invalid_request_error", repr(err.get("type")))
        r.check("message 含请求方法与路径",
                "GET /v1/nonexistent" in str(err.get("message", "")),
                repr(err.get("message"))[:100])
    else:
        r.check("error 为对象信封", False, repr(body)[:120])

    # 变体：路径存在但方法/子路径不匹配
    st2, body2 = call(base, "POST", "/v1/chat/completions/extra", key, {})
    r.check("POST /v1/chat/completions/extra -> 404", st2 == 404, "实际 %s" % st2)
    if isinstance(body2, dict) and isinstance(body2.get("error"), dict):
        r.check("变体同为 unknown_url 信封", body2["error"].get("code") == "unknown_url")
    else:
        r.check("变体同为 unknown_url 信封", False, repr(body2)[:120])
    return r


def r12_video_response_alias(base, key, video_model, poll_timeout):
    """R12：prompt-only 创建任务 → 轮询至 succeeded → 顶层 video_url 别名与 content.video_url 一致（P2）。

    真实调用 Ark 上游并产生计费（~45s），需 --include-video 显式开启。
    """
    r = CaseResult("R12", "视频 prompt-only 闭环 + 响应别名不覆盖原字段")

    # 1) 仅 prompt 创建（未转换时 Ark 因缺 content 拒绝，200 即证明适配生效）
    st, body = call(base, "POST", "/v1/videos/generations/tasks", key,
                    {"model": video_model, "prompt": "一只猫在草地上奔跑，阳光明媚"})
    r.check("prompt-only 创建 -> 200", st == 200, "实际 %s %s" % (st, repr(body)[:120]))
    if st != 200 or not isinstance(body, dict):
        return r
    task_id = body.get("id") or body.get("task_id")
    r.check("返回任务 ID", bool(task_id), repr(task_id))
    if not task_id:
        return r
    print("  任务 ID：%s（轮询至终态，最长 %ds）" % (task_id, poll_timeout))

    # 2) 轮询至终态
    deadline = time.time() + poll_timeout
    final = None
    while time.time() < deadline:
        time.sleep(5)
        st2, body2 = call(base, "GET", "/v1/videos/generations/tasks/" + str(task_id), key)
        if st2 != 200 or not isinstance(body2, dict):
            r.check("轮询响应 200", False, "status=%s body=%s" % (st2, repr(body2)[:120]))
            return r
        status = str(body2.get("status", "")).lower()
        print("    status=%s" % status)
        if status in ("succeeded", "failed", "cancelled", "expired"):
            final = body2
            break
    r.check("在时限内到达终态", final is not None)
    if final is None:
        return r
    r.check("终态为 succeeded", str(final.get("status", "")).lower() == "succeeded",
            repr(final.get("status")))

    # 3) 别名断言：顶层 video_url 与 content.video_url 同时存在且一致（归一化不覆盖原字段）
    content = final.get("content") if isinstance(final.get("content"), dict) else {}
    top_url = final.get("video_url")
    nested_url = content.get("video_url")
    r.check("顶层 video_url 存在（别名已补）", isinstance(top_url, str) and bool(top_url),
            repr(top_url)[:80])
    r.check("嵌套 content.video_url 保留（不覆盖原字段）",
            isinstance(nested_url, str) and bool(nested_url), repr(nested_url)[:80])
    r.check("别名与嵌套一致", bool(top_url) and top_url == nested_url)
    return r

# ---------------------------------------------------------------- 入口


def main():
    ap = argparse.ArgumentParser(description="P1-P4 回归关键用例 R02/R12/R23")
    ap.add_argument("--key", default=os.environ.get("ATH_API_KEY", ""), help="v1 API Key")
    ap.add_argument("--base", default="http://127.0.0.1:8080")
    ap.add_argument("--model", default="doubao/doubao-embedding-vision-251215")
    ap.add_argument("--video-model", default="doubao/doubao-seedance-2-0-260128")
    ap.add_argument("--poll-timeout", type=int, default=360)
    ap.add_argument("--include-video", action="store_true",
                    help="执行 R12（真实计费，创建视频任务并轮询至终态，seedance-2-0 实测约 5 分钟）")
    args = ap.parse_args()

    if not args.key:
        raise SystemExit(
            "缺少 API Key：用 --key 传入，或设置环境变量 ATH_API_KEY。\n"
            "取测试 key：/Applications/ServBay/bin/mysql -h127.0.0.1 -uroot -pServBay.dev "
            "ai_transit_hub -N -s -e \"SELECT api_key FROM user_api_keys WHERE status=1 LIMIT 1\"")

    print("服务：%s   模型：%s" % (args.base, args.model))
    results = [r02_embeddings_array(args.base, args.key, args.model),
               r23_unknown_url(args.base, args.key)]

    if args.include_video:
        results.append(r12_video_response_alias(args.base, args.key,
                                                args.video_model, args.poll_timeout))
    else:
        print("\n[R12] 已跳过（真实计费用例，--include-video 开启）")

    for r in results:
        r.print_report()

    failed = [r.case_id for r in results if not r.passed]
    print("\n汇总：%d/%d 通过%s" % (
        len(results) - len(failed), len(results),
        ("，失败：" + ", ".join(failed)) if failed else ""))
    sys.exit(1 if failed else 0)


if __name__ == "__main__":
    main()
