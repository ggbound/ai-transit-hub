package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/your-org/ai-transit-hub/internal/config"
)

// fakeBin 在 dir 下写入可执行 shell 脚本（LookPath 只要求存在且可执行，不要求真实二进制）
func fakeBin(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
}

// fakeBinDir 创建空的 PATH 目录（目录内无任何可执行文件 → LookPath 全部失败）
func fakeBinDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// fakeNpmInstall 仿真 npm install：解析 --prefix，创建 node_modules 并把环境变量落盘供断言。
// PATH 注入断言依赖 .path-dump（重置 PATH 之前用 shell 内建 echo 落盘，保留 npm 实际收到的 PATH）；
// 之后恢复基础 PATH：测试已把 PATH 换成仅含 fake 二进制的目录，mkdir/env 等标准工具需显式寻址。
const fakeNpmInstall = `#!/bin/sh
prefix=""
while [ $# -gt 0 ]; do
  if [ "$1" = "--prefix" ]; then
    prefix="$2"
    shift
  fi
  shift
done
if [ -z "$prefix" ]; then
  echo "npm error: missing --prefix" >&2
  exit 1
fi
echo "PATH=$PATH" > "$prefix/.path-dump"
PATH=/usr/bin:/bin
export PATH
mkdir -p "$prefix/node_modules"
env > "$prefix/.env-dump"
`

// installFakeNode 在 envRoot/_node 铺自托管 Node 仿制运行时（ensureNode 磁盘缓存路径的
// 离线替身），返回 bin 目录。node 回显注入后的 PATH 供注入断言；npm 用 fakeNpmInstall。
func installFakeNode(t *testing.T, m *PyEnvManager) string {
	t.Helper()
	binDir := filepath.Join(m.envRoot, nodeToolsDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("prepare _node/bin: %v", err)
	}
	fakeBin(t, binDir, "node", "#!/bin/sh\necho \"PATH=$PATH\"\nexit 0\n")
	fakeBin(t, binDir, "npm", fakeNpmInstall)
	return binDir
}

// tarEntry 测试用 tar 条目（body 仅 TypeReg 有效）
type tarEntry struct {
	hdr  tar.Header
	body string
}

