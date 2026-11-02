package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/your-org/ai-transit-hub/internal/config"
	"github.com/your-org/ai-transit-hub/internal/logger"
)

// 任务级执行环境隔离（AGENT_PY_ENV_MODE=venv）：
//
//	Python：宿主侧（可联网）uv 工具链引导 → 托管 CPython 安装 → 按任务构建 venv + 装依赖，
//	解释器指向 <envRoot>/<taskID>/bin/python3，与系统 Python 完全隔离。
//	Node：npm: 依赖经 npm 安装到 <envRoot>/<taskID>_node/node_modules；node/npm 运行时三级确定：
//	自托管缓存（<envRoot>/_node）→ 宿主 PATH → 官方发行包自动下载引导（sha256 校验），
//	宿主机零 Node 依赖。
//
// 沙箱侧（禁网）：两类任务环境目录经文件系统只读进入（darwin readDenyClauses 豁免 / linux --ro-bind）。
// 所有任务环境集中在 envRoot（默认 <AgentTaskPath>/_pyenvs，工作区外）：
// agent 不可写执行环境；TTL 清理跳过 "_" 前缀工具链目录。
const (
	pyEnvModeVenv       = "venv"
	uvToolsDir          = "_uv"
	npmCacheDir         = "_npm-cache"
	nodeEnvSuffix       = "_node"
	nodeToolsDir        = "_node" // 自托管 Node.js 运行时目录（<envRoot>/_node/bin/node）
	maxUVArchiveBytes   = 256 << 20
	maxNodeArchiveBytes = 256 << 20
	// defaultNodeVersion 自托管 Node.js 版本兜底（正常路径来自 AGENT_NODE_VERSION 配置）
	defaultNodeVersion = "22.20.0"
)

// PyEnvManager 任务执行环境管理器（venv 模式：Python venv + 任务级 node_modules）。nil = system 模式（宿主系统 python3）
type PyEnvManager struct {
	cfg     config.PyEnvConfig
	envRoot string // 任务环境根（<AgentTaskPath>/_pyenvs 或 AGENT_PY_ENV_ROOT）

	mu         sync.Mutex // 串行化 uv / CPython / Node 引导（进程级一次性开销）
	uvPath     string     // uv 二进制绝对路径（成功后缓存；失败不缓存，可重试）
	pyOK       bool       // 托管 CPython 已就绪
	nodeBinDir string     // 自托管 Node bin 目录（成功后缓存；"" = 沿用宿主 PATH 或未就绪）
}

// NewPyEnvManager 创建管理器；Mode != venv 时返回 nil（system 模式，维持历史行为）
func NewPyEnvManager(cfg config.PyEnvConfig, defaultEnvRoot string) *PyEnvManager {
	if cfg.Mode != pyEnvModeVenv {
		return nil
	}
	envRoot := cfg.EnvRoot
	if envRoot == "" {
		envRoot = filepath.Join(defaultEnvRoot, "_pyenvs")
	}
	// 与沙箱豁免比较保持同一形式：resolvedPath 基于绝对路径，此处先 Abs 化
	if abs, err := filepath.Abs(envRoot); err == nil {
		envRoot = abs
	}
	return &PyEnvManager{cfg: cfg, envRoot: envRoot}
}

// TaskVenvDir 任务专属 venv 目录
func (m *PyEnvManager) TaskVenvDir(taskID string) string {
	return filepath.Join(m.envRoot, taskID)
}

// TaskPythonPath 任务 venv 的 python3 解释器绝对路径
func (m *PyEnvManager) TaskPythonPath(taskID string) string {
	return filepath.Join(m.TaskVenvDir(taskID), "bin", "python3")
}

// TaskNodeEnvDir 任务专属 Node 环境目录（node_modules 的父目录）。
// 独立于 venv 的兄弟目录：<taskID>_node——uv venv 重建时会清空既有目录，
// node_modules 嵌套 venv 内会被连带删除，故平级存放
func (m *PyEnvManager) TaskNodeEnvDir(taskID string) string {
	return filepath.Join(m.envRoot, taskID+nodeEnvSuffix)
}

