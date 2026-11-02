package service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/your-org/ai-transit-hub/internal/config"
	"github.com/your-org/ai-transit-hub/internal/models"
)

// Node 环境报错的集成测试：跨层走真实调用链（frontmatter 声明 → precheck 收集 →
// 任务环境构建 → 沙箱子进程注入 → 生命周期清理），外部二进制（uv/npm/node）以
// fake shell 脚本仿真，不依赖宿主机是否安装 Node.js、不访问网络。

// fakeUV 仿真 uv：`venv --python <ver> <dir>` 时建最小 venv 骨架（bin/python3 存在即视为就绪），
// `python install` / `pip install` 一律 no-op 成功。脚本内恢复基础 PATH（同 fakeNpmInstall 的原因）
const fakeUV = `#!/bin/sh
PATH=/usr/bin:/bin
export PATH
if [ "$1" = "venv" ]; then
  for last; do :; done
  mkdir -p "$last/bin"
  : > "$last/bin/python3"
  chmod +x "$last/bin/python3"
fi
exit 0
`

// installFakeUV 把 fake uv 放到 envRoot/_uv/uv：ensureUV 探测到该路径存在且可执行
// 即直接采用，跳过网络引导（ensureUV 的离线缓存路径）
func installFakeUV(t *testing.T, m *PyEnvManager) {
	t.Helper()
	dir := filepath.Join(m.envRoot, uvToolsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("prepare _uv dir: %v", err)
	}
	fakeBin(t, dir, "uv", fakeUV)
}

// skillWithDeps 构造带 dependencies 声明的技能（frontmatter 顶层声明形式）
func skillWithDeps(name string, deps ...string) models.Skill {
	fm := `{"dependencies":[`
	for i, d := range deps {
		if i > 0 {
			fm += ","
		}
		fm += `"` + d + `"`
	}
	return models.Skill{Name: name, Frontmatter: models.JSON(fm + `]}`)}
}

// TestPrecheckIntegrationBuildsTaskNodeEnv 全链路 happy path：npm:/pip:/python3 混合声明
// 经 precheck 收集后，任务 venv 与任务级 node_modules 都被真实构建。宿主无 node，
// npm: 声明走自托管运行时（installFakeNode 铺设的引导产物）完成安装
func TestPrecheckIntegrationBuildsTaskNodeEnv(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t)) // 宿主无 node/npm，不参与任何环节

	m := newTestPyEnvManager(t, "")
	installFakeUV(t, m)
	installFakeNode(t, m)
	r := &AgentRuntime{pyenv: m}

	skills := []models.Skill{skillWithDeps("fetcher", "npm:typescript", "pip:requests", "python3")}
	if err := r.precheckSkillDependencies(context.Background(), "task-1", skills); err != nil {
		t.Fatalf("precheck failed: %v", err)
	}
	// 任务 venv：fake uv 已建骨架（bin/python3 存在）
	if _, err := os.Stat(filepath.Join(m.TaskVenvDir("task-1"), "bin", "python3")); err != nil {
		t.Errorf("task venv not built: %v", err)
	}
	// 任务 Node 环境：fake npm 已建 node_modules 与预置 package.json
	if _, err := os.Stat(m.TaskNodeModulesDir("task-1")); err != nil {
		t.Errorf("task node_modules not built: %v", err)
	}
	pj, err := os.ReadFile(filepath.Join(m.TaskNodeEnvDir("task-1"), "package.json"))
	if err != nil {
		t.Fatalf("package.json not prepared: %v", err)
	}
	if got := string(pj); got != "{}\n" {
		t.Errorf("package.json = %q, want {}", got)
	}
}

// TestPrecheckIntegrationNodeBootstrapFailure 宿主无 node 且下载引导不可达：venv 构建
// 成功后 Node 环境构建失败，报错跨层串联不丢信息（runtime 层 task node environment
// build failed → pyenv 层 node runtime bootstrap failed → download 上下文）
func TestPrecheckIntegrationNodeBootstrapFailure(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t)) // 无 node/npm；fake uv 不经 PATH 寻址，不受影响

	m := newTestPyEnvManager(t, "")
	m.cfg.NodeDownloadURL = "http://127.0.0.1:0/node-v22.20.0-test.tar.gz" // 不可达：立即 connection refused
	installFakeUV(t, m)
	r := &AgentRuntime{pyenv: m}

	skills := []models.Skill{skillWithDeps("fetcher", "npm:typescript")}
	err := r.precheckSkillDependencies(context.Background(), "task-1", skills)
	if err == nil {
		t.Fatal("expected error when node bootstrap fails")
	}
	assertContains(t, err.Error(), "task node environment build failed")
	assertContains(t, err.Error(), "node runtime bootstrap failed")
	assertContains(t, err.Error(), "download")
	// 构建顺序证据：venv 先建成功，失败发生在其后的 Node 环境构建
	if _, serr := os.Stat(filepath.Join(m.TaskVenvDir("task-1"), "bin", "python3")); serr != nil {
		t.Errorf("task venv should be built before node env failure: %v", serr)
	}
}