// buildTarGz 把条目打包成 tar.gz 字节流（仿官方发行包下载体）
func buildTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		if e.hdr.Typeflag == tar.TypeReg {
			e.hdr.Size = int64(len(e.body)) // tar.Writer 按 hdr.Size 校验写入长度
		}
		if err := tw.WriteHeader(&e.hdr); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if e.body != "" {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatalf("write tar body: %v", err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

// fakeNodeDist 仿官方发行包：顶层 distName/（bin/node 可执行文件、bin/npm →
// ../lib/node_modules/npm/cli.sh 相对符号链接、cli.sh 为 fakeNpmInstall）。
// symlink 保留是 npm 可用的前提，也是解压安全实现要重点守护的部分
func fakeNodeDist(t *testing.T, distName string) []byte {
	t.Helper()
	return buildTarGz(t, []tarEntry{
		{hdr: tar.Header{Name: distName + "/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{hdr: tar.Header{Name: distName + "/bin/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{hdr: tar.Header{Name: distName + "/lib/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{hdr: tar.Header{Name: distName + "/lib/node_modules/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{hdr: tar.Header{Name: distName + "/lib/node_modules/npm/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{hdr: tar.Header{Name: distName + "/bin/node", Typeflag: tar.TypeReg, Mode: 0o755}, body: "#!/bin/sh\nexit 0\n"},
		{hdr: tar.Header{Name: distName + "/bin/npm", Typeflag: tar.TypeSymlink, Linkname: "../lib/node_modules/npm/cli.sh", Mode: 0o777}},
		{hdr: tar.Header{Name: distName + "/lib/node_modules/npm/cli.sh", Typeflag: tar.TypeReg, Mode: 0o755}, body: fakeNpmInstall},
	})
}

// newTestPyEnvManager 构造指向临时 envRoot 的管理器（绕过 NewPyEnvManager 的 Mode 门控）
func newTestPyEnvManager(t *testing.T, registry string) *PyEnvManager {
	t.Helper()
	return &PyEnvManager{
		cfg: config.PyEnvConfig{
			InstallTimeout: 30 * time.Second,
			NpmRegistry:    registry,
		},
		envRoot: t.TempDir(),
	}
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("output does not contain %q:\ngot: %s", want, got)
	}
}

func assertNotContains(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Errorf("output unexpectedly contains %q:\ngot: %s", want, got)
	}
}

// TestEnsureTaskNodeEnvBootstrapFailure 宿主无 node 且下载不可达（127.0.0.1:0 立即
// connection refused，避免真实网络访问）：报错经 node runtime bootstrap failed 包装
// 并携带 download 上下文；失败不落盘（不缓存，网络恢复后可重试）
func TestEnsureTaskNodeEnvBootstrapFailure(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t)) // 空目录：node/npm 均不可寻址
	m := newTestPyEnvManager(t, "")
	m.cfg.NodeDownloadURL = "http://127.0.0.1:0/node-v22.20.0-test.tar.gz"
	_, err := m.EnsureTaskNodeEnv(context.Background(), "task-1", []string{"typescript"})
	if err == nil {
		t.Fatal("expected error when node download fails")
	}
	assertContains(t, err.Error(), "node runtime bootstrap failed")
	assertContains(t, err.Error(), "download http://127.0.0.1:0/node-v22.20.0-test.tar.gz")
	if _, serr := os.Stat(filepath.Join(m.envRoot, nodeToolsDir)); !os.IsNotExist(serr) {
		t.Errorf("_node must not exist after failed bootstrap, stat err = %v", serr)
	}
}

// TestEnsureTaskNodeEnvBootstrapSuccess httptest 仿真官方发行包下载 + SHASUMS256.txt 校验：
// 自托管运行时落盘 envRoot/_node（npm 相对符号链接保留），fake npm 经 cli.sh 装好
// node_modules，npmEnv 把自托管 bin 前置 PATH，临时解压目录无残留
func TestEnsureTaskNodeEnvBootstrapSuccess(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t)) // 宿主无 node/npm：强制走下载引导
	const distName = "node-v22.20.0-test-darwin-arm64"
	dist := fakeNodeDist(t, distName)
	sum := sha256.Sum256(dist)
	mux := http.NewServeMux()
	mux.HandleFunc("/"+distName+".tar.gz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(dist)
	})
	mux.HandleFunc("/SHASUMS256.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s  %s.tar.gz\n", hex.EncodeToString(sum[:]), distName)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	m := newTestPyEnvManager(t, "")
	m.cfg.NodeDownloadURL = srv.URL + "/" + distName + ".tar.gz" // SHA256 缺省 → 同目录 SHASUMS256.txt
	envDir, err := m.EnsureTaskNodeEnv(context.Background(), "task-1", []string{"typescript"})
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	nodeBin := filepath.Join(m.envRoot, nodeToolsDir, "bin")
	if st, serr := os.Stat(filepath.Join(nodeBin, "node")); serr != nil || st.IsDir() {
		t.Errorf("self-hosted node missing or dir: %v", serr)
	}
	if st, serr := os.Lstat(filepath.Join(nodeBin, "npm")); serr != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Errorf("bin/npm must be a preserved symlink: %v", serr)
	}
	if _, serr := os.Stat(filepath.Join(envDir, "node_modules")); serr != nil {
		t.Errorf("node_modules not created by self-hosted npm: %v", serr)
	}
	pathDump, serr := os.ReadFile(filepath.Join(envDir, ".path-dump"))
	if serr != nil {
		t.Fatalf("fake npm did not dump PATH: %v", serr)
	}
	assertContains(t, string(pathDump), "PATH="+nodeBin+":")
	entries, rerr := os.ReadDir(m.envRoot)
	if rerr != nil {
		t.Fatalf("read envRoot: %v", rerr)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), nodeToolsDir+"-tmp-") {
			t.Errorf("tmp extraction dir %s not cleaned up", e.Name())
		}
	}
}

