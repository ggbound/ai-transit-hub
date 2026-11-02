package service

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveInterpreterNodeMissing 宿主无 node 且无自托管缓存：TOOL ERROR 须附模型可执行的恢复路径
func TestResolveInterpreterNodeMissing(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t)) // 空目录：node 不可寻址
	_, got := resolveInterpreter("node", t.TempDir())
	assertContains(t, got, `interpreter "node" not available on host`)
	assertContains(t, got, "Node.js is not installed")
	assertContains(t, got, "rewrite the logic as a Python script")
	assertContains(t, got, "install Node.js")
}

// TestResolveInterpreterNodeJSAlias nodejs 别名同享 Node 专属提示
func TestResolveInterpreterNodeJSAlias(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t))
	_, got := resolveInterpreter("nodejs", t.TempDir())
	assertContains(t, got, "Node.js is not installed")
}

// TestResolveInterpreterNonNodeMissing 非 node 解释器缺失：维持通用报错，不掺入 Node 提示
func TestResolveInterpreterNonNodeMissing(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t))
	_, got := resolveInterpreter("python3", t.TempDir())
	assertContains(t, got, `interpreter "python3" not available on host`)
	assertNotContains(t, got, "Node.js is not installed")
}

// TestResolveInterpreterFound fake node 在 PATH：校验通过，原样返回（宿主优先语义）
func TestResolveInterpreterFound(t *testing.T) {
	bin := fakeBinDir(t)
	fakeBin(t, bin, "node", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)
	got, msg := resolveInterpreter("node", t.TempDir())
	if msg != "" {
		t.Errorf("resolveInterpreter(fake node) msg = %q, want empty", msg)
	}
	if got != "node" {
		t.Errorf("resolveInterpreter(fake node) = %q, want %q", got, "node")
	}
}

// TestResolveInterpreterAbsoluteVenvPath venv 模式传入解释器绝对路径：不受 PATH 清空影响
func TestResolveInterpreterAbsoluteVenvPath(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t))
	bin := fakeBinDir(t)
	py := filepath.Join(bin, "python3")
	fakeBin(t, bin, "python3", "#!/bin/sh\nexit 0\n")
	if got, msg := resolveInterpreter(py, t.TempDir()); msg != "" || got != py {
		t.Errorf("resolveInterpreter(abs python3) = (%q, %q), want (%q, \"\")", got, msg, py)
	}
}

// TestResolveInterpreterSelfHostedFallback 宿主无 node 但自托管运行时已引导：
// 回退 <envRoot>/_node/bin/node 绝对路径，不报错
func TestResolveInterpreterSelfHostedFallback(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t)) // 宿主无 node
	m := newTestPyEnvManager(t, "")
	nodeBin := installFakeNode(t, m)
	got, msg := resolveInterpreter("node", m.envRoot)
	if msg != "" {
		t.Fatalf("resolveInterpreter(self-hosted) msg = %q, want empty", msg)
	}
	if got != filepath.Join(nodeBin, "node") {
		t.Errorf("resolveInterpreter(self-hosted) = %q, want %q", got, filepath.Join(nodeBin, "node"))
	}
}

// TestResolveInterpreterSelfHostedBroken 自托管缓存残缺（node 不可执行）：
// 不采用，回落到不可用报错
func TestResolveInterpreterSelfHostedBroken(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t))
	m := newTestPyEnvManager(t, "")
	nodeBin := installFakeNode(t, m)
	// 置为不可执行（0o644）：managedNodeBin 要求可执行
	// 注意 os.WriteFile 对已存在文件不改权限，须显式 Chmod
	nodePath := filepath.Join(nodeBin, "node")
	if err := os.WriteFile(nodePath, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatalf("break node binary: %v", err)
	}
	if err := os.Chmod(nodePath, 0o644); err != nil {
		t.Fatalf("break node binary chmod: %v", err)
	}
	_, msg := resolveInterpreter("node", m.envRoot)
	if msg == "" {
		t.Fatal("expected error when self-hosted node is not executable")
	}
	assertContains(t, msg, `interpreter "node" not available on host`)
}

// TestParseSystemProxyLoopbackPorts 解析 scutil --proxy 输出：仅收集 Enable=1
// 且代理 host 为回环地址的端口，同端口去重；非回环 host、未启用、畸形值均跳过
func TestParseSystemProxyLoopbackPorts(t *testing.T) {
	// hostOut：本机（127.0.0.1:7890，HTTP/HTTPS/SOCKS 启用，含异常列表与 PAC 关闭）
	hostOut := `<dictionary> {
  HTTPEnable : 1
  HTTPPort : 7890
  HTTPProxy : 127.0.0.1
  HTTPSEnable : 1
  HTTPSPort : 7890
  HTTPSProxy : 127.0.0.1
  SOCKSEnable : 1
  SOCKSPort : 7890
  SOCKSProxy : 127.0.0.1
  ProxyAutoConfigEnable : 0
  ExceptionsList : <array> {
    0 : localhost
    1 : 127.0.0.1
  }
}
`
	cases := []struct {
		name string
		in   string
		want []int
	}{
		{name: "本机 scutil 实际输出", in: hostOut, want: []int{7890}},
		{name: "各组不同端口按协议序去重前收集", in: `<dictionary> {
  HTTPEnable : 1
  HTTPPort : 8080
  HTTPProxy : localhost
  HTTPSEnable : 1
  HTTPSPort : 8443
  HTTPSProxy : ::1
  SOCKSEnable : 1
  SOCKSPort : 1080
  SOCKSProxy : 0.0.0.0
}
`, want: []int{8080, 8443, 1080}},
		{name: "非回环域名 host 不收集", in: `<dictionary> {
  HTTPEnable : 1
  HTTPPort : 3128
  HTTPProxy : "proxy.corp.example.com"
}
`, want: nil},
		{name: "Enable=0 不收集", in: `<dictionary> {
  HTTPEnable : 0
  HTTPPort : 7890
  HTTPProxy : 127.0.0.1
}
`, want: nil},
		{name: "FTP 组同理收集", in: `<dictionary> {
  FTPEnable : 1
  FTPPort : 2121
  FTPProxy : 127.0.0.1
}
`, want: []int{2121}},
		{name: "畸形端口/缺键跳过", in: `<dictionary> {
  HTTPEnable : 1
  HTTPPort : not-a-port
  HTTPProxy : 127.0.0.1
  SOCKSEnable : 1
  SOCKSPort : 0
  SOCKSProxy : 127.0.0.1
}
`, want: nil},
		{name: "空输出", in: "", want: nil},
		{name: "非键值行忽略", in: "random text without kv\n: leading colon only\n", want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseSystemProxyLoopbackPorts(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}