// TestPrecheckIntegrationInvalidNpmDeclaration 注入防护声明（"-" 开头）触发 fail-fast：
// 报错点名技能与声明，且完全不触发任务环境构建（venv 与 _node 目录都不出现）
func TestPrecheckIntegrationInvalidNpmDeclaration(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t))

	m := newTestPyEnvManager(t, "")
	installFakeUV(t, m)
	r := &AgentRuntime{pyenv: m}

	skills := []models.Skill{skillWithDeps("evil", "npm:--force")}
	err := r.precheckSkillDependencies(context.Background(), "task-1", skills)
	if err == nil {
		t.Fatal("expected error for invalid npm declaration")
	}
	assertContains(t, err.Error(), `skill "evil" requires "npm:--force"`)
	assertContains(t, err.Error(), "invalid npm package name")
	for _, dir := range []string{m.TaskVenvDir("task-1"), m.TaskNodeEnvDir("task-1")} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("task env %s must not be created on precheck failure", dir)
		}
	}
}

// fakeVersionedNode 覆写自托管 bin 里的 node：--version 输出真实版本号供约束实测比对，
// 其余调用沿用 installFakeNode 的 PATH 回显行为（PATH 里可能含数字，版本探测不能依赖它）
const fakeVersionedNode = `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo v22.20.0
  exit 0
fi
echo "PATH=$PATH"
exit 0
`

// TestPrecheckIntegrationBareNodeDeclaration 宿主无 node + 裸 "node >= 18" 声明：由三级
// 引导的自托管运行时兜底满足（不再误报 "not available on host"），版本约束与引导产物
// 实测比对（fake node 输出 v22.20.0 ≥ 18）
func TestPrecheckIntegrationBareNodeDeclaration(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t)) // 宿主无 node/npm，不参与任何环节

	m := newTestPyEnvManager(t, "")
	installFakeUV(t, m) // precheck 无 pip 依赖也会构建任务 venv（ensureUV 引导）
	binDir := installFakeNode(t, m)
	fakeBin(t, binDir, "node", fakeVersionedNode)
	r := &AgentRuntime{pyenv: m}

	skills := []models.Skill{skillWithDeps("designer", "node >= 18")}
	if err := r.precheckSkillDependencies(context.Background(), "task-1", skills); err != nil {
		t.Fatalf("precheck failed: %v", err)
	}
	if m.NodeRuntimeBinDir() == "" {
		t.Error("self-hosted node runtime should be resolved by precheck")
	}
}

// TestPrecheckIntegrationBareNodeVersionMismatch 引导产物版本不满足约束时拦截：
// "node < 18" 对实测 v22.20.0 报 does not satisfy（点名技能与声明）
func TestPrecheckIntegrationBareNodeVersionMismatch(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t))

	m := newTestPyEnvManager(t, "")
	binDir := installFakeNode(t, m)
	fakeBin(t, binDir, "node", fakeVersionedNode)
	r := &AgentRuntime{pyenv: m}

	skills := []models.Skill{skillWithDeps("designer", "node < 18")}
	err := r.precheckSkillDependencies(context.Background(), "task-1", skills)
	if err == nil {
		t.Fatal("expected error when managed node does not satisfy constraint")
	}
	assertContains(t, err.Error(), `skill "designer" requires "node < 18"`)
	assertContains(t, err.Error(), "does not satisfy")
}

// TestPrecheckIntegrationBareNodeBootstrapFailure 宿主无 node、无自托管缓存且下载不可达：
// 裸 node 声明的报错携带引导上下文（不再是笼统的 "not available on host"）
func TestPrecheckIntegrationBareNodeBootstrapFailure(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t))

	m := newTestPyEnvManager(t, "")
	m.cfg.NodeDownloadURL = "http://127.0.0.1:0/node-v22.20.0-test.tar.gz" // 不可达：立即 connection refused
	r := &AgentRuntime{pyenv: m}

	skills := []models.Skill{skillWithDeps("designer", "node >= 18")}
	err := r.precheckSkillDependencies(context.Background(), "task-1", skills)
	if err == nil {
		t.Fatal("expected error when node bootstrap fails")
	}
	assertContains(t, err.Error(), `skill "designer" requires "node >= 18"`)
	assertContains(t, err.Error(), "node runtime bootstrap failed")
}

