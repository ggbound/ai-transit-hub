#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
v1 兼容性改造全面回归测试套件（多维度 / 多输入参数）

覆盖回归计划（docs/v1-compatibility-p1-p4-regression-plan-2026-10-24.md）中的免费/低成本必测用例：

  A. 共享管线正向（P4 防守）：R31 chat 非流式、R32 chat 流式、R33 messages(Bearer/X-Api-Key)、
     R34 messages 流式、R35 responses 非流式+流式、R38 X-Api-Key + /api/v1/models
  B. P3 能力元数据：R16 全量完整性、R17 text 类动态端点、R18 多模态嵌入判定、
     R19 video 类契约、R20 Anthropic 列表不受影响、R21 缓存一致性
  C. P4 错误信封矩阵：R22 401×3、R23 未知 URL×2、R24 非 v1 404 不变、R25 透传保真×2、
     R26 模型路由×2、R27 413、R28 双前缀一致性
  D. P1 embeddings：R01 单文本、R02 数组、R03 多模态数组+usage 累加、R04 多模态单条、
     R05 上游错误透传、R06 10 条数组
  E. P2 校验（免费部分）：R10 prompt/content 冲突、R11 prompt 类型错误、R14 任务不存在
  F. 计费用例（默认跳过）：
     --include-video  R08 prompt-only 创建 + R13 running 查询 + R12 闭环与别名（真实计费，
                      seedance-2-0 实测约 5 分钟，阶梯轮询 5/15/30s，上限默认 420s）
     --include-image  R36 images 生成（真实计费）

用法：
  python3 scripts/regression_full_suite.py --key <KEY>
  python3 scripts/regression_full_suite.py --key <KEY> --include-video --include-image

Key 也可用环境变量 ATH_API_KEY 传入。取测试 key：
  /Applications/ServBay/bin/mysql -h127.0.0.1 -uroot -pServBay.dev ai_transit_hub \\
    -N -s -e "SELECT api_key FROM user_api_keys WHERE status=1 LIMIT 1"