// TaskNodeModulesDir 任务 node_modules 目录（npm: 依赖安装位置）
func (m *PyEnvManager) TaskNodeModulesDir(taskID string) string {
	return filepath.Join(m.TaskNodeEnvDir(taskID), "node_modules")
}

// EnsureTaskNodeEnv 确保任务 Node 环境就绪并安装 npm 依赖（幂等：node_modules 已含依赖时
// npm install 为 no-op）。node/npm 运行时三级确定（自托管 → 宿主 PATH → 自动下载引导），
// 依赖经宿主侧安装后由沙箱只读挂载进入。返回任务 Node 环境目录。
func (m *PyEnvManager) EnsureTaskNodeEnv(ctx context.Context, taskID string, pkgs []string) (string, error) {
	if err := validTaskEnvID(taskID); err != nil {
		return "", err
	}
	envDir := m.TaskNodeEnvDir(taskID)
	if len(pkgs) == 0 {
		return envDir, nil
	}
	ctx, cancel := context.WithTimeout(ctx, m.cfg.InstallTimeout)
	defer cancel()

	// node/npm 运行时：envRoot/_node 自托管缓存 → 宿主 PATH → 官方发行包下载引导（uv 同款三级策略）
	nodeBinDir, err := m.ensureNode(ctx)
	if err != nil {
		return "", fmt.Errorf("node runtime bootstrap failed: %w", err)
	}
	npmBin := "npm"
	if nodeBinDir != "" {
		npmBin = filepath.Join(nodeBinDir, "npm")
	}

	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return "", err
	}
	// 预置最小 package.json：无论 npm 版本，install 都按本地包确定性落盘到 node_modules
	pj := filepath.Join(envDir, "package.json")
	if _, err := os.Stat(pj); err != nil {
		if err := os.WriteFile(pj, []byte("{}\n"), 0o644); err != nil {
			return "", err
		}
	}
	cmd := exec.CommandContext(ctx, npmBin, append([]string{
		"install", "--prefix", envDir, "--no-audit", "--no-fund", "--loglevel=error",
	}, pkgs...)...)
	cmd.Env = m.npmEnv(nodeBinDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("install npm packages %v: %w: %s", pkgs, err, tailOf(string(out), 400))
	}
	logger.Info("[PyEnv] task node env ready at %s (%d npm packages)", envDir, len(pkgs))
	return envDir, nil
}

// npmEnv npm 安装进程环境：缓存收敛到 envRoot 下（"_npm-cache"，"_" 前缀天然豁免 TTL 清理）；
// 使用自托管 Node 时将其 bin 目录前置 PATH（npm 脚本运行需要同目录的 node）
func (m *PyEnvManager) npmEnv(nodeBinDir string) []string {
	env := os.Environ()
	if nodeBinDir != "" {
		env = setEnvVar(env, "PATH", nodeBinDir+":"+os.Getenv("PATH"))
	}
	env = setEnvVar(env, "npm_config_cache", filepath.Join(m.envRoot, npmCacheDir))
	env = setEnvVar(env, "npm_config_update_notifier", "false")
	if m.cfg.NpmRegistry != "" {
		env = setEnvVar(env, "npm_config_registry", m.cfg.NpmRegistry)
	}
	return env
}