// TestPrecheckIntegrationSystemMode system 模式（pyenv=nil）不构建任务环境，
// npm: 声明回落到宿主 npm 全局校验，报错附 venv 模式出路
func TestPrecheckIntegrationSystemMode(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t)) // 宿主无 npm

	r := &AgentRuntime{} // system 模式
	skills := []models.Skill{skillWithDeps("fetcher", "npm:typescript")}
	err := r.precheckSkillDependencies(context.Background(), "task-1", skills)
	if err == nil {
		t.Fatal("expected error when npm is missing in system mode")
	}
	assertContains(t, err.Error(), `skill "fetcher" requires "npm:typescript"`)
	assertContains(t, err.Error(), `cannot verify npm package "typescript"`)
	assertContains(t, err.Error(), "npm not available on host")
	assertContains(t, err.Error(), "AGENT_PY_ENV_MODE=venv")
}

// TestPlainSandboxInjectsNodeEnv 执行层注入链路：NodeEnvDir 经 buildScriptEnv 进入真实
// 子进程环境——NODE_PATH 指向任务 node_modules，PATH 以 node_modules/.bin 起头
func TestPlainSandboxInjectsNodeEnv(t *testing.T) {
	bin := fakeBinDir(t)
	fakeBin(t, bin, "node", "#!/bin/sh\necho \"NODE_PATH=$NODE_PATH\"\necho \"PATH=$PATH\"\n")
	t.Setenv("PATH", bin)

	m := newTestPyEnvManager(t, "")
	wsDir := t.TempDir()
	spec := ExecSpec{
		Interpreter: "node",
		Script:      filepath.Join(wsDir, "dump.js"), // fake node 不读内容
		WorkDir:     wsDir,
		NodeEnvDir:  m.TaskNodeEnvDir("task-1"),
	}
	out := plainSandbox{}.Exec(context.Background(), spec)
	nmDir := m.TaskNodeModulesDir("task-1")
	assertContains(t, out, "NODE_PATH="+nmDir)
	assertContains(t, out, "PATH="+filepath.Join(nmDir, ".bin")+":")
	assertContains(t, out, "exit code: 0")
}

// TestPlainSandboxSelfHostedNode 宿主无 node + 自托管运行时已引导：plain 后端把解释器
// 解析到 <envRoot>/_node/bin/node（resolveInterpreter 自托管回退），且 buildScriptEnv
// 把其 bin 前置 PATH（自托管优先于注入的 node_modules/.bin）
func TestPlainSandboxSelfHostedNode(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t)) // 宿主无 node：强制自托管回退

	m := newTestPyEnvManager(t, "")
	nodeBin := installFakeNode(t, m)
	wsDir := t.TempDir()
	spec := ExecSpec{
		Interpreter: "node",
		Script:      filepath.Join(wsDir, "dump.js"), // fake node 不读内容
		WorkDir:     wsDir,
		NodeEnvDir:  m.TaskNodeEnvDir("task-1"),
	}
	out := plainSandbox{}.Exec(context.Background(), spec)
	assertContains(t, out, "PATH="+nodeBin+":")
	assertContains(t, out, "exit code: 0")
}

// TestSeatbeltNodeEnvReadExemption 沙箱豁免链路（仅 darwin + sandbox-exec 可用时）：
// 生产布局（envRoot 与任务工作区同父目录）下，脚本经真实 sandbox-exec 可读任务
// node_modules（envRoot 读豁免生效），兄弟工作区仍被屏蔽（任务间隔离不受豁免影响）
func TestSeatbeltNodeEnvReadExemption(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("seatbelt backend only available on darwin")
	}
	// 先以宿主 PATH 解析 sandbox-exec 绝对路径（t.Setenv 换掉 PATH 后 Exec 内的
	// LookPath 就找不到真二进制了），在 fake bin 里放 exec 转发 wrapper 兜住两次探测
	sbExec, err := exec.LookPath("sandbox-exec")
	if err != nil {
		t.Skip("sandbox-exec not available on this host")
	}
	bin := fakeBinDir(t)
	fakeBin(t, bin, "sandbox-exec", "#!/bin/sh\nexec "+sbExec+" \"$@\"\n")
	fakeBin(t, bin, "node", `#!/bin/sh
PATH=/usr/bin:/bin
export PATH
echo "PROBE=$(cat "$NODE_PATH/.probe")"
if cat "$NODEENV_TEST_SIBLING/secret" >/dev/null 2>&1; then
  echo "SIBLING_READ_OK"
else
  echo "SIBLING_READ_DENIED"
fi
`)
	t.Setenv("PATH", bin)

	root := t.TempDir() // workspaceRoot：任务工作区与 envRoot 的公共父目录（生产布局）
	wsDir := filepath.Join(root, "task-1")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatalf("prepare ws dir: %v", err)
	}
	sibling := filepath.Join(root, "task-2")
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatalf("prepare sibling dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sibling, "secret"), []byte("TOP_SECRET"), 0o644); err != nil {
		t.Fatalf("prepare sibling file: %v", err)
	}

	m := &PyEnvManager{cfg: config.PyEnvConfig{InstallTimeout: 30 * time.Second}, envRoot: filepath.Join(root, "_pyenvs")}
	nodeEnvDir := m.TaskNodeEnvDir("task-1")
	if err := os.MkdirAll(m.TaskNodeModulesDir("task-1"), 0o755); err != nil {
		t.Fatalf("prepare node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(m.TaskNodeModulesDir("task-1"), ".probe"), []byte("PROBE_OK"), 0o644); err != nil {
		t.Fatalf("prepare probe file: %v", err)
	}

	// 传 sibling 路径给脚本（变量名不含敏感词，sanitizeEnv 放行）
	t.Setenv("NODEENV_TEST_SIBLING", sibling)

	out := (seatbeltSandbox{}).Exec(context.Background(), ExecSpec{
		Interpreter: "node",
		Script:      filepath.Join(wsDir, "dump.js"),
		WorkDir:     wsDir,
		NodeEnvDir:  nodeEnvDir,
	})
	assertContains(t, out, "PROBE=PROBE_OK")      // envRoot 读豁免生效：node_modules 可读
	assertContains(t, out, "SIBLING_READ_DENIED") // 兄弟工作区屏蔽不因豁免失效
	assertNotContains(t, out, "TOP_SECRET")
}

