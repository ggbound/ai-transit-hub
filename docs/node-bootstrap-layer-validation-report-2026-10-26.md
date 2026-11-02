# Node 自动引导层验证总结报告

- 日期：2026-10-26
- 范围：backend Node 自动引导层（uv 引导模式的 Node 对应实现），任务编号 nb-1 ~ nb-6
- 结论：**三级引导策略全部验证通过，回归全绿，现场已恢复**

---

## 1. 背景与目标

为解决服务器未安装 Node.js 时 agent 脚本任务无法执行的问题，按 uv（CPython）引导模式为 Node.js 实现同构的自动引导层，三级确定策略：

```
① 自托管磁盘缓存 <envRoot>/_pyenvs/_node   （引导一次，后续零下载）
            ↓ 未命中
② 宿主 PATH（node + npm 双全才可用）        （有宿主 node 时零成本回退）
            ↓ 不可用
③ 官方 tarball 下载 + SHA256 校验 + 原子落盘 （fail-closed：失败零残留）
```

## 2. 实现范围（nb-1 ~ nb-4）

| 文件 | 内容 |
|---|---|
| [config.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/config/config.go#L80-L88) | `NodeVersion`（默认 22.20.0）/ `NodeDownloadURL`（默认空=官方）/ `NodeSHA256`（默认空=拉 SHASUMS256.txt）；环境变量 `AGENT_NODE_VERSION` / `AGENT_NODE_DOWNLOAD_URL` / `AGENT_NODE_SHA256` |
| [agent_pyenv.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/service/agent_pyenv.go#L423-L480) | `downloadNode`：官方 URL `https://nodejs.org/dist/v%s/node-v%s-%s.tar.gz`；SHA 缺省取 `<URL 目录>/SHASUMS256.txt`；mismatch 报 `sha256 mismatch`；**tmpDir 建在 SHA 校验之后**（失败零残留）；引导标记日志 `[PyEnv] Node.js %s bootstrapped at %s` |
| [agent_pyenv.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/service/agent_pyenv.go#L109-L164) | 整个环境构建由 `context.WithTimeout(ctx, m.cfg.InstallTimeout)`（默认 5m）包裹 |
| [agent_sandbox.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/service/agent_sandbox.go#L160) | `resolveInterpreter` 自托管分支：Stat 校验 `_node/bin/node`（常规文件 + 可执行位）并返回其绝对路径；不可用时 fail-closed 文案（引导改写为 Python 或请求用户安装） |

安全设计要点：

- 下载缓冲全部在内存，SHA256 校验失败/超时不落任何目录；
- 解压采用安全路径检查 + 原子 rename，无竞态窗口；
- 缓存命中要求 node 二进制可执行，npm/corepack 以相对 symlink 保留。

## 3. 单元/集成测试与回归（nb-5）

新增/改造测试：[agent_pyenv_test.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/service/agent_pyenv_test.go)、[agent_sandbox_test.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/service/agent_sandbox_test.go)、[agent_nodeenv_integration_test.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/service/agent_nodeenv_integration_test.go)。

覆盖用例：

- 缓存命中（含 symlink 保留校验）；
- 宿主回退成功 / 宿主缺失 fail-closed；
- 下载引导成功（httptest 仿真发行包 + `SHASUMS256.txt`，行格式 `<hex>␣␣<filename>`）；
- SHA mismatch fail-closed（下载 URL 指向 `127.0.0.1:0` 立即拒连 / 校验和不符）；
- 下载隔离：测试内注入断言通过 fake npm 的 `.path-dump` / `.env-dump` 落盘文件回读。

测试开发过程中修复 5 处问题（4 处测试侧 + 1 处实现缺陷）：

1. **import 笔误**：`archive/gzip` 不存在于 Go 标准库，正确包为 `compress/gzip`（此前数轮误诊为"工具链 zip 损坏"，实为本项）；
2. **实现缺陷**：`resolveInterpreter` 自托管分支原返回 bin **目录**，fork/exec 报 Permission denied → 改为返回 `bin/node` 绝对路径（Stat 校验常规文件 + 可执行位）；
3. `buildTarGz` 未给 TypeReg 设 `hdr.Size` → `archive/tar: write too long`；
4. `os.WriteFile` 对已存在文件不改变权限（O_CREATE 仅创建时应用 perm）→ 补显式 `os.Chmod`；
5. fake npm 脚本开头重置 PATH 冲掉注入断言 → 改为先用 shell 内建 `echo` 落盘 `.path-dump` 再重置 PATH。

回归结果：`go build + go vet + go test ./...` 全绿（`GOTOOLCHAIN=go1.26.9` 本地覆盖，go.mod 要求 1.25.0、本机 ServBay go 1.24.13，不改 go.mod）。

## 4. E2E 验证（nb-6）

E2E 模式：真实 API `POST /v1/agent/dispatch` → 轮询 `GET /v1/agent/tasks/:id` → 校验 `agent_task_steps` 落库 + `[PyEnv]` 日志。测试 skill：13 `nodeenv-e2e`（依赖 npm:dayjs）。

### 场景 A：宿主有 node —— 回退不回归

宿主 node：`/Applications/ServBay/script/alias/node` = **v22.23.2**。任务 agt_20261026135805_c07e2fde1bc45bc5：

- 派发 13:58:05 → env ready 13:58:07（**2 秒就绪**，走宿主回退，无下载）；
- dayjs 安装到任务级 env（`_pyenvs/agt_..._node`），脚本 happy path，任务 completed；
- 日志无 `runtime bootstrap failed`，`_pyenvs` 无 `_node` 残留（宿主可用时第三级不触发）。

### 场景 B：宿主无 node —— 降级链路

服务进程以剔除 node 的 PATH（`/usr/bin:/bin:/usr/sbin:/sbin`）启动，强制触发第三级；`AGENT_NODE_DOWNLOAD_URL` 切 npmmirror 镜像绕开 nodejs.org 慢速网络。

**B1 第三级下载引导成功**（任务 agt_20261026141014_c2cc83c7b5c1621c，26 秒完成引导）：

```
14:10:14 [PyEnv] managed CPython 3.12 ready
14:10:40 [PyEnv] Node.js 22.20.0 bootstrapped at .../storage/agent_tasks/_pyenvs/_node/bin
14:10:45 [PyEnv] task node env ready at .../agt_20261026141014_c2cc83c7b5c1621c_node (1 npm packages)
14:10:55 [PyEnv] removed task env .../agt_20261026141014_c2cc83c7b5c1621c[_node]   ← 任务结束自动清理
```

脚本落库输出 `node: v22.20.0`（宿主为 v22.23.2，**版本差异证实执行的是自托管 node**），dayjs 正常安装，任务 completed。

**B2 `_node` 缓存内容校验**：111MB node 二进制（常规文件 + 可执行位），npm/corepack 相对 symlink 保留，直跑 `bin/node -v` = v22.20.0。

**B3 L1 磁盘缓存命中**（任务 agt_20261026141134_85808d3014b23c94）：

```
14:11:34 [PyEnv] task node env ready at .../agt_20261026141134_85808d3014b23c94_node (1 npm packages)
```

**无新 bootstrapped 日志**（零下载），env 与派发同秒就绪，输出同样 `node: v22.20.0`，任务 completed。

**B4 官方源超时 fail-closed**（任务 agt_20261026140400_f39db7e7ae8c454f）：

默认 nodejs.org 在当前网络下载 25MB 过慢，触发 5m `InstallTimeout`，任务 failed，报错链精确：

```
14:09:01 [ERROR] [AgentTask] agt_20261026140400_f39db7e7ae8c454f failed at iteration 0:
  task node environment build failed: node runtime bootstrap failed:
  download https://nodejs.org/dist/v22.20.0/node-v22.20.0-darwin-arm64.tar.gz: context deadline exceeded
```

`_pyenvs` 零残留（下载缓冲在内存、tmpDir 建在 SHA 校验之后）。**缓解手段即为该场景设计的 `AGENT_NODE_DOWNLOAD_URL` 镜像配置**——切 npmmirror 后 26 秒引导成功。

## 5. 现场恢复

- skill 13/14（nodeenv-e2e / nodeenv-e2e-bad）已恢复 `enabled=0`；
- 降级服务（无 node PATH + npmmirror）已停止，服务已用正常 PATH 重启（宿主 node v22.23.2 回到可见状态）；
- `_node` 缓存已清理，`_pyenvs` 恢复原始状态（仅 `_npm-cache`、`_uv`）。

## 6. 结论与运维建议

| 验证项 | 结果 |
|---|---|
| 三级策略：缓存 → 宿主 → 下载 | 通过 |
| 宿主回退不回归（零下载、2s 就绪） | 通过 |
| 第三级下载引导（SHA256 校验 + 原子落盘） | 通过（npmmirror 26s） |
| L1 磁盘缓存二次命中（零下载） | 通过 |
| 官方源超时 fail-closed（报错链精确、零残留） | 通过 |
| 单元/集成测试回归 | 全绿 |

运维建议：

- 服务器无宿主 node 或网络访问 nodejs.org 受限时，在 `.env` 配置 `AGENT_NODE_DOWNLOAD_URL` 指向镜像（如 `https://npmmirror.com/mirrors/node`）与可选的 `AGENT_NODE_SHA256` 固定校验和；
- 首次引导成功后 `_node` 缓存常驻（约 111MB），后续任务零下载；如需强制重新引导，删除 `storage/agent_tasks/_pyenvs/_node` 即可；
- 引导超时上限由 `AGENT_PY_INSTALL_TIMEOUT`（默认 5m）控制。

## 7. 经验备忘

- Go 标准库 gzip 包为 `compress/gzip`（无 `archive/gzip`）；
- `os.WriteFile` 对已存在文件不改权限，需显式 `Chmod`；
- `tar.Writer` 按 `hdr.Size` 校验写入长度，TypeReg 必须先设 Size（symlink/dir 保持 0）；
- E2E 模拟"宿主无 node"：以裁剪 PATH 启动服务进程即可，无需改容器/系统环境；
- E2E 重建二进制后必须重启服务，避免"旧二进制"造成假阴性/假阳性。