// EnsureTaskVenv 确保任务 venv 就绪并安装依赖（幂等：解释器已存在则跳过创建，
// 依赖重复安装对 uv pip 是 no-op）。返回 venv 目录。
func (m *PyEnvManager) EnsureTaskVenv(ctx context.Context, taskID string, pkgs []string) (string, error) {
	if err := validTaskEnvID(taskID); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, m.cfg.InstallTimeout)
	defer cancel()

	if _, err := m.ensureUV(ctx); err != nil {
		return "", fmt.Errorf("bootstrap uv: %w", err)
	}
	if err := m.ensureManagedPython(ctx); err != nil {
		return "", fmt.Errorf("install managed CPython %s: %w", m.cfg.PythonVersion, err)
	}

	venvDir := m.TaskVenvDir(taskID)
	if _, err := os.Stat(filepath.Join(venvDir, "bin", "python3")); err != nil {
		// 不存在或残缺则（重）建；不依赖 uv venv 对已存在目录的行为，运行时自控幂等
		if out, err := m.runUV(ctx, "venv", "--python", m.cfg.PythonVersion, venvDir); err != nil {
			return "", fmt.Errorf("uv venv: %w: %s", err, tailOf(string(out), 400))
		}
	}
	if err := m.installPkgs(ctx, venvDir, pkgs); err != nil {
		return "", err
	}
	return venvDir, nil
}

// installPkgs 经 uv pip 安装依赖（wheels-only 禁 sdist，杜绝安装期任意代码执行）
func (m *PyEnvManager) installPkgs(ctx context.Context, venvDir string, pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := []string{"pip", "install", "--python", venvDir}
	if m.cfg.WheelsOnly {
		args = append(args, "--only-binary", ":all:")
	}
	args = append(args, pkgs...)
	out, err := m.runUV(ctx, args...)
	if err != nil {
		return fmt.Errorf("install python packages %v: %w: %s", pkgs, err, tailOf(string(out), 400))
	}
	return nil
}

// ensureUV 引导 uv 单文件静态二进制到 envRoot/_uv/uv（sha256 校验；mutex + 成功缓存，
// 失败不缓存以便网络恢复后重试）
func (m *PyEnvManager) ensureUV(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.uvPath != "" {
		if _, err := os.Stat(m.uvPath); err == nil {
			return m.uvPath, nil
		}
		m.uvPath = "" // 被外部清理：重新引导
	}
	target := filepath.Join(m.envRoot, uvToolsDir, "uv")
	if st, err := os.Stat(target); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
		m.uvPath = target
		return target, nil
	}
	url := m.cfg.UVDownloadURL
	if url == "" {
		var err error
		if url, err = defaultUVDownloadURL(m.cfg.UVVersion); err != nil {
			return "", err
		}
	}
	archive, err := httpDownload(ctx, url, maxUVArchiveBytes)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	want := m.cfg.UVSHA256
	if want == "" {
		// 官方旁挂 .sha256（内容形如 "<hex>  uv-xxx.tar.gz"），取首字段
		sumLine, err := httpGetString(ctx, url+".sha256")
		if err != nil {
			return "", fmt.Errorf("fetch %s.sha256: %w", url, err)
		}
		want = strings.TrimSpace(strings.SplitN(strings.TrimSpace(sumLine), " ", 2)[0])
	}
	got := sha256.Sum256(archive)
	if !strings.EqualFold(hex.EncodeToString(got[:]), want) {
		return "", fmt.Errorf("uv archive sha256 mismatch: refusing to install (got %s want %s)", hex.EncodeToString(got[:]), want)
	}
	tmpDir := filepath.Join(m.envRoot, uvToolsDir, "tmp-"+hex.EncodeToString(got[:])[:12])
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)
	tarPath := filepath.Join(tmpDir, "uv.tar.gz")
	if err := os.WriteFile(tarPath, archive, 0o644); err != nil {
		return "", err
	}
	if err := extractUVBinary(tarPath, target); err != nil {
		return "", err
	}
	m.uvPath = target
	logger.Info("[PyEnv] uv %s bootstrapped at %s", m.cfg.UVVersion, target)
	return target, nil
}

// ensureManagedPython 安装 uv 托管 CPython（python-build-standalone 独立构建，
// 不触碰系统 Python；mutex + 成功缓存）
func (m *PyEnvManager) ensureManagedPython(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pyOK {
		return nil
	}
	out, err := m.runUV(ctx, "python", "install", m.cfg.PythonVersion)
	if err != nil {
		return fmt.Errorf("%w: %s", err, tailOf(string(out), 400))
	}
	m.pyOK = true
	logger.Info("[PyEnv] managed CPython %s ready", m.cfg.PythonVersion)
	return nil
}