跳过说明：R09（content-only 创建，计费）、R15/R07/R39（DB 记账，脚本外对账）、
R29/R30（需额外构造数据）、R37（agent 任务流，超出 P1-P4 范围）。
退出码：0 = 全部通过；1 = 存在失败。
"""
import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request

# ---------------------------------------------------------------- 结果收集

class Case(object):
    def __init__(self, cid, title):
        self.cid = cid
        self.title = title
        self.items = []      # (断言名, 是否通过, 详情)
        self.notes = []      # 备注信息
        self.skipped = False

    def check(self, name, ok, detail=""):
        self.items.append((name, bool(ok), str(detail)))
        return bool(ok)

    def skip(self, reason):
        self.skipped = True
        self.notes.append("SKIP: " + reason)

    def note(self, msg):
        self.notes.append(msg)

    @property
    def ok(self):
        if self.skipped:
            return True
        return all(ok for _, ok, _ in self.items)

    def report(self):
        if self.skipped:
            print("  => 跳过")
            for n in self.notes:
                print("    " + n)
            return
        for name, ok, detail in self.items:
            mark = "PASS" if ok else "FAIL"
            line = "  [%s] %s" % (mark, name)
            if detail:
                line += "  (%s)" % detail
            print(line)
        for n in self.notes:
            print("    " + n)
        print("  => %s" % ("通过" if self.ok else "未通过"))


def summary_print(cases):
    total = len(cases)
    passed = sum(1 for c in cases if c.ok and not c.skipped)
    skipped = sum(1 for c in cases if c.skipped)
    failed = total - passed - skipped
    print("\n汇总：%d/%d 通过，跳过 %d，失败 %d" % (passed, total, skipped, failed))
    if failed:
        print("失败用例：" + ", ".join(c.cid for c in cases if not c.ok and not c.skipped))
    if skipped:
        print("跳过用例：" + ", ".join(c.cid for c in cases if c.skipped))
    return failed == 0


# ---------------------------------------------------------------- HTTP 封装

def req(base, path, key=None, body=None, auth="bearer", method=None,
        headers=None, timeout=90, raw_body=None):
    """返回 (status, 解析后的对象或原始文本)"""
    url = base.rstrip("/") + path
    data = None
    if raw_body is not None:
        data = raw_body
    elif body is not None:
        data = json.dumps(body).encode("utf-8")
    r = urllib.request.Request(url, data=data)
    r.add_header("Content-Type", "application/json")
    if auth == "bearer" and key:
        r.add_header("Authorization", "Bearer " + key)
    elif auth == "xapi" and key:
        r.add_header("x-api-key", key)
        r.add_header("anthropic-version", "2023-06-01")
    if method:
        r.get_method = (lambda m=method: m)
    for k, v in (headers or {}).items():
        r.add_header(k, v)
    try:
        resp = urllib.request.urlopen(r, timeout=timeout)
        status = resp.getcode()
        text = resp.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as e:
        status = e.code
        text = e.read().decode("utf-8", "replace")
    try:
        return status, json.loads(text)
    except ValueError:
        return status, text


def sse(base, path, key, body, auth="bearer", timeout=120, max_lines=5000):
    """读取 SSE 流，返回 (status, 原始行列表)"""
    url = base.rstrip("/") + path
    data = json.dumps(body).encode("utf-8")
    r = urllib.request.Request(url, data=data)
    r.add_header("Content-Type", "application/json")
    r.add_header("Accept", "text/event-stream")
    if auth == "bearer":
        r.add_header("Authorization", "Bearer " + key)
    elif auth == "xapi":
        r.add_header("x-api-key", key)
        r.add_header("anthropic-version", "2023-06-01")
    try:
        resp = urllib.request.urlopen(r, timeout=timeout)
        status = resp.getcode()
        lines = []
        while len(lines) < max_lines:
            line = resp.readline()
            if not line:
                break
            lines.append(line.decode("utf-8", "replace").rstrip("\n"))
        return status, lines
    except urllib.error.HTTPError as e:
        return e.code, [e.read().decode("utf-8", "replace")]


def err_envelope_ok(body, code=None, type_="invalid_request_error"):
    """断言 body 为 OpenAI 错误信封，可选断言 code/type"""
    if not isinstance(body, dict) or not isinstance(body.get("error"), dict):
        return False, "error 非对象信封: %r" % (body if isinstance(body, str) else type(body))
    e = body["error"]
    if not e.get("message") or not e.get("type"):
        return False, "message/type 缺失: %r" % (e,)
    if code is not None and e.get("code") != code:
        return False, "code=%r (期望 %s)" % (e.get("code"), code)
    if type_ is not None and e.get("type") != type_:
        return False, "type=%r" % (e.get("type"),)
    return True, "code=%s type=%s" % (e.get("code"), e.get("type"))


# ---------------------------------------------------------------- 用例实现

def section_a_pipeline(base, key, chat_model, messages_model, responses_model):
    cases = []

    # R31 chat 非流式
    c = Case("R31", "chat 非流式 200（P4 改写 respondV1Success 后正向不受影响）")
    st, b = req(base, "/v1/chat/completions", key, {
        "model": chat_model, "max_tokens": 16,
        "messages": [{"role": "user", "content": "只回复两个字：好的"}]})
    c.check("HTTP 200", st == 200, "实际 %s" % st)
    if st == 200 and isinstance(b, dict):
        c.check("无 error 字段（成功响应未被误判）", "error" not in b,
                json.dumps(b.get("error"), ensure_ascii=False)[:120])
        ch = (b.get("choices") or [{}])[0]
        msg = ch.get("message") or {}
        c.check("choices[0].message.content 非空", bool(msg.get("content")),
                repr(msg.get("content"))[:60])
        c.check("usage 完整且 total_tokens>0",
                isinstance(b.get("usage"), dict) and b["usage"].get("total_tokens", 0) > 0,
                "total_tokens=%s" % (b.get("usage") or {}).get("total_tokens"))
        c.note("object=%s model=%s" % (b.get("object"), b.get("model")))
    else:
        c.check("响应为 JSON 对象", isinstance(b, dict), repr(b)[:200])
    cases.append(c)

    # R32 chat 流式
    c = Case("R32", "chat 流式 SSE 正常结束（data: [DONE]）")
    st, lines = sse(base, "/v1/chat/completions", key, {
        "model": chat_model, "max_tokens": 16, "stream": True,
        "messages": [{"role": "user", "content": "从 1 数到 5，用顿号分隔"}]})
    c.check("HTTP 200", st == 200, "实际 %s" % st)
    has_done = any(l.strip() == "data: [DONE]" for l in lines)
    deltas = [l for l in lines if l.startswith("data: ") and "[DONE]" not in l]
    c.check("出现 [DONE] 结束标记", has_done)
    c.check("存在增量 data 块（≥2）", len(deltas) >= 2, "增量块 %d 个" % len(deltas))
    cases.append(c)

    # R33a / R33b messages 非流式（Bearer / X-Api-Key）
    for variant, auth in (("R33a", "bearer"), ("R33b", "xapi")):
        c = Case(variant, "messages 非流式（%s）Anthropic 格式完整" %
                 ("Bearer" if auth == "bearer" else "X-Api-Key"))
        st, b = req(base, "/v1/messages", key, {
            "model": messages_model, "max_tokens": 16,
            "messages": [{"role": "user", "content": "只回复两个字：好的"}]}, auth=auth)
        c.check("HTTP 200", st == 200, "实际 %s" % st)
        if st == 200 and isinstance(b, dict):
            c.check("type == message", b.get("type") == "message", "type=%r" % b.get("type"))
            c.check("role == assistant", b.get("role") == "assistant", "role=%r" % b.get("role"))
            content = b.get("content") or []
            text = "".join(x.get("text", "") for x in content if isinstance(x, dict))
            c.check("content 含非空文本", bool(text.strip()), repr(text)[:60])
            c.check("stop_reason 存在", "stop_reason" in b, "stop_reason=%r" % b.get("stop_reason"))
            c.check("usage 输入 token >=1", (b.get("usage") or {}).get("input_tokens", 0) >= 1,
                    "usage=%s" % json.dumps(b.get("usage")))
        else:
            c.check("响应为 JSON 对象", isinstance(b, dict), repr(b)[:200])
        cases.append(c)

    # R34 messages 流式
    c = Case("R34", "messages 流式事件序列完整")
    st, lines = sse(base, "/v1/messages", key, {
        "model": messages_model, "max_tokens": 16, "stream": True,
        "messages": [{"role": "user", "content": "从 1 数到 5，用顿号分隔"}]})
    c.check("HTTP 200", st == 200, "实际 %s" % st)
    raw = "\n".join(lines)
    c.check("message_start 事件", "message_start" in raw)
    c.check("content_block_delta 事件", "content_block_delta" in raw)
    c.check("message_stop 事件", "message_stop" in raw)
    cases.append(c)

    # R35a / R35b responses 非流式 + 流式
    c = Case("R35a", "responses 非流式（object=response，status 合法 + output 非空）")
    st, b = req(base, "/v1/responses", key, {
        "model": responses_model, "max_output_tokens": 16,
        "input": "只回复两个字：好的"})
    c.check("HTTP 200", st == 200, "实际 %s" % st)
    if st == 200 and isinstance(b, dict):
        c.check("object == response", b.get("object") == "response", "object=%r" % b.get("object"))
        # max_output_tokens 截断时 status=incomplete（OpenAI Responses 合法语义），completed/incomplete 均接受
        stt = b.get("status")
        c.check("status ∈ {completed, incomplete}", stt in ("completed", "incomplete"), "status=%r" % stt)
        if stt == "incomplete":
            det = b.get("incomplete_details") or {}
            # reason 值随上游方言：OpenAI 为 max_output_tokens，豆包 Ark 为 length——透传保真不改写
            c.check("incomplete_details.reason 非空（记录方言）",
                    bool(det.get("reason")), "incomplete_details=%r" % det)
        out = b.get("output") or []
        c.check("output 非空", isinstance(out, list) and len(out) > 0, "output 项数 %d" % len(out))
    else:
        c.check("响应为 JSON 对象", isinstance(b, dict), repr(b)[:200])
    cases.append(c)

    c = Case("R35b", "responses 流式事件序列")
    st, lines = sse(base, "/v1/responses", key, {
        "model": responses_model, "max_output_tokens": 16, "stream": True,
        "input": "从 1 数到 5，用顿号分隔"})
    c.check("HTTP 200", st == 200, "实际 %s" % st)
    raw = "\n".join(lines)
    c.check("存在 event 行", sum(1 for l in lines if l.startswith("event:")) > 0)
    c.check("以 response.completed 结束（或 [DONE]）",
            "response.completed" in raw or "[DONE]" in raw)
    cases.append(c)

    # R38 X-Api-Key + /api/v1/models
    c = Case("R38", "X-Api-Key 认证 + /api/v1/models 备用前缀")
    st, b = req(base, "/api/v1/models", key, auth="xapi")
    c.check("HTTP 200", st == 200, "实际 %s" % st)
    c.check("data 非空", isinstance(b, dict) and len(b.get("data", [])) > 0,
            "模型数 %s" % (len(b.get("data", [])) if isinstance(b, dict) else "?"))
    cases.append(c)
    return cases


def section_b_models(models, base, key):
    cases = []

    def comp_map(mlist):
        return {m["id"]: m.get("compatibility") for m in mlist if isinstance(m, dict)}

    # R16 + R40：全量模型元数据完整性
    # 设计：endpoints/request 恒有；response 仅对网关有响应保证的类型声明
    # （embedding=P1 归一化、video=P2 适配），text/image 无 response 段属预期。
    c = Case("R16", "全量模型 compatibility 按类型分段完整 + 原有字段保留（含 R40）")
    ids = [m.get("id") for m in models if isinstance(m, dict)]
    c.check("模型数 >0", len(models) > 0, "共 %d 个" % len(models))
    bad = []
    n_resp = 0
    for m in models:
        mid = m.get("id", "?")
        comp = m.get("compatibility")
        if not isinstance(comp, dict):
            bad.append("%s: 无 compatibility" % mid)
            continue
        for k in ("endpoints", "request"):
            if k not in comp:
                bad.append("%s: 缺 %s" % (mid, k))
        ep = comp.get("endpoints") or {}
        is_emb = ep.get("create") == "POST /v1/embeddings"
        is_vid = "query" in ep
        if is_emb or is_vid:
            n_resp += 1
            if "response" not in comp:
                bad.append("%s: embedding/video 类型缺 response" % mid)
        elif "response" in comp:
            bad.append("%s: text/image 类型不应有 response" % mid)
        if "capabilities" not in m:
            bad.append("%s: 缺原有 capabilities" % mid)
    c.check("按类型分段完整（embedding/video 含 response）+ capabilities 保留",
            not bad, "; ".join(bad[:5]) if bad else
            "覆盖 %d 个模型，其中 %d 个声明 response" % (len(models), n_resp))
    c.note("text/image 无 response 段为设计行为（modelCompatibility 仅对 embedding/video 声明响应保证）")
    cases.append(c)

    # R17 text 类动态端点
    c = Case("R17", "text 类动态端点（豆包系无 messages；ZhiyunTB 含 responses）")
    cm = comp_map(models)
    db_mini = cm.get("doubao/doubao-seed-2-0-mini-260428") or {}
    ep = db_mini.get("endpoints") or {}
    c.check("豆包 text：有 chat 端点", "chat" in ep, "endpoints=%s" % sorted(ep.keys()))
    c.check("豆包 text：无 messages 端点（SupportsMessages=false）", "messages" not in ep,
            "endpoints=%s" % sorted(ep.keys()))
    zy = cm.get("ZhiyunTB/deepseek-v4-flash-0731") or {}
    c.check("ZhiyunTB deepseek：含 responses 端点",
            "responses" in (zy.get("endpoints") or {}),
            "endpoints=%s" % sorted((zy.get("endpoints") or {}).keys()))
    cases.append(c)

    # R18 多模态嵌入判定
    c = Case("R18", "多模态嵌入模型 request.input 含 multimodal_content[]（核心）")
    emb = [(mid, (comp or {})) for mid, comp in cm.items() if "embeddings" in ((comp or {}).get("endpoints") or {})]
    c.note("嵌入模型: %s" % emb if emb else "嵌入模型: 未通过 endpoints=embeddings 识别，改按 id 匹配")
    if not emb:
        emb = [(mid, (comp or {})) for mid, comp in cm.items()
               if "embedding" in mid.lower() and "create" in ((comp or {}).get("endpoints") or {})]
    vision = [(mid, comp) for mid, comp in emb if "vision" in mid.lower()]
    plain = [(mid, comp) for mid, comp in emb if "vision" not in mid.lower()]
    if vision:
        inp = ((vision[0][1].get("request") or {}).get("input")) or []
        c.check("vision 嵌入模型 input 含 multimodal_content[]",
                "multimodal_content[]" in inp, "%s input=%s" % (vision[0][0], inp))
    else:
        c.check("存在 vision 嵌入模型", False, "当前模型列表无 vision 嵌入模型")
    if plain:
        inp = ((plain[0][1].get("request") or {}).get("input")) or []
        c.check("通用嵌入模型 input 不含 multimodal_content[]",
                "multimodal_content[]" not in inp, "%s input=%s" % (plain[0][0], inp))
    else:
        c.note("数据面无第二个（非 vision）嵌入模型，该子断言不可测（记录备忘，不计失败）")
    cases.append(c)

    # R19 video 类契约
    c = Case("R19", "video 类契约（endpoints/exclusive_fields/response 别名）")
    vid = cm.get("doubao/doubao-seedance-2-0-260128") or {}
    ep = vid.get("endpoints") or {}
    c.check("endpoints.create 存在", "create" in ep, "endpoints=%s" % sorted(ep.keys()))
    c.check("endpoints.query 存在", "query" in ep, "endpoints=%s" % sorted(ep.keys()))
    c.check("exclusive_fields == [prompt, content]",
            vid.get("request", {}).get("exclusive_fields") == ["prompt", "content"],
            "实际 %s" % (vid.get("request", {}).get("exclusive_fields")))
    c.check("response.video_url == top_level_alias",
            vid.get("response", {}).get("video_url") == "top_level_alias",
            "实际 %r" % (vid.get("response", {}).get("video_url")))
    cases.append(c)

    # R20 Anthropic 列表不受影响
    c = Case("R20", "GET /v1/anthropic/models 保持 Anthropic 极简格式（无 compatibility）")
    st, b = req(base, "/v1/anthropic/models", key)
    c.check("HTTP 200", st == 200, "实际 %s" % st)
    if st == 200 and isinstance(b, dict):
        items = b.get("data") or []
        c.check("data 非空", len(items) > 0, "共 %d 项" % len(items))
        no_comp = all("compatibility" not in i for i in items if isinstance(i, dict))
        c.check("各模型项均无 compatibility 块", no_comp)
        has_id = all("id" in i for i in items if isinstance(i, dict))
        c.check("各模型项含 id", has_id)
        c.note("样例项: %s" % json.dumps(items[0], ensure_ascii=False)[:160] if items else "")
    else:
        c.check("响应为 JSON 对象", isinstance(b, dict), repr(b)[:200])
    cases.append(c)

    # R21 缓存一致性（连续两次请求 compatibility 一致）
    c = Case("R21", "连续两次 /v1/models 输出一致（缓存序列化无损）")
    st2, b2 = req(base, "/v1/models", key)
    c.check("第二次 HTTP 200", st2 == 200, "实际 %s" % st2)
    same = comp_map(models) == comp_map(b2.get("data", []) if isinstance(b2, dict) else [])
    c.check("两次 compatibility 映射完全一致", same)
    cases.append(c)
    return cases


def section_c_errors(base, key, chat_model):
    cases = []

    # R22 401 三连
    c = Case("R22", "401 三连（无认证 / 坏 Bearer / 坏 X-Api-Key）→ 对象信封")
    st, b = req(base, "/v1/models", None)
    ok, d = err_envelope_ok(b, "missing_api_key")
    c.check("无认证头 → 401 missing_api_key", st == 401 and ok, "st=%s %s" % (st, d))
    st, b = req(base, "/v1/models", "sk-invalid-not-a-real-key")
    ok, d = err_envelope_ok(b, "invalid_api_key")
    c.check("坏 Bearer → 401 invalid_api_key", st == 401 and ok, "st=%s %s" % (st, d))
    st, b = req(base, "/v1/models", "sk-invalid-not-a-real-key", auth="xapi")
    ok, d = err_envelope_ok(b, "invalid_api_key")
    c.check("坏 X-Api-Key → 401 invalid_api_key", st == 401 and ok, "st=%s" % st)
    cases.append(c)

    # R23 未知 URL（双场景）
    c = Case("R23", "未知 v1 URL → 404 unknown_url 信封")
    st, b = req(base, "/v1/nonexistent", key)
    ok, d = err_envelope_ok(b, "unknown_url")
    msg = b.get("error", {}).get("message", "") if isinstance(b, dict) else ""
    c.check("GET /v1/nonexistent → 404 信封", st == 404 and ok, "st=%s %s" % (st, d))
    c.check("message 含方法与路径", "GET /v1/nonexistent" in msg, repr(msg)[:80])
    st, b = req(base, "/v1/chat/completions/extra", key, {"model": chat_model})
    ok, d = err_envelope_ok(b, "unknown_url")
    c.check("POST /v1/chat/completions/extra → 404 信封", st == 404 and ok, "st=%s %s" % (st, d))
    cases.append(c)

    # R24 非 v1 前缀 404 不变
    c = Case("R24", "非 v1 前缀 404 保持 gin 默认（不被信封拦截）")
    st, b = req(base, "/nonexistent", key)
    c.check("GET /nonexistent → 404", st == 404, "实际 %s" % st)
    c.check("body == {'error': '404 page not found'}", b == {"error": "404 page not found"},
            repr(b)[:80])
    cases.append(c)

    # R25 透传保真 ×2
    c = Case("R25a", "chat 缺 messages → 400 上游原文透传（未被网关包装）")
    st, b = req(base, "/v1/chat/completions", key, {"model": chat_model, "max_tokens": 16})
    c.check("HTTP 400", st == 400, "实际 %s" % st)
    if isinstance(b, dict) and isinstance(b.get("error"), dict):
        ue = b["error"]
        c.check("error.code 非网关兜底 upstream_error", ue.get("code") != "upstream_error",
                "code=%r message=%r" % (ue.get("code"), str(ue.get("message"))[:80]))
    else:
        c.check("error 为对象（上游 Ark 格式）", isinstance(b, dict) and "error" in b, repr(b)[:160])
    cases.append(c)

    # 双态接受：部分上游（ZhiyunTB deepseek）不强制 max_tokens 返回 200，
    # 透传保真原则下网关不拦截也不改写，两种结果均须为合法 Anthropic 形状
    c = Case("R25b", "messages 缺 max_tokens 透传保真：400→error 信封 / 200→合法 message")
    st, b = req(base, "/v1/messages", key, {
        "model": MESSAGES_MODEL_FALLBACK, "messages": [{"role": "user", "content": "hi"}]})
    if st == 400:
        c.check("400 → 顶层 type == error（Anthropic 信封，非 OpenAI 包装）",
                isinstance(b, dict) and b.get("type") == "error", repr(b)[:160])
    elif st == 200:
        ok = (isinstance(b, dict) and b.get("type") == "message"
              and b.get("role") == "assistant"
              and isinstance(b.get("content"), list) and len(b["content"]) > 0)
        c.check("200 → 合法 Anthropic message（上游不强制 max_tokens，透传不拦截）",
                ok, repr(b)[:160])
    else:
        c.check("HTTP 400 或 200", False, "实际 %s %s" % (st, repr(b)[:120]))
    cases.append(c)

    # R26 模型路由
    c = Case("R26", "模型路由 404（provider_not_found / model_not_found）")
    st, b = req(base, "/v1/chat/completions", key,
                {"model": "nope/none", "messages": [{"role": "user", "content": "x"}]})
    ok, d = err_envelope_ok(b, "provider_not_found")
    c.check("未知 provider → 404 provider_not_found", st == 404 and ok, "st=%s %s" % (st, d))
    st, b = req(base, "/v1/chat/completions", key,
                {"model": "doubao/not-a-real-model-xyz", "messages": [{"role": "user", "content": "x"}]})
    ok, d = err_envelope_ok(b, "model_not_found")
    c.check("未知模型 → 404 model_not_found", st == 404 and ok, "st=%s %s" % (st, d))
    cases.append(c)

    # R27 413
    c = Case("R27", "超过 10MB 请求体 → 413 request_too_large")
    pad = "a" * (11 * 1024 * 1024)
    raw = json.dumps({"model": chat_model, "messages": [{"role": "user", "content": pad}]})
    st, b = req(base, "/v1/chat/completions", key, timeout=120, raw_body=raw.encode("utf-8"))
    ok, d = err_envelope_ok(b, "request_too_large")
    c.check("HTTP 413 + request_too_large 信封", st == 413 and ok, "st=%s %s" % (st, d))
    cases.append(c)

    # R28 双前缀一致性
    c = Case("R28", "/api/v1 前缀错误信封与 /v1 一致")
    st, b = req(base, "/api/v1/models", None)
    ok, d = err_envelope_ok(b, "missing_api_key")
    c.check("无认证 → 401 missing_api_key", st == 401 and ok, "st=%s %s" % (st, d))
    st, b = req(base, "/api/v1/nonexistent", key)
    ok, d = err_envelope_ok(b, "unknown_url")
    c.check("未知 URL → 404 unknown_url", st == 404 and ok, "st=%s %s" % (st, d))
    st, b = req(base, "/api/v1/chat/completions", key,
                {"model": "nope/none", "messages": [{"role": "user", "content": "x"}]})
    ok, d = err_envelope_ok(b, "provider_not_found")
    c.check("未知 provider → 404 provider_not_found", st == 404 and ok, "st=%s %s" % (st, d))
    cases.append(c)
    return cases


def section_d_embeddings(base, key, embed_model):
    cases = []

    # R01 单文本
    c = Case("R01", "单文本输入 → 数组长度 1（回归 T18）")
    st, b = req(base, "/v1/embeddings", key, {"model": embed_model, "input": "你好"})
    c.check("HTTP 200", st == 200, "实际 %s" % st)
    if st == 200 and isinstance(b, dict):
        c.check("顶层 object == list", b.get("object") == "list", "object=%r" % b.get("object"))
        data = b.get("data") or []
        c.check("data 为数组且长度 1", isinstance(data, list) and len(data) == 1, "长度 %d" % len(data))
        if data and isinstance(data[0], dict):
            c.check("data[0].object == embedding", data[0].get("object") == "embedding",
                    "object=%r" % data[0].get("object"))
            c.check("data[0].index == 0", data[0].get("index") == 0, "index=%r" % data[0].get("index"))
            c.check("向量非空", isinstance(data[0].get("embedding"), list) and len(data[0]["embedding"]) > 0,
                    "维度 %s" % len(data[0].get("embedding") or []))
        c.note("model=%r usage=%s" % (b.get("model"), json.dumps(b.get("usage"))))
    else:
        c.check("响应为 JSON 对象", isinstance(b, dict), repr(b)[:200])
    cases.append(c)

    # R02 数组输入
    c = Case("R02", "数组输入 → N 条向量（P1 核心修复）")
    st, b = req(base, "/v1/embeddings", key, {"model": embed_model, "input": ["你好", "世界"]})
    c.check("HTTP 200", st == 200, "实际 %s" % st)
    data = b.get("data") if isinstance(b, dict) else None
    c.check("data 长度 == 2（修复前为 1）", isinstance(data, list) and len(data) == 2,
            "长度 %s" % (len(data) if isinstance(data, list) else "?"))
    if isinstance(data, list) and len(data) == 2:
        c.check("index == [0, 1]", [d.get("index") for d in data] == [0, 1],
                "%s" % [d.get("index") for d in data])
        dims = [len(d.get("embedding") or []) for d in data]
        c.check("向量非空且维度一致", all(d > 0 for d in dims) and len(set(dims)) == 1, "维度 %s" % dims)
        c.check("两条向量内容不同", data[0]["embedding"] != data[1]["embedding"])
        c.check("全部 object == embedding", all(d.get("object") == "embedding" for d in data))
    cases.append(c)

    # R03 多模态数组 + usage 累加 / R04 多模态单条（共用做 usage 对比）
    st3, b3 = req(base, "/v1/embeddings", key,
                  {"model": embed_model, "input": ["回归测试文本", "回归测试文本", "回归测试文本"]})
    st4, b4 = req(base, "/v1/embeddings", key,
                  {"model": embed_model, "input": "回归测试文本"})
    c3 = Case("R03", "多模态模型 3 条数组 → 3 条向量 + usage 累加（核心）")
    c3.check("HTTP 200", st3 == 200, "实际 %s" % st3)
    d3 = b3.get("data") if isinstance(b3, dict) else None
    c3.check("data 长度 == 3", isinstance(d3, list) and len(d3) == 3,
             "长度 %s" % (len(d3) if isinstance(d3, list) else "?"))
    if isinstance(d3, list) and len(d3) == 3:
        c3.check("index == [0, 1, 2]（并发合并顺序）",
                 [d.get("index") for d in d3] == [0, 1, 2],
                 "%s" % [d.get("index") for d in d3])
    u3 = (b3.get("usage") or {}).get("prompt_tokens", 0) if isinstance(b3, dict) else 0
    c3.note("usage.prompt_tokens(3条)=%s" % u3)
    cases.append(c3)

    c4 = Case("R04", "多模态模型单输入 → 单请求路径 data 长度 1")
    c4.check("HTTP 200", st4 == 200, "实际 %s" % st4)
    d4 = b4.get("data") if isinstance(b4, dict) else None
    c4.check("data 长度 == 1", isinstance(d4, list) and len(d4) == 1,
             "长度 %s" % (len(d4) if isinstance(d4, list) else "?"))
    u4 = (b4.get("usage") or {}).get("prompt_tokens", 0) if isinstance(b4, dict) else 0
    c4.note("usage.prompt_tokens(1条)=%s" % u4)
    cases.append(c4)

    c3.note("与 R04 对比：倍率 = %.2f（期望 ≈3）" % (float(u3) / u4 if u4 else -1))
    if u4:
        c3.check("usage 累加生效（倍率 2~4 倍）", 2.0 <= float(u3) / u4 <= 4.0,
                 "倍率 %.2f" % (float(u3) / u4))

    # R05 上游错误透传
    c = Case("R05", "input 空串 → 400 上游原文透传（P4 未破坏保真主路径）")
    st, b = req(base, "/v1/embeddings", key, {"model": embed_model, "input": ""})
    c.check("HTTP 400", st == 400, "实际 %s" % st)
    if isinstance(b, dict) and isinstance(b.get("error"), dict):
        ue = b["error"]
        c.check("error.code 非网关兜底 upstream_error", ue.get("code") != "upstream_error",
                "code=%r message=%r" % (ue.get("code"), str(ue.get("message"))[:80]))
        if ue.get("code") == "MissingParameter":
            c.note("上游 Ark MissingParameter 原文透传确认")
    else:
        c.check("error 为对象", isinstance(b, dict) and "error" in b, repr(b)[:160])
    cases.append(c)

    # R06 10 条数组
    c = Case("R06", "10 条数组 → 10 条向量、index 0-9 无乱序（并发上限 8 合并稳定性）")
    inputs = ["第%d号样本" % i for i in range(10)]
    st, b = req(base, "/v1/embeddings", key, {"model": embed_model, "input": inputs})
    c.check("HTTP 200", st == 200, "实际 %s" % st)
    data = b.get("data") if isinstance(b, dict) else None
    c.check("data 长度 == 10", isinstance(data, list) and len(data) == 10,
            "长度 %s" % (len(data) if isinstance(data, list) else "?"))
    if isinstance(data, list) and len(data) == 10:
        idx = [d.get("index") for d in data]
        c.check("index == 0..9 无乱序", idx == list(range(10)), "%s" % idx)
        dims = sorted({len(d.get("embedding") or []) for d in data})
        c.check("维度一致", len(dims) == 1 and dims[0] > 0, "维度集合 %s" % dims)
        uniq = len({tuple(d.get("embedding") or []) for d in data})
        c.check("10 条向量互不相同", uniq == 10, "去重后 %d 条" % uniq)
        u = (b.get("usage") or {}).get("prompt_tokens", 0)
        c.note("usage.prompt_tokens=%s" % u)
    cases.append(c)
    return cases


def section_e_videos_free(base, key, video_model):
    cases = []

    # R10 prompt+content 冲突
    c = Case("R10", "prompt 与 content 并存 → 400 prompt_content_conflict（L3，校验在计费前）")
    st, b = req(base, "/v1/videos/generations/tasks", key, {
        "model": video_model, "prompt": "一只猫",
        "content": [{"type": "text", "text": "一只猫"}]})
    ok, d = err_envelope_ok(b, "prompt_content_conflict")
    c.check("400 + prompt_content_conflict 信封", st == 400 and ok, "st=%s %s" % (st, d))
    cases.append(c)

    # R11 prompt 类型错误
    c = Case("R11", "prompt 类型错误（123）→ 400 invalid_prompt_type")
    st, b = req(base, "/v1/videos/generations/tasks", key, {"model": video_model, "prompt": 123})
    ok, d = err_envelope_ok(b, "invalid_prompt_type")
    c.check("400 + invalid_prompt_type 信封", st == 400 and ok, "st=%s %s" % (st, d))
    cases.append(c)

    # R14 任务不存在
    c = Case("R14", "查询不存在任务 → 404 task_not_found（新信封）")
    st, b = req(base, "/v1/videos/generations/tasks/cgt-nonexistent-0000", key)
    ok, d = err_envelope_ok(b, "task_not_found")
    c.check("404 + task_not_found 信封", st == 404 and ok, "st=%s %s" % (st, d))
    cases.append(c)
    return cases


def section_f_video_paid(base, key, video_model, poll_timeout):
    """计费用例：R08 创建 + R13 running 查询 + R12 闭环与别名（一个视频任务）"""
    cases = []

    # R08 prompt-only 创建
    c = Case("R08", "prompt-only 创建任务（P2 核心修复）")
    st, b = req(base, "/v1/videos/generations/tasks", key,
                {"model": video_model, "prompt": "一只橘猫在草地上奔跑，阳光明媚，5 秒镜头"}, timeout=60)
    c.check("HTTP 200", st == 200, "实际 %s" % st)
    task_id = b.get("id") if isinstance(b, dict) else None
    c.check("返回任务 ID", bool(task_id), "id=%r" % task_id)
    cases.append(c)
    if not task_id:
        return cases

    # R13 running 任务立即查询
    c = Case("R13", "running 任务立即查询：无 content 归一化污染、无 video_url:null")
    st, b = req(base, "/v1/videos/generations/tasks/%s" % task_id, key, timeout=60)
    c.check("HTTP 200", st == 200, "实际 %s" % st)
    if st == 200 and isinstance(b, dict):
        c.check("status 为进行中状态", b.get("status") in ("queued", "running", "submitted"),
                "status=%r" % b.get("status"))
        c.check("无 content 键污染（或为合法 null 缺省）",
                "content" not in b or b.get("content") is None,
                "keys=%s" % sorted(b.keys()))
        c.check("顶层无 video_url:null",
                not ("video_url" in b) or b.get("video_url") is not None,
                "video_url=%r" % b.get("video_url"))
    cases.append(c)

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
    if isinstance(final, dict) and final.get("status") == "succeeded":
        c.check("终态 == succeeded", final.get("status") == "succeeded",
                "status=%r" % final.get("status"))
        c.check("顶层 video_url 存在", bool(final.get("video_url")))
        c.check("content.video_url 存在（别名写入）",
                bool((final.get("content") or {}).get("video_url")))
        c.check("两处 URL 一致", final.get("video_url") == (final.get("content") or {}).get("video_url"))
        c.note("duration=%s ratio=%s resolution=%s fps=%s usage=%s" % (
            final.get("duration"), final.get("ratio"), final.get("resolution"),
            final.get("framespersecond"), json.dumps(final.get("usage"))))
    elif final is not None:
        c.check("终态为 succeeded", False, "实际终态 status=%r body=%s" % (
            final.get("status"), json.dumps(final, ensure_ascii=False)[:200]))
    cases.append(c)
    return cases


def section_f_image_paid(base, key, image_model):
    c = Case("R36", "images 生成返回 data[].url（真实计费）")
    # seedream-4-5 上游要求 size ≥ 3686400 像素（1920x1920），1024x1024 会被 Ark 400 拒绝（透传保真）
    st, b = req(base, "/v1/images/generations", key, {
        "model": image_model, "prompt": "一只戴圣诞帽的橘猫，简约扁平插画风格",
        "size": "2048x2048", "n": 1}, timeout=180)
    c.check("HTTP 200", st == 200, "实际 %s" % st)
    if st == 200 and isinstance(b, dict):
        data = b.get("data") or []
        c.check("data 非空", len(data) > 0, "项数 %d" % len(data))
        if data:
            first = data[0]
            c.check("首项含 url 或 b64_json",
                    bool(first.get("url")) or bool(first.get("b64_json")),
                    "keys=%s" % sorted(first.keys()))
    else:
        c.check("响应为 JSON 对象", isinstance(b, dict), repr(b)[:200])
    return [c]


# ---------------------------------------------------------------- 主入口

MESSAGES_MODEL_FALLBACK = "ZhiyunTB/deepseek-v4-flash-0731"

# --- R12 阶梯轮询参数：快速失败段 → running 稳定期 → 长尾/完成窗口 ---
VIDEO_POLL_FAST_S, VIDEO_POLL_FAST_UNTIL = 5, 30    # 0-30s：秒级失败尽早发现
VIDEO_POLL_MID_S,  VIDEO_POLL_MID_UNTIL  = 15, 150  # 30-150s：running 稳定期
VIDEO_POLL_SLOW_S                        = 30       # 150s+：长尾与期望完成窗口


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", default="http://127.0.0.1:8080")
    ap.add_argument("--key", default=os.environ.get("ATH_API_KEY"))
    ap.add_argument("--chat-model", default="doubao/doubao-seed-2-0-mini-260428")
    ap.add_argument("--messages-model", default="ZhiyunTB/deepseek-v4-flash-0731")
    ap.add_argument("--responses-model", default="doubao/doubao-seed-2-0-mini-260428")
    ap.add_argument("--embed-model", default="doubao/doubao-embedding-vision-251215")
    ap.add_argument("--image-model", default="doubao/doubao-seedream-4-5-251128")
    ap.add_argument("--video-model", default="doubao/doubao-seedance-2-0-260128")
    ap.add_argument("--poll-timeout", type=int, default=420)
    ap.add_argument("--include-video", action="store_true",
                    help="执行 R08/R13/R12 视频闭环（真实计费，约 5 分钟）")
    ap.add_argument("--include-image", action="store_true",
                    help="执行 R36 图片生成（真实计费）")
    args = ap.parse_args()
    if not args.key:
        print("缺少 API Key：用 --key 或环境变量 ATH_API_KEY 传入（取 key 命令见文件头注释）")
        sys.exit(2)
    base = args.base

    print("服务：%s" % base)
    all_cases = []

    # 先取模型清单（P3 各用例依赖）
    st, body = req(base, "/v1/models", args.key)
    if st != 200 or not isinstance(body, dict) or not body.get("data"):
        print("无法获取 /v1/models（%s），终止" % st)
        sys.exit(2)
    models = body["data"]
    print("模型清单获取成功（%d 个）\n" % len(models))

    print("== A. 共享管线正向（P4 防守）==")
    for c in section_a_pipeline(base, args.key, args.chat_model,
                                args.messages_model, args.responses_model):
        print("\n[%s] %s" % (c.cid, c.title))
        c.report()
        all_cases.append(c)

    print("\n== B. P3 能力元数据 ==")
    for c in section_b_models(models, base, args.key):
        print("\n[%s] %s" % (c.cid, c.title))
        c.report()
        all_cases.append(c)

    print("\n== C. P4 错误信封矩阵 ==")
    for c in section_c_errors(base, args.key, args.chat_model):
        print("\n[%s] %s" % (c.cid, c.title))
        c.report()
        all_cases.append(c)

    print("\n== D. P1 embeddings 专项 ==")
    for c in section_d_embeddings(base, args.key, args.embed_model):
        print("\n[%s] %s" % (c.cid, c.title))
        c.report()
        all_cases.append(c)

    print("\n== E. P2 校验与错误（免费部分）==")
    for c in section_e_videos_free(base, args.key, args.video_model):
        print("\n[%s] %s" % (c.cid, c.title))
        c.report()
        all_cases.append(c)

    if args.include_video:
        print("\n== F1. P2 视频闭环（计费）==")
        for c in section_f_video_paid(base, args.key, args.video_model, args.poll_timeout):
            print("\n[%s] %s" % (c.cid, c.title))
            c.report()
            all_cases.append(c)
    else:
        c = Case("R08/R13/R12", "视频闭环（prompt-only 创建 + running 查询 + 终态别名）")
        c.skip("真实计费用例，--include-video 开启")
        print("\n[R08/R13/R12] %s" % c.title)
        c.report()
        all_cases.append(c)

    if args.include_image:
        print("\n== F2. images（计费）==")
        for c in section_f_image_paid(base, args.key, args.image_model):
            print("\n[%s] %s" % (c.cid, c.title))
            c.report()
            all_cases.append(c)
    else:
        c = Case("R36", "images 生成")
        c.skip("真实计费用例，--include-image 开启")
        print("\n[R36] %s" % c.title)
        c.report()
        all_cases.append(c)

    ok = summary_print(all_cases)
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