// TestSeatbeltSelfHostedNodeExecutable 沙箱执行自托管运行时（仅 darwin + sandbox-exec
// 可用时）：宿主无 node 时解释器解析到 <envRoot>/_node/bin/node，经真实 sandbox-exec
// 可执行（envRoot 读豁免覆盖引导产物），PATH 注入同样生效
func TestSeatbeltSelfHostedNodeExecutable(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("seatbelt backend only available on darwin")
	}
	// 先以宿主 PATH 解析 sandbox-exec 绝对路径（t.Setenv 换掉 PATH 后 Exec 内的
	// LookPath 就找不到真二进制了），在 fake bin 里放 exec 转发 wrapper 兜住两次探测
	sbExec, err := exec.LookPath("sandbox-exec")
	if err != nil {
		t.Skip("sandbox-exec not available on this host")
	}
	bin := fakeBinDir(t)
	fakeBin(t, bin, "sandbox-exec", "#!/bin/sh\nexec "+sbExec+" \"$@\"\n")
	t.Setenv("PATH", bin) // 宿主无 node：强制自托管回退

	root := t.TempDir() // workspaceRoot：任务工作区与 envRoot 的公共父目录（生产布局）
	wsDir := filepath.Join(root, "task-1")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatalf("prepare ws dir: %v", err)
	}
	m := &PyEnvManager{cfg: config.PyEnvConfig{InstallTimeout: 30 * time.Second}, envRoot: filepath.Join(root, "_pyenvs")}
	nodeBin := installFakeNode(t, m)

	out := (seatbeltSandbox{}).Exec(context.Background(), ExecSpec{
		Interpreter: "node",
		Script:      filepath.Join(wsDir, "dump.js"),
		WorkDir:     wsDir,
		NodeEnvDir:  m.TaskNodeEnvDir("task-1"),
	})
	assertContains(t, out, "PATH="+nodeBin+":") // envRoot 豁免内执行 + 注入前缀可见
	assertContains(t, out, "exit code: 0")
}

// TestRemoveTaskEnvLifecycle 生命周期收尾：任务 venv 与 Node 环境建好后，
// RemoveTaskEnv 一次清掉两个目录（RunTask defer 依赖的清理语义）
func TestRemoveTaskEnvLifecycle(t *testing.T) {
	bin := fakeBinDir(t)
	fakeBin(t, bin, "node", "#!/bin/sh\nexit 0\n")
	fakeBin(t, bin, "npm", fakeNpmInstall)
	t.Setenv("PATH", bin)

	m := newTestPyEnvManager(t, "")
	installFakeUV(t, m)
	ctx := context.Background()
	if _, err := m.EnsureTaskVenv(ctx, "task-1", []string{"requests"}); err != nil {
		t.Fatalf("ensure venv: %v", err)
	}
	if _, err := m.EnsureTaskNodeEnv(ctx, "task-1", []string{"typescript"}); err != nil {
		t.Fatalf("ensure node env: %v", err)
	}
	m.RemoveTaskEnv("task-1")
	for _, dir := range []string{m.TaskVenvDir("task-1"), m.TaskNodeEnvDir("task-1")} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("RemoveTaskEnv should delete %s", dir)
		}
	}
}