// runUV 执行 uv 子命令（uv_cache/python 安装目录与镜像经环境变量注入，均收敛在 envRoot 下）
func (m *PyEnvManager) runUV(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, m.uvPath, args...)
	cmd.Env = m.uvEnv()
	return cmd.CombinedOutput()
}

func (m *PyEnvManager) uvEnv() []string {
	env := os.Environ()
	env = setEnvVar(env, "UV_CACHE_DIR", filepath.Join(m.envRoot, uvToolsDir, "cache"))
	env = setEnvVar(env, "UV_PYTHON_INSTALL_DIR", filepath.Join(m.envRoot, uvToolsDir, "python"))
	env = setEnvVar(env, "UV_NO_PROGRESS", "1")
	if m.cfg.IndexURL != "" {
		env = setEnvVar(env, "UV_DEFAULT_INDEX", m.cfg.IndexURL)
	}
	if m.cfg.PythonInstallMirror != "" {
		env = setEnvVar(env, "UV_PYTHON_INSTALL_MIRROR", m.cfg.PythonInstallMirror)
	}
	return env
}

// defaultUVDownloadURL uv 官方 release 压缩包地址（单文件静态二进制 + 旁挂 .sha256）
func defaultUVDownloadURL(ver string) (string, error) {
	triple, err := uvTargetTriple()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://releases.astral.sh/github/uv/releases/download/%s/uv-%s.tar.gz", ver, triple), nil
}

// uvTargetTriple 当前平台对应的 uv release 三元组
func uvTargetTriple() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return "aarch64-apple-darwin", nil
		case "amd64":
			return "x86_64-apple-darwin", nil
		}
	case "linux":
		// musl 静态构建：对 glibc 无要求，发行版兼容面最大
		switch runtime.GOARCH {
		case "amd64":
			return "x86_64-unknown-linux-musl", nil
		case "arm64":
			return "aarch64-unknown-linux-musl", nil
		}
	}
	return "", fmt.Errorf("unsupported platform %s/%s for uv bootstrap (set AGENT_UV_DOWNLOAD_URL)", runtime.GOOS, runtime.GOARCH)
}

// extractUVBinary 从 uv tar.gz 中提取 uv 二进制（Typeflag==TypeReg 且 base 名为 "uv"；
// 目标路径固定，无 tar 路径穿越风险）
func extractUVBinary(tarPath, target string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("uv binary not found in archive")
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == "uv" {
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
			return nil
		}
	}
}

// ———— Node.js 运行时自动引导（对齐 uv 引导模式：宿主机零 Node 依赖）————

// ensureNode 确保 Node.js 运行时可用，返回 node/npm 所在 bin 目录（"" = 沿用宿主 PATH）。
// 三级探测：内存缓存 → envRoot/_node 自托管缓存 → 宿主 PATH（node+npm 均在才采用）→
// 官方发行包下载引导（sha256 校验）。成功缓存，失败不缓存以便网络恢复后重试。
func (m *PyEnvManager) ensureNode(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.nodeBinDir != "" {
		if _, err := os.Stat(filepath.Join(m.nodeBinDir, "node")); err == nil {
			return m.nodeBinDir, nil
		}
		m.nodeBinDir = "" // 被外部清理：重新引导
	}
	if bin := managedNodeBin(m.envRoot); bin != "" {
		m.nodeBinDir = bin
		return bin, nil
	}
	if _, err := exec.LookPath("node"); err == nil {
		if _, err := exec.LookPath("npm"); err == nil {
			return "", nil // 宿主 PATH 就绪：沿用宿主 Node，不前置任何路径
		}
	}
	bin, err := m.downloadNode(ctx)
	if err != nil {
		return "", err
	}
	m.nodeBinDir = bin
	return bin, nil
}