// TestEnsureTaskNodeEnvSHAMismatch 校验和失配 fail-closed：拒绝安装且 _node 不落盘
func TestEnsureTaskNodeEnvSHAMismatch(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t))
	const distName = "node-v22.20.0-test-darwin-arm64"
	dist := fakeNodeDist(t, distName)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(dist) // SHA256 已显式配置：不请求 SHASUMS256.txt
	}))
	defer srv.Close()

	m := newTestPyEnvManager(t, "")
	m.cfg.NodeDownloadURL = srv.URL + "/" + distName + ".tar.gz"
	m.cfg.NodeSHA256 = strings.Repeat("0", 64)
	_, err := m.EnsureTaskNodeEnv(context.Background(), "task-1", []string{"typescript"})
	if err == nil {
		t.Fatal("expected sha256 mismatch error")
	}
	assertContains(t, err.Error(), "sha256 mismatch")
	if _, serr := os.Stat(filepath.Join(m.envRoot, nodeToolsDir)); !os.IsNotExist(serr) {
		t.Errorf("_node must not exist after sha mismatch, stat err = %v", serr)
	}
}

// TestEnsureNodeLevelOrder 三级探测顺序：自托管磁盘缓存 > 宿主 PATH（node+npm 双全才
// 采用）> 下载引导（单测经不可达 URL 触达，避免真实网络）
func TestEnsureNodeLevelOrder(t *testing.T) {
	ctx := context.Background()
	// 1. 宿主 node+npm 双全 → ("", nil) 沿用宿主，不触发下载
	hostBin := fakeBinDir(t)
	fakeBin(t, hostBin, "node", "#!/bin/sh\nexit 0\n")
	fakeBin(t, hostBin, "npm", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", hostBin)
	m := newTestPyEnvManager(t, "")
	m.cfg.NodeDownloadURL = "http://127.0.0.1:0/node.tar.gz" // 误走下载即失败，暴露顺序错误
	if bin, err := m.ensureNode(ctx); err != nil || bin != "" {
		t.Errorf("host PATH ready: ensureNode = (%q, %v), want (\"\", nil)", bin, err)
	}

	// 2. 宿主有 node 无 npm → 不采用（npm 不可用），落到下载引导
	onlyNode := fakeBinDir(t)
	fakeBin(t, onlyNode, "node", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", onlyNode)
	m2 := newTestPyEnvManager(t, "")
	m2.cfg.NodeDownloadURL = "http://127.0.0.1:0/node.tar.gz"
	_, err := m2.ensureNode(ctx)
	if err == nil {
		t.Error("host node without npm must fall through to download")
	} else {
		assertContains(t, err.Error(), "download")
	}

	// 3. 磁盘缓存优先于宿主 PATH：宿主双全仍用自托管
	t.Setenv("PATH", hostBin)
	m3 := newTestPyEnvManager(t, "")
	managed := installFakeNode(t, m3)
	if bin, err := m3.ensureNode(ctx); err != nil || bin != managed {
		t.Errorf("disk cache priority: ensureNode = (%q, %v), want (%q, nil)", bin, err, managed)
	}
}

// TestEnsureTaskNodeEnvNoPkgsSkipsHostCheck 无 npm 依赖时不触碰宿主 node/npm（含 PATH 为空场景）
func TestEnsureTaskNodeEnvNoPkgsSkipsHostCheck(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t)) // 宿主无 node/npm，但本用例不应探测它们
	m := newTestPyEnvManager(t, "")
	dir, err := m.EnsureTaskNodeEnv(context.Background(), "task-1", nil)
	if err != nil {
		t.Fatalf("unexpected error for empty deps: %v", err)
	}
	if dir != m.TaskNodeEnvDir("task-1") {
		t.Errorf("dir = %q, want task node env dir %q", dir, m.TaskNodeEnvDir("task-1"))
	}
}

// TestEnsureTaskNodeEnvInstallFailure npm install 失败：报错须含包名与 npm 输出尾部（便于定位网络/registry 问题）
func TestEnsureTaskNodeEnvInstallFailure(t *testing.T) {
	bin := fakeBinDir(t)
	fakeBin(t, bin, "node", "#!/bin/sh\nexit 0\n")
	fakeBin(t, bin, "npm", `#!/bin/sh
echo "npm error code E404" >&2
echo "npm error 404 Not Found - GET https://registry.npmjs.org/no-such-pkg" >&2
exit 1
`)
	t.Setenv("PATH", bin)
	m := newTestPyEnvManager(t, "")
	_, err := m.EnsureTaskNodeEnv(context.Background(), "task-1", []string{"no-such-pkg"})
	if err == nil {
		t.Fatal("expected error when npm install fails")
	}
	assertContains(t, err.Error(), "install npm packages [no-such-pkg]")
	assertContains(t, err.Error(), "E404")
}