// managedNodeBin 自托管 Node 运行时缓存探测：<envRoot>/_node/bin/node 存在且可执行时返回其 bin 目录
func managedNodeBin(envRoot string) string {
	bin := filepath.Join(envRoot, nodeToolsDir, "bin", "node")
	if st, err := os.Stat(bin); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
		return filepath.Dir(bin)
	}
	return ""
}

// NodeRuntimeBinDir 自托管 Node bin 目录（nil 安全；未引导或残缺时返回 ""）。
// 环境画像读取用：只探测不引导（引导发生在 npm 依赖预检/执行期）
func (m *PyEnvManager) NodeRuntimeBinDir() string {
	if m == nil {
		return ""
	}
	return managedNodeBin(m.envRoot)
}

// EnsureNodeRuntime 确保 node/npm 运行时可用并返回其 bin 目录（"" = 沿用宿主 PATH）。
// 依赖预检用：三级引导（自托管缓存 → 宿主 PATH → 官方发行包下载），成功即缓存幂等；
// 使裸 node/npm 依赖声明在宿主机未装 Node 时也能由自托管运行时满足
func (m *PyEnvManager) EnsureNodeRuntime(ctx context.Context) (string, error) {
	return m.ensureNode(ctx)
}

// downloadNode 下载 Node.js 官方发行包到 envRoot/_node（sha256 校验 + 整树安全解压，
// <envRoot>/_node-tmp-<hash12> 临时目录解压后原子 rename 落盘；"_" 前缀豁免 TTL 清理）
func (m *PyEnvManager) downloadNode(ctx context.Context) (string, error) {
	ver := m.cfg.NodeVersion
	if ver == "" {
		ver = defaultNodeVersion
	}
	url := m.cfg.NodeDownloadURL
	if url == "" {
		var err error
		if url, err = defaultNodeDownloadURL(ver); err != nil {
			return "", err
		}
	}
	archive, err := httpDownload(ctx, url, maxNodeArchiveBytes)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	want := m.cfg.NodeSHA256
	if want == "" {
		// Node 发行包无 .sha256 旁挂，校验源为同目录 SHASUMS256.txt（行格式 "<hex>  <filename>"），
		// 自建镜像无该文件时必须显式配置 AGENT_NODE_SHA256（fail-closed）
		base := path.Base(url)
		sums, err := httpGetString(ctx, strings.TrimSuffix(url, base)+"SHASUMS256.txt")
		if err != nil {
			return "", fmt.Errorf("fetch SHASUMS256.txt for %s: %w", url, err)
		}
		if want, err = shasumsEntry(sums, base); err != nil {
			return "", err
		}
	}
	got := sha256.Sum256(archive)
	if !strings.EqualFold(hex.EncodeToString(got[:]), want) {
		return "", fmt.Errorf("node dist archive sha256 mismatch: refusing to install (got %s want %s)", hex.EncodeToString(got[:]), want)
	}
	target := filepath.Join(m.envRoot, nodeToolsDir)
	tmpDir := filepath.Join(m.envRoot, nodeToolsDir+"-tmp-"+hex.EncodeToString(got[:])[:12])
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)
	tarPath := filepath.Join(tmpDir, "node.tar.gz")
	if err := os.WriteFile(tarPath, archive, 0o644); err != nil {
		return "", err
	}
	// 包内是 node-vX.Y.Z-<plat>/ 完整目录树（bin/node + bin/npm 相对符号链接 + lib/node_modules/npm 整树），
	// 整树解压后整体挪入 _node；symlink 必须保留否则 npm 不可用
	if err := extractNodeDist(tarPath, tmpDir); err != nil {
		return "", err
	}
	distDir, err := nodeDistRoot(tmpDir)
	if err != nil {
		return "", err
	}
	if err := os.Rename(distDir, target); err != nil {
		return "", fmt.Errorf("promote %s to %s: %w", distDir, target, err)
	}
	logger.Info("[PyEnv] Node.js %s bootstrapped at %s", ver, filepath.Join(target, "bin"))
	return filepath.Join(target, "bin"), nil
}

// nodeDistRoot 定位解压后的发行包根目录（tar 包内为单一顶层目录 node-vX.Y.Z-<plat>/）
func nodeDistRoot(tmpDir string) (string, error) {
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(tmpDir, e.Name()))
		}
	}
	if len(dirs) == 1 {
		return dirs[0], nil
	}
	return "", fmt.Errorf("node dist archive: expected exactly one top-level directory, got %d", len(dirs))
}

// defaultNodeDownloadURL Node.js 官方发行包地址（nodejs.org/dist，SHASUMS256.txt 同目录旁挂）
func defaultNodeDownloadURL(ver string) (string, error) {
	triple, err := nodeTargetTriple()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://nodejs.org/dist/v%s/node-v%s-%s.tar.gz", ver, ver, triple), nil
}

// nodeTargetTriple 当前平台对应的 Node.js 发行包平台后缀
func nodeTargetTriple() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return "darwin-arm64", nil
		case "amd64":
			return "darwin-x64", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "linux-x64", nil
		case "arm64":
			return "linux-arm64", nil
		}
	}
	return "", fmt.Errorf("unsupported platform %s/%s for node bootstrap (set AGENT_NODE_DOWNLOAD_URL)", runtime.GOOS, runtime.GOARCH)
}

// shasumsEntry 从 SHASUMS256.txt 内容取指定文件名的 sha256
func shasumsEntry(content, filename string) (string, error) {
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 2 && fields[1] == filename {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("SHASUMS256.txt has no entry for %s: refusing to install (set AGENT_NODE_SHA256 for mirror deployments)", filename)
}

// extractNodeDist 发行包 tar.gz 整树安全解压到 targetDir（保留包内顶层目录，由 nodeDistRoot 定位）：
// 拒绝路径穿越/绝对路径/未知条目类型；链接目标解析后不得逃逸解压根
// （symlink 相对所在目录解析，hardlink 相对解压根解析）
func extractNodeDist(tarPath, targetDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("node dist archive: unsafe entry %q", hdr.Name)
		}
		dest := filepath.Join(targetDir, name)
		if dest == targetDir { // 顶层 "." 条目：仅目录合法
			if hdr.Typeflag != tar.TypeDir {
				return fmt.Errorf("node dist archive: unexpected top-level entry %q", hdr.Name)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			base := filepath.Dir(dest)
			if hdr.Typeflag == tar.TypeLink {
				base = targetDir // hardlink 目标相对解压根
			}
			if err := checkNodeLinkTarget(targetDir, base, dest, filepath.Clean(hdr.Linkname)); err != nil {
				return err
			}
			_ = os.Remove(dest)
			if hdr.Typeflag == tar.TypeSymlink {
				err = os.Symlink(hdr.Linkname, dest)
			} else {
				err = os.Link(filepath.Join(targetDir, filepath.Clean(hdr.Linkname)), dest)
			}
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("node dist archive: unsupported entry type %q (%s)", hdr.Typeflag, hdr.Name)
		}
	}
}

// checkNodeLinkTarget 链接目标安全校验：非绝对路径，且按其解析基准（base）Clean 后仍位于解压根内
func checkNodeLinkTarget(root, base, link, target string) error {
	if filepath.IsAbs(target) {
		return fmt.Errorf("node dist archive: absolute link target %q in %q", target, link)
	}
	resolved := filepath.Clean(filepath.Join(base, target))
	if resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator)) {
		return fmt.Errorf("node dist archive: link %q target %q escapes extraction root", link, target)
	}
	return nil
}

// httpDownload 下载到内存（LimitReader 设上限防滥用）
func httpDownload(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return data, nil
}

// httpGetString 小文本下载（sha256 旁挂文件等）
func httpGetString(ctx context.Context, url string) (string, error) {
	b, err := httpDownload(ctx, url, 4096)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// StartCleanupLoop 启动过期任务环境清理（每小时扫描；nil 安全；TTL<=0 不启动）
func (m *PyEnvManager) StartCleanupLoop() {
	if m == nil || m.cfg.CleanupTTL <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			m.cleanupExpired()
		}
	}()
}