// TestEnsureTaskNodeEnvSuccess fake npm 装包成功：目录/预置 package.json/npmEnv（缓存收敛与 registry 覆盖）逐项断言
func TestEnsureTaskNodeEnvSuccess(t *testing.T) {
	bin := fakeBinDir(t)
	fakeBin(t, bin, "node", "#!/bin/sh\nexit 0\n")
	fakeBin(t, bin, "npm", fakeNpmInstall)
	t.Setenv("PATH", bin)

	const registry = "https://registry.example.com"
	m := newTestPyEnvManager(t, registry)
	envDir, err := m.EnsureTaskNodeEnv(context.Background(), "task-1", []string{"typescript", "left-pad"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if envDir != m.TaskNodeEnvDir("task-1") {
		t.Errorf("envDir = %q, want %q", envDir, m.TaskNodeEnvDir("task-1"))
	}
	if _, err := os.Stat(filepath.Join(envDir, "node_modules")); err != nil {
		t.Errorf("node_modules not created by npm install: %v", err)
	}
	// 预置最小 package.json
	pj, err := os.ReadFile(filepath.Join(envDir, "package.json"))
	if err != nil {
		t.Fatalf("package.json not prepared: %v", err)
	}
	if strings.TrimSpace(string(pj)) != "{}" {
		t.Errorf("package.json = %q, want {}", string(pj))
	}
	// npmEnv：缓存收敛到 envRoot/_npm-cache（"_" 前缀豁免 TTL）、registry 覆盖生效
	dump, err := os.ReadFile(filepath.Join(envDir, ".env-dump"))
	if err != nil {
		t.Fatalf("fake npm did not dump env: %v", err)
	}
	envDump := string(dump)
	assertContains(t, envDump, "npm_config_cache="+filepath.Join(m.envRoot, npmCacheDir))
	assertContains(t, envDump, "npm_config_registry="+registry)
	assertContains(t, envDump, "npm_config_update_notifier=false")
}

// TestTaskNodeEnvDirLayout 目录约定：<envRoot>/<taskID>_node，node_modules 为其子目录
func TestTaskNodeEnvDirLayout(t *testing.T) {
	m := newTestPyEnvManager(t, "")
	root := m.envRoot
	if got, want := m.TaskNodeEnvDir("task-1"), filepath.Join(root, "task-1_node"); got != want {
		t.Errorf("TaskNodeEnvDir = %q, want %q", got, want)
	}
	if got, want := m.TaskNodeModulesDir("task-1"), filepath.Join(root, "task-1_node", "node_modules"); got != want {
		t.Errorf("TaskNodeModulesDir = %q, want %q", got, want)
	}
}

// TestShasumsEntry SHASUMS256.txt 解析：取目标文件行（两字段、容忍多余空白），缺失即报错
func TestShasumsEntry(t *testing.T) {
	content := "abcd  other.tar.gz\n1234  node-v22.20.0-test.tar.gz\n  ef56   extra.txt\n"
	got, err := shasumsEntry(content, "node-v22.20.0-test.tar.gz")
	if err != nil || got != "1234" {
		t.Errorf("shasumsEntry = (%q, %v), want (\"1234\", nil)", got, err)
	}
	if _, err := shasumsEntry(content, "no-such.tar.gz"); err == nil {
		t.Error("expected error for missing entry")
	} else {
		assertContains(t, err.Error(), "has no entry for no-such.tar.gz")
	}
}

// TestExtractNodeDistSecurity 发行包解压安全矩阵：路径穿越/绝对路径/链接逃逸/未知条目
// 类型全部拒绝；正常包（含相对 symlink）完整解出且链接保留
func TestExtractNodeDistSecurity(t *testing.T) {
	cases := []struct {
		name    string
		dist    []byte
		wantErr string
	}{
		{"traversal", buildTarGz(t, []tarEntry{
			{hdr: tar.Header{Name: "../evil.txt", Typeflag: tar.TypeReg, Mode: 0o644}, body: "x"},
		}), "unsafe entry"},
		{"absolute path", buildTarGz(t, []tarEntry{
			{hdr: tar.Header{Name: "/etc/evil", Typeflag: tar.TypeReg, Mode: 0o644}, body: "x"},
		}), "unsafe entry"},
		{"symlink escape", buildTarGz(t, []tarEntry{
			{hdr: tar.Header{Name: "dist/bin/", Typeflag: tar.TypeDir, Mode: 0o755}},
			{hdr: tar.Header{Name: "dist/bin/evil", Typeflag: tar.TypeSymlink, Linkname: "../../../evil", Mode: 0o777}},
		}), "escapes extraction root"},
		{"absolute link", buildTarGz(t, []tarEntry{
			{hdr: tar.Header{Name: "dist/bin/", Typeflag: tar.TypeDir, Mode: 0o755}},
			{hdr: tar.Header{Name: "dist/bin/evil", Typeflag: tar.TypeSymlink, Linkname: "/etc/evil", Mode: 0o777}},
		}), "absolute link target"},
		{"hardlink escape", buildTarGz(t, []tarEntry{
			{hdr: tar.Header{Name: "dist/", Typeflag: tar.TypeDir, Mode: 0o755}},
			{hdr: tar.Header{Name: "dist/evil", Typeflag: tar.TypeLink, Linkname: "../../evil", Mode: 0o644}},
		}), "escapes extraction root"},
		{"unsupported type", buildTarGz(t, []tarEntry{
			{hdr: tar.Header{Name: "dist/char", Typeflag: tar.TypeChar, Mode: 0o644}},
		}), "unsupported entry type"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			archive := filepath.Join(tmp, "dist.tar.gz")
			if err := os.WriteFile(archive, tc.dist, 0o644); err != nil {
				t.Fatalf("write archive: %v", err)
			}
			err := extractNodeDist(archive, t.TempDir())
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			assertContains(t, err.Error(), tc.wantErr)
		})
	}
	// happy path：bin/node 可执行落盘，bin/npm 相对符号链接保留
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "dist.tar.gz")
	if err := os.WriteFile(archive, fakeNodeDist(t, "node-vX-test"), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	dest := t.TempDir()
	if err := extractNodeDist(archive, dest); err != nil {
		t.Fatalf("extract happy path: %v", err)
	}
	if st, serr := os.Stat(filepath.Join(dest, "node-vX-test", "bin", "node")); serr != nil || st.IsDir() || st.Mode()&0o111 == 0 {
		t.Errorf("bin/node not extracted executable: %v", serr)
	}
	if st, serr := os.Lstat(filepath.Join(dest, "node-vX-test", "bin", "npm")); serr != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Errorf("bin/npm symlink not preserved: %v", serr)
	}
}

// TestManagedNodeBin 自托管缓存探测四态：缺席/目录/不可执行 → ""，可执行 → bin 目录
func TestManagedNodeBin(t *testing.T) {
	envRoot := t.TempDir()
	binDir := filepath.Join(envRoot, nodeToolsDir, "bin")
	binNode := filepath.Join(binDir, "node")
	if got := managedNodeBin(envRoot); got != "" {
		t.Errorf("absent: got %q, want empty", got)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.Mkdir(binNode, 0o755); err != nil {
		t.Fatalf("mkdir node: %v", err)
	}
	if got := managedNodeBin(envRoot); got != "" {
		t.Errorf("dir: got %q, want empty", got)
	}
	if err := os.Remove(binNode); err != nil {
		t.Fatalf("remove node dir: %v", err)
	}
	if err := os.WriteFile(binNode, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatalf("write non-exec node: %v", err)
	}
	if got := managedNodeBin(envRoot); got != "" {
		t.Errorf("non-executable: got %q, want empty", got)
	}
	if err := os.Chmod(binNode, 0o755); err != nil {
		t.Fatalf("chmod node: %v", err)
	}
	if got := managedNodeBin(envRoot); got != binDir {
		t.Errorf("executable: got %q, want %q", got, binDir)
	}
}