// cleanupExpired 删除 mtime 早于 TTL 的任务环境目录（venv 与 <taskID>_node；
// "_" 前缀工具链目录与非目录条目跳过）
func (m *PyEnvManager) cleanupExpired() {
	entries, err := os.ReadDir(m.envRoot)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-m.cfg.CleanupTTL)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		dir := filepath.Join(m.envRoot, e.Name())
		if err := os.RemoveAll(dir); err == nil {
			logger.Info("[PyEnv] cleaned expired task env %s", dir)
		}
	}
}

// RemoveTaskEnv 删除任务专属执行环境（venv 与 node_modules，任务结束即调：
// 环境只为本次执行服务，终态后保留只会堆积同构目录）。_ 前缀与路径可疑字符拒绝
// （防误删工具链目录/穿越）；目录不存在视为已清理（幂等）；删除失败仅告警，由 TTL 清理循环兜底
func (m *PyEnvManager) RemoveTaskEnv(taskID string) {
	if m == nil {
		return
	}
	name := filepath.Base(taskID)
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, "_") || strings.ContainsAny(name, "/\\") {
		return
	}
	for _, dir := range []string{
		filepath.Join(m.envRoot, name),
		filepath.Join(m.envRoot, name+nodeEnvSuffix),
	} {
		if err := os.RemoveAll(dir); err != nil {
			logger.Warn("[PyEnv] failed to remove task env %s: %v", dir, err)
		} else {
			logger.Info("[PyEnv] removed task env %s", dir)
		}
	}
}

// VenvInfo 读取 venv 的 CPython 版本与预装包清单（环境画像展示用；venv 缺失时返回空值）
func (m *PyEnvManager) VenvInfo(venvDir string) (version string, pkgs []string) {
	data, err := os.ReadFile(filepath.Join(venvDir, "pyvenv.cfg"))
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if v, ok := strings.CutPrefix(line, "version"); ok {
				v = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v), "="))
				if v != "" {
					version = v
				}
			}
		}
	}
	matches, _ := filepath.Glob(filepath.Join(venvDir, "lib", "python*", "site-packages", "*.dist-info"))
	for _, p := range matches {
		base := filepath.Base(p)
		if i := strings.LastIndex(base, "-"); i > 0 {
			base = base[:i]
		}
		pkgs = append(pkgs, base)
	}
	sort.Strings(pkgs)
	return version, pkgs
}

// NodeEnvInfo 读取任务 Node 环境的预装包清单（环境画像展示用；node_modules 缺失时返回空）。
// 支持 @scope/name 形式的嵌套目录展开
func (m *PyEnvManager) NodeEnvInfo(nodeEnvDir string) (pkgs []string) {
	nmDir := filepath.Join(nodeEnvDir, "node_modules")
	entries, err := os.ReadDir(nmDir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if strings.HasPrefix(e.Name(), "@") {
			// @scope 目录：展开第二层为 @scope/name
			scoped, err := os.ReadDir(filepath.Join(nmDir, e.Name()))
			if err != nil {
				continue
			}
			for _, se := range scoped {
				if se.IsDir() && !strings.HasPrefix(se.Name(), ".") {
					pkgs = append(pkgs, e.Name()+"/"+se.Name())
				}
			}
			continue
		}
		pkgs = append(pkgs, e.Name())
	}
	sort.Strings(pkgs)
	return pkgs
}

// validTaskEnvID 任务 ID 作为 envRoot 子目录名的安全校验（防路径穿越/隐藏目录）
func validTaskEnvID(id string) error {
	if id == "" || len(id) > 128 || strings.ContainsAny(id, "/\\") || strings.HasPrefix(id, ".") {
		return fmt.Errorf("invalid task id for task environment: %q", id)
	}
	return nil
}

// tailOf 取字符串尾部（错误输出截断展示用）
func tailOf(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}
