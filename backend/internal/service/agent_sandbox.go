package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/your-org/ai-transit-hub/internal/config"
	"github.com/your-org/ai-transit-hub/internal/logger"
)

// Sandbox 脚本执行沙箱后端抽象：execute_script 的实际执行单位。
// 各后端自行完成"解释器校验 + 路径规整 + 隔离边界构造 + 进程捕获运行"，
// 运行时（AgentRuntime）只负责按自适应链选出首个可用后端并下发 ExecSpec。
// 兜底语义：auto 链末位默认收录 plain（无沙箱直跑，仅有超时与输出截断），
// 保证沙箱缺失环境任务不中断；AGENT_SANDBOX_ALLOW_PLAIN=false 或显式锁定
// 单一后端时 fail-closed（无可用后端即拒绝执行，不静默降级）
type Sandbox interface {
	// Name 后端标识（日志与宿主机环境画像展示用）
	Name() string
	// Available 当前宿主机/环境下该后端是否实际可用。
	// 逐次探测（而非构造时缓存），保证运行环境变化后行为一致；
	// 唯一例外是 docker 后端对 daemon/镜像的探测带短 TTL 缓存（探活含子进程启动，成本高）
	Available() bool
	// SupportsVenv 该后端能否承载"宿主侧构建的任务环境"（任务 venv 与任务级 node_modules，
	// 二者同为宿主 OS 构建产物，兼容性判定合并）。
	// 托管 CPython 与 node_modules 内的原生模块按宿主 OS 构建，跨 OS 的容器内无法运行，
	// 因此 docker 后端仅在 Linux 宿主上支持（macOS 容器内跑不了 darwin 二进制）
	SupportsVenv() bool
	// ProfileLine 注入 system prompt 的沙箱边界描述（宿主机环境画像用）
	ProfileLine() string
	// Exec 执行脚本并返回回传给模型的文本（exit code + stdout + stderr 或 TOOL ERROR）
	Exec(ctx context.Context, spec ExecSpec) string
}

// ExecSpec 一次脚本执行的参数。均为宿主机侧路径（与调用方视角一致），
// 容器类后端在 Exec 内部完成路径映射与环境组装
type ExecSpec struct {
	Interpreter string   // 解释器（venv 模式下为任务 venv 的 python3 绝对路径）
	Script      string   // 脚本路径（工作区内或技能包内）
	Args        []string // 脚本命令行参数（数量上限已在 executeTool 门控）
	WorkDir     string   // 任务工作区（cwd 与唯一可写区）
	SkillRoot   string   // 技能包根绝对路径（SKILL_ROOT 注入；工作区自建脚本为空）
	VenvDir     string   // 任务 venv 目录（非 venv 模式为空）
	NodeEnvDir  string   // 任务 Node 环境目录（node_modules 的父目录；无 npm: 依赖为空。独立于 VenvDir：容器内仅 venv 会把解释器覆盖为其 python3）
}

// dockerProbeTTL docker 后端 daemon/镜像探活结果的缓存时长。
// docker info 是子进程启动级别的开销（数十至数百毫秒），同一脚本执行轮次内
// 会被多次调用（门控 + 画像 + 实际执行），短 TTL 既能合并探测又保持故障后快速自愈
const dockerProbeTTL = 30 * time.Second

// containerDefaultPath 容器内默认 PATH（对齐主流镜像的镜像内默认值）
const containerDefaultPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// containerWsDir / containerSkillRoot / containerPyenvRoot 容器内路径映射约定：
// 任务工作区 → /workspace（唯一可写挂载）；技能包根 → /skill（只读）；
// 任务环境根（venv 与任务级 node_modules 的公共父目录）→ /pyenvs（只读）
const (
	containerWsDir     = "/workspace"
	containerSkillRoot = "/skill"
	containerPyenvRoot = "/pyenvs"
)

// buildSandboxChain 按配置构造沙箱后端链（顺序即优先级）：
//   - AGENT_SANDBOX_BACKEND=auto（默认）：按 GOOS 收录本机原生后端（darwin=seatbelt /
//     linux=bwrap），docker 后端配置了 AGENT_SANDBOX_IMAGE 才收录，plain 后端默认收录
//     作为链末位兜底（AGENT_SANDBOX_ALLOW_PLAIN=false 关闭兜底，安全优先）
//   - 显式指定后端名：链仅含该后端（不满足时 fail-closed，不静默降级）；
//     未知值记录错误并回退 auto
func buildSandboxChain(cfg config.SandboxConfig) []Sandbox {
	type candidate struct {
		name string
		sb   Sandbox
		def  bool // auto 模式下是否默认参与选择
	}
	candidates := []candidate{
		{name: "seatbelt", sb: &seatbeltSandbox{}, def: runtime.GOOS == "darwin"},
		{name: "bwrap", sb: &bwrapSandbox{}, def: runtime.GOOS == "linux"},
		{name: "docker", sb: &dockerSandbox{image: cfg.DockerImage}, def: cfg.DockerImage != ""},
		{name: "plain", sb: &plainSandbox{}, def: cfg.AllowPlain},
	}
	backend := strings.ToLower(strings.TrimSpace(cfg.Backend))
	if backend == "" {
		backend = "auto"
	}
	if backend != "auto" {
		for _, c := range candidates {
			if c.name == backend {
				return []Sandbox{c.sb}
			}
		}
		logger.Error("sandbox backend %q unknown (valid: auto/seatbelt/bwrap/docker/plain), falling back to auto", cfg.Backend)
	}
	var chain []Sandbox
	for _, c := range candidates {
		if c.def {
			chain = append(chain, c.sb)
		}
	}
	return chain
}

// absSpec 执行参数路径规整：script/wsDir 可能是相对服务进程 cwd 的路径，而本地后端
// 子进程 cwd=wsDir，相对 scriptPath 会被子进程错误解析到工作区之下 → 统一转绝对路径
func absSpec(spec *ExecSpec) {
	if abs, err := filepath.Abs(spec.Script); err == nil {
		spec.Script = abs
	}
	if abs, err := filepath.Abs(spec.WorkDir); err == nil {
		spec.WorkDir = abs
	}
	if spec.SkillRoot != "" {
		// 注意：filepath.Abs("") 会返回 cwd，空值必须先排除（工作区自建脚本无包根语义）
		if abs, err := filepath.Abs(spec.SkillRoot); err == nil {
			spec.SkillRoot = abs
		}
	}
}

// prepareWsTmp 准备脚本工作区临时目录（cwd/写入区 = 任务工作区，中间文件跨脚本共享），
// 创建失败时回退工作区根
func prepareWsTmp(wsDir string) string {
	tmpDir := filepath.Join(wsDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return wsDir
	}
	return tmpDir
}

// envRootOf 任务环境目录的父目录（= PyEnvManager.envRoot），作为沙箱挂载/读侧豁免单位；
// 无任务环境（venvDir 与 nodeEnvDir 均为空）时返回空。
// filepath.Dir("") 返回 "."，空值必须先排除
func envRootOf(venvDir, nodeEnvDir string) string {
	if venvDir != "" {
		return filepath.Dir(venvDir)
	}
	if nodeEnvDir != "" {
		return filepath.Dir(nodeEnvDir)
	}
	return ""
}

// resolveInterpreter 解释器可用性校验 + 自托管回退（白名单由调用方保证）。
// 返回 (可执行文件, "")：宿主 PATH 可寻址时原样返回（宿主优先语义）；
// node 系在宿主缺失时回退自托管 Node 运行时（<envRoot>/_node/bin/node，EnsureTaskNodeEnv
// 的自动引导产物）；两者皆无时返回 ("", 直接回传给模型的 TOOL ERROR 文本)。
// 此处只探测不下载——执行路径不做慢速引导，引导由 npm 依赖声明触发
func resolveInterpreter(interpreter, envRoot string) (string, string) {
	if _, err := exec.LookPath(interpreter); err == nil {
		return interpreter, ""
	}
	if interpreter == "node" || interpreter == "nodejs" {
		if bin := managedNodeBin(envRoot); bin != "" {
			node := filepath.Join(bin, "node")
			if info, err := os.Stat(node); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
				return node, ""
			}
		}
		return "", fmt.Sprintf("TOOL ERROR: interpreter %q not available on host (Node.js is not installed on the server; rewrite the logic as a Python script, or ask the user to install Node.js)", interpreter)
	}
	return "", fmt.Sprintf("TOOL ERROR: interpreter %q not available on host", interpreter)
}

// runCaptured 捕获式运行子进程（各后端共用的最终执行步骤）：
//   - 超时 scriptTimeout（到期 SIGKILL）
//   - stdout/stderr 各截断至 maxScriptOutput
//
// 返回回传给模型的文本（exit code + stdout + stderr）
func runCaptured(ctx context.Context, name string, argv []string, dir string, env []string) string {
	cctx, cancel := context.WithTimeout(ctx, scriptTimeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, name, argv...)
	cmd.Dir = dir
	cmd.Env = env
	outBuf := newLimitedBuffer(maxScriptOutput)
	errBuf := newLimitedBuffer(maxScriptOutput)
	cmd.Stdout = outBuf
	cmd.Stderr = errBuf
	runErr := cmd.Run()

	var b strings.Builder
	if cctx.Err() == context.DeadlineExceeded {
		fmt.Fprintf(&b, "TOOL ERROR: script timed out after %s (partial output below)\n", scriptTimeout)
	} else if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			fmt.Fprintf(&b, "exit code: %d\n", ee.ExitCode())
		} else {
			// 进程都没起来（如解释器损坏），直接报错
			return fmt.Sprintf("TOOL ERROR: exec failed: %v", runErr)
		}
	} else {
		b.WriteString("exit code: 0\n")
	}
	fmt.Fprintf(&b, "stdout:\n%s\nstderr:\n%s", outBuf.String(), errBuf.String())
	return b.String()
}

// seatbeltSandbox darwin 原生沙箱后端（sandbox-exec / Seatbelt）
type seatbeltSandbox struct{}

func (seatbeltSandbox) Name() string { return "seatbelt" }

func (seatbeltSandbox) Available() bool {
	_, err := exec.LookPath("sandbox-exec")
	return err == nil
}

func (seatbeltSandbox) SupportsVenv() bool { return true }

func (seatbeltSandbox) ProfileLine() string {
	return "execute_script 禁外网（DNS 解析、外网直连与系统代理通道均已封堵，任何外部 HTTP(S) 请求必然失败，不要尝试探测或下载外部资源），允许本机回环（可监听并连接 127.0.0.1，如启动本地 HTTP 服务）；文件写入仅限任务工作区与系统临时目录；宿主机敏感路径（其他任务工作区、配置与凭据文件）不可读。页面图片用内联 SVG/CSS 渐变等本地素材，字体用系统字体栈，配齐后直接产出页面并自检，无需再探测环境"
}

func (seatbeltSandbox) Exec(ctx context.Context, spec ExecSpec) string {
	absSpec(&spec)
	envRoot := envRootOf(spec.VenvDir, spec.NodeEnvDir)
	resolved, errMsg := resolveInterpreter(spec.Interpreter, envRoot)
	if errMsg != "" {
		return errMsg
	}
	tmpDir := prepareWsTmp(spec.WorkDir)
	sb, err := exec.LookPath("sandbox-exec")
	if err != nil {
		// Available 探测通过后环境突变（理论上罕见）：fail-closed，不落裸跑
		return "TOOL ERROR: sandbox-exec unavailable on this host"
	}
	argv := append([]string{"-p", buildSeatbeltProfile(resolvedPath(spec.WorkDir), resolvedPath(envRoot))},
		resolved, spec.Script)
	argv = append(argv, spec.Args...)
	return runCaptured(ctx, sb, argv, spec.WorkDir, buildScriptEnv(tmpDir, spec.WorkDir, spec.SkillRoot, spec.VenvDir, spec.NodeEnvDir, "seatbelt"))
}

// bwrapSandbox Linux 原生沙箱后端（bubblewrap）
type bwrapSandbox struct{}

func (bwrapSandbox) Name() string { return "bwrap" }

// Available linux 侧在 LookPath 之外做一次最小试运行：bwrap 依赖非特权用户命名空间等
// 内核能力，二进制存在但内核禁用（容器 seccomp 拦截 unshare 等）时同样视为不可用
// （fail-closed）
func (bwrapSandbox) Available() bool {
	bw, err := exec.LookPath("bwrap")
	if err != nil {
		return false
	}
	return bwrapProbe(bw)
}

func (bwrapSandbox) SupportsVenv() bool { return true }

func (bwrapSandbox) ProfileLine() string {
	return "execute_script 禁外网（独立网络命名空间，外网不可达），允许本机回环（可监听并连接 127.0.0.1，如启动本地 HTTP 服务）；仅任务工作区可写（系统临时目录独立挂载）；宿主机敏感路径（其他任务工作区、配置与凭据文件）已屏蔽；进程视图隔离。请直接依据上述事实编写脚本，无需再探测环境"
}

func (bwrapSandbox) Exec(ctx context.Context, spec ExecSpec) string {
	absSpec(&spec)
	envRoot := envRootOf(spec.VenvDir, spec.NodeEnvDir)
	resolved, errMsg := resolveInterpreter(spec.Interpreter, envRoot)
	if errMsg != "" {
		return errMsg
	}
	tmpDir := prepareWsTmp(spec.WorkDir)
	bw, err := exec.LookPath("bwrap")
	if err != nil {
		return "TOOL ERROR: bwrap unavailable on this host"
	}
	argv := append(buildBwrapArgv(spec.WorkDir, envRoot), resolved, spec.Script)
	argv = append(argv, spec.Args...)
	return runCaptured(ctx, bw, argv, spec.WorkDir, buildScriptEnv(tmpDir, spec.WorkDir, spec.SkillRoot, spec.VenvDir, spec.NodeEnvDir, "bwrap"))
}

// bwrapProbe 试运行最小 bwrap 沙箱，验证内核实际允许建立所需的命名空间。
// unshare 参数与 buildBwrapArgv 保持一致，确保探活结果对真实执行有效
func bwrapProbe(bwrap string) bool {
	probe := exec.Command(bwrap,
		"--die-with-parent", "--new-session",
		"--ro-bind", "/", "/",
		"--proc", "/proc", "--dev", "/dev",
		"--unshare-net", "--unshare-pid",
		"--", "/bin/true",
	)
	if out, err := probe.CombinedOutput(); err != nil {
		logger.Error("sandbox probe: bwrap present but not runnable on this host: %s", strings.TrimSpace(string(out)))
		return false
	}
	return true
}

// dockerSandbox 容器沙箱后端（跨平台可用，跨环境一致性最强）：
// 一次性容器执行脚本——禁网、根文件系统只读、PID/进程数受限，
// 仅任务工作区（/workspace）与容器临时区可写。
// 环境一致性由 AGENT_SANDBOX_IMAGE 指定的镜像保证（镜像内需含所需解释器/依赖）。
// 注意与 venv 模式的关系：托管 CPython 按宿主 OS 构建，仅 Linux 宿主的 venv 可进入
// 同平台容器（见 SupportsVenv）；macOS 宿主 + venv 模式时该后端会被自动跳过
type dockerSandbox struct {
	image  string     // 执行镜像（AGENT_SANDBOX_IMAGE；空=后端禁用）
	mu     sync.Mutex // 保护探活缓存
	lastOK time.Time  // 最近一次探活成功时刻（TTL 内直接放行）
}

func (d *dockerSandbox) Name() string { return "docker" }

// Available 探测 docker CLI、daemon 可达性与执行镜像存在性（带 TTL 缓存）。
// 镜像经 image inspect 校验而非拉取：缺失时视为后端不可用（由链中下一后端兜底），
// 避免执行期才暴露 image pull 失败
func (d *dockerSandbox) Available() bool {
	if d.image == "" {
		return false
	}
	bin, err := exec.LookPath("docker")
	if err != nil {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if time.Since(d.lastOK) < dockerProbeTTL {
		return true
	}
	if out, err := exec.Command(bin, "info").CombinedOutput(); err != nil {
		logger.Error("sandbox probe: docker daemon unreachable: %s", firstProbeLine(out, err))
		return false
	}
	if out, err := exec.Command(bin, "image", "inspect", d.image).CombinedOutput(); err != nil {
		logger.Error("sandbox probe: docker image %s unavailable: %s", d.image, firstProbeLine(out, err))
		return false
	}
	d.lastOK = time.Now()
	return true
}

func (d *dockerSandbox) SupportsVenv() bool { return runtime.GOOS == "linux" }

func (d *dockerSandbox) ProfileLine() string {
	return "execute_script 在一次性容器内运行：禁网；容器根文件系统只读，仅任务工作区（/workspace）与容器临时目录可写；进程数受限，宿主机文件系统不可见。请直接依据上述事实编写脚本，无需再探测环境"
}

// Exec 组装 docker run 一次性容器执行。环境变量在容器内重新组装：
// 基底为过滤后的宿主 env（保留 LANG 等无害项，敏感项已由 sanitizeEnv 剔除），
// PATH/TMPDIR/工作区相关变量按容器内路径语义覆盖
func (d *dockerSandbox) Exec(ctx context.Context, spec ExecSpec) string {
	absSpec(&spec)
	bin, err := exec.LookPath("docker")
	if err != nil {
		return "TOOL ERROR: docker sandbox unavailable on this host"
	}
	envRoot := envRootOf(spec.VenvDir, spec.NodeEnvDir)

	// 解释器/脚本映射为容器内路径：工作区脚本 → /workspace/<rel>，技能包脚本 → /skill/<rel>，
	// 任务 venv → /pyenvs/<taskID>（仅 venv 映射解释器；node 环境不改变解释器）
	interpreterIn := spec.Interpreter
	scriptIn := spec.Script
	venvIn := ""
	if spec.VenvDir != "" {
		venvIn = filepath.Join(containerPyenvRoot, filepath.Base(spec.VenvDir))
		interpreterIn = filepath.Join(venvIn, "bin", "python3")
	}
	if rel, err := filepath.Rel(spec.SkillRoot, spec.Script); spec.SkillRoot != "" && err == nil && !dottedEscape(rel) {
		scriptIn = filepath.ToSlash(filepath.Join(containerSkillRoot, rel))
	} else if rel, err := filepath.Rel(spec.WorkDir, spec.Script); err == nil && !dottedEscape(rel) {
		scriptIn = filepath.ToSlash(filepath.Join(containerWsDir, rel))
	}

	// 容器内 env：过滤后宿主 env 起底，再覆盖容器语义的关键项。
	// PATH 经局部变量统一组装（venv/node 前缀叠加后一次写入，避免相互覆盖）
	env := sanitizeEnv()
	pathIn := containerDefaultPath
	env = setEnvVar(env, "TMPDIR", containerWsDir+"/tmp") // 工作区内 tmp：中间文件跨脚本共享（语义对齐本地后端）
	env = setEnvVar(env, "SKILL_WORKSPACE", containerWsDir)
	if spec.SkillRoot != "" {
		env = setEnvVar(env, "SKILL_ROOT", containerSkillRoot)
	} else {
		env = unsetEnvVar(env, "SKILL_ROOT")
	}
	env = setEnvVar(env, "PYTHONDONTWRITEBYTECODE", "1")
	// 容器内同样缺 Chrome 内层沙箱所需特权（setuid helper / user namespaces），
	// 与本地沙箱后端语义一致：交由脚本侧 cdp.mjs 透传 --no-sandbox
	env = setEnvVar(env, "TASTE_CHROME_EXTRA_ARGS", "--no-sandbox")
	if spec.VenvDir != "" {
		pathIn = filepath.Join(venvIn, "bin") + ":" + pathIn
		env = setEnvVar(env, "VIRTUAL_ENV", venvIn)
		// 任务环境自洽：宿主残留的 Python 路径设置会把解释器引向意外环境，必须清除
		env = unsetEnvVar(env, "PYTHONHOME")
		env = unsetEnvVar(env, "PYTHONPATH")
	}
	if spec.NodeEnvDir != "" {
		nodeIn := filepath.Join(containerPyenvRoot, filepath.Base(spec.NodeEnvDir))
		nmIn := filepath.Join(nodeIn, "node_modules")
		// NODE_PATH 仅对 CJS require 生效（Node 限制，ESM import 不读取）
		pathIn = filepath.Join(nmIn, ".bin") + ":" + pathIn
		env = setEnvVar(env, "NODE_PATH", nmIn)
	}
	// 自托管 Node 运行时（宿主无 node 时的引导产物）：envRoot 非空时已只读挂载到 /pyenvs，
	// 容器内前置其 bin 使纯逻辑 node 脚本也能解析到 node；envRoot 为空则无挂载点，无法注入
	if envRoot != "" {
		if _, err := os.Stat(filepath.Join(envRoot, nodeToolsDir, "bin", "node")); err == nil {
			pathIn = filepath.Join(containerPyenvRoot, nodeToolsDir, "bin") + ":" + pathIn
		}
	}
	env = setEnvVar(env, "PATH", pathIn)

	// 挂载与隔离参数：工作区可写（宿主 uid:gid 保证产出文件归属正确），
	// 技能包/任务环境根只读，其余一律不可见
	argv := []string{
		"run", "--rm",
		"--network", "none",
		"--user", strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid()),
		"--pids-limit", "128",
		"--read-only",
		"--tmpfs", "/tmp",
		"-v", spec.WorkDir + ":" + containerWsDir,
		"-w", containerWsDir,
	}
	if spec.SkillRoot != "" {
		argv = append(argv, "-v", spec.SkillRoot+":"+containerSkillRoot+":ro")
	}
	if envRoot != "" {
		argv = append(argv, "-v", envRoot+":"+containerPyenvRoot+":ro")
	}
	for _, e := range env {
		argv = append(argv, "-e", e)
	}
	argv = append(argv, d.image, interpreterIn, scriptIn)
	argv = append(argv, spec.Args...)
	return runCaptured(ctx, bin, argv, "", env)
}

// dottedEscape 相对路径是否越出基准目录（.. 开头）或就是基准目录本身
func dottedEscape(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// firstProbeLine 探活失败输出摘要（多行输出取首个非空行，避免日志刷屏）
func firstProbeLine(out []byte, err error) string {
	_ = err
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return "no output"
}

// buildSeatbeltProfile 构造 Seatbelt profile：默认放行本机操作，禁外网，
// 放行本机回环（监听+连接，inspect 类脚本需起 127.0.0.1 HTTP 服务并驱动 Chrome
// 调试端口），文件写入仅限任务工作区与系统临时区，读取侧封堵敏感路径
// （解释器/系统库读取保持放行）。
// envRoot 非空（venv 模式）时作为读侧豁免传递（任务环境需只读可执行）。
// profile 必须以 (allow default) 起底——实测纯 deny 起底会导致 execvp EPERM。
// 回环放行必须用类限定过滤器（network-inbound 管 bind/listen、network-outbound 管 connect）：
// 实测 (allow network* (local ip "localhost:*")) 会因出站连接的 local endpoint 匹配
// localhost 而误放行全部出站连接；且 ip 过滤器的 host 只接受 * 或 localhost，
// 必须带端口（"127.0.0.1:*" / "0.0.0.0/0" 等形式 sandbox-exec 直接拒绝解析）。
// 回环放行后还须封堵系统代理回环端口（见 systemProxyLoopbackPorts）：
// Python urllib 会经系统代理配置自动连「回环上的代理端口」，由宿主代理进程转发
// 外网请求，外网实际可达——实测 T1 任务素材探测/下载循环 31 轮烧尽预算即此因
// unix socket 放行（network* + subpath）覆盖两类实测依赖：
//   - 工作区内 node 脚本间通信 socket；Chrome ProcessSingleton 的 SingletonSocket
//     创建在用户 Darwin 临时区（getconf DARWIN_USER_TEMP_DIR，Chrome 无视 TMPDIR），
//     需同步放行该目录——路径必须用解析 symlink 后的真实形态（/var→/private/var），
//     否则 subpath 过滤不生效
//
// Chrome 在外层 Seatbelt 下内层沙箱初始化失败（Operation not permitted），
// 由脚本侧经 TASTE_CHROME_EXTRA_ARGS=--no-sandbox 禁用其内层沙箱解决（见 buildScriptEnv）
func buildSeatbeltProfile(wsDir, envRoot string) string {
	var b strings.Builder
	b.WriteString("(version 1)(allow default)(deny network*)(deny file-write*)")
	b.WriteString(`(allow network-inbound (local ip "localhost:*"))`)
	b.WriteString(`(allow network-outbound (remote ip "localhost:*"))`)
	// 封堵系统代理回环端口。deny 恒优先于 allow（与子句顺序无关），紧随 allow
	// 仅为可读性：urllib 等读系统代理的 HTTP 客户端连 127.0.0.1:PORT 时命中 deny
	// （实测报 Operation not permitted），回环直连其余端口不受影响。
	// 残余风险：PAC/WPAD 代理无固定端口可枚举（本机 ProxyAutoConfigEnable:0），
	// 该场景下通道仍可能放行，由 ProfileLine 文案引导兜底
	for _, port := range systemProxyLoopbackPorts() {
		fmt.Fprintf(&b, `(deny network-outbound (remote ip "localhost:%d"))`, port)
	}
	// unix socket：Chrome ProcessSingleton 等本机 IPC 依赖；subpath 只覆盖工作区
	// 与 Darwin 临时区（外网 unix socket 连接不在此列，deny network* 仍兜底）
	b.WriteString("(allow network*")
	for _, d := range []string{wsDir, darwinUserTempDir()} {
		if d != "" {
			fmt.Fprintf(&b, " (subpath %q)", d)
		}
	}
	b.WriteString(")")
	b.WriteString("(allow file-write*")
	for _, d := range []string{wsDir, "/private/tmp", "/tmp", darwinUserTempDir()} {
		if d != "" {
			fmt.Fprintf(&b, " (subpath %q)", d)
		}
	}
	b.WriteString(` (literal "/dev/null"))`)
	b.WriteString(readDenyClauses(wsDir, envRoot))
	return b.String()
}

// darwinUserTempDir 用户级 Darwin 临时目录（confstr DARWIN_USER_TEMP_DIR，
// 形如 /var/folders/.../T——注意 getconf 返回值带尾斜杠且未解析 /var→/private/var
// symlink，Seatbelt subpath 过滤按真实路径匹配，必须 EvalSymlinks 规整）。
// Chrome/Chromium 的进程级 IPC（ProcessSingleton 的 SingletonSocket、Crashpad 等）
// 固定落在此目录且无视 TMPDIR 环境变量，沙箱需对其放行 unix socket 与文件写入。
// 非 darwin 平台或取值失败返回 ""（子句省略）
func darwinUserTempDir() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	out, err := exec.Command("getconf", "DARWIN_USER_TEMP_DIR").Output()
	if err != nil {
		return ""
	}
	dir := resolvedPath(strings.TrimSpace(string(out)))
	if dir == "" || !filepath.IsAbs(dir) {
		return ""
	}
	return dir
}

// systemProxyLoopbackPorts 收集 macOS 系统代理中指向回环地址且已启用的端口。
// 背景：Seatbelt 放行本机回环后，Python urllib 仍会经 getproxies_macosx_sysconf
// 自动读取系统代理（scutil --proxy）——「允许连接 127.0.0.1」意外放行了「连接
// 回环上的代理端口」，代理进程（宿主侧）替脚本转发外网请求，禁外网语义失真
// （T1 事故：素材探测/下载循环 31 轮烧尽预算）。封堵该端口后代理通道报
// Operation not permitted，回环 HTTP 服务不受影响。
// 非 darwin（bwrap/docker 有独立网络命名空间，无系统代理可借用）或 scutil
// 不可用/解析失败时返回 nil，profile 不发射 deny 子句（deny network* 兜底语义不变）。
// 残余风险：PAC/WPAD 代理端口无法枚举（见 buildSeatbeltProfile 注释）
func systemProxyLoopbackPorts() []int {
	if runtime.GOOS != "darwin" {
		return nil
	}
	out, err := exec.Command("scutil", "--proxy").Output()
	if err != nil {
		return nil
	}
	return parseSystemProxyLoopbackPorts(string(out))
}

// parseSystemProxyLoopbackPorts 解析 scutil --proxy 输出（键值行形如
// "HTTPEnable : 1"，host 值域名带引号、IP 不带）。按协议组取 Enable=1 且
// 代理 host 为回环地址的端口，去重后返回；顺序固定便于测试与日志稳定
func parseSystemProxyLoopbackPorts(scutilOut string) []int {
	kv := make(map[string]string)
	for _, line := range strings.Split(scutilOut, "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, " : ")
		if idx <= 0 {
			continue
		}
		kv[strings.TrimSpace(line[:idx])] = strings.Trim(strings.TrimSpace(line[idx+3:]), `"`)
	}
	loopback := map[string]bool{
		"127.0.0.1": true, "localhost": true, "::1": true, "0.0.0.0": true,
	}
	seen := make(map[int]bool)
	var ports []int
	for _, proto := range []string{"HTTP", "HTTPS", "FTP", "SOCKS"} {
		if kv[proto+"Enable"] != "1" || !loopback[kv[proto+"Proxy"]] {
			continue
		}
		port, err := strconv.Atoi(kv[proto+"Port"])
		if err != nil || port <= 0 || port > 65535 {
			continue
		}
		if !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}
	return ports
}

// readDenyClauses 构造读侧封堵子句。deny 恒优先于 allow，无法用通配再豁免本工作区，
// 因此兄弟工作区只能逐个枚举 deny（wsDir 需为 resolved 后的绝对路径）：
//   - 服务进程 cwd 下的 .env 类配置文件（数据库/服务凭据）
//   - 用户 HOME 下的凭据与敏感配置（.ssh/.aws/.gnupg 等）
//   - workspaceRoot 下除本任务工作区外的兄弟工作区（任务间数据隔离）；
//     envRoot 非空（venv 模式）时豁免该目录——任务环境（托管解释器、依赖包与 node_modules）需只读可执行，
//     envRoot 与各工作区同位于 workspaceRoot 之下，兄弟目录枚举会命中它
func readDenyClauses(wsDir, envRoot string) string {
	var b strings.Builder
	denyLiteral := func(path string) {
		if path != "" {
			fmt.Fprintf(&b, `(deny file-read* (literal %q))`, path)
		}
	}
	denySubpath := func(path string) {
		if path != "" {
			fmt.Fprintf(&b, `(deny file-read* (subpath %q))`, path)
		}
	}
	// 服务进程 cwd：.env 类配置
	if cwd, err := os.Getwd(); err == nil {
		for _, f := range []string{".env", ".env.local", ".env.production"} {
			denyLiteral(filepath.Join(cwd, f))
		}
	}
	// HOME：凭据文件用 literal，敏感目录用 subpath
	if home, err := os.UserHomeDir(); err == nil {
		for _, f := range []string{".netrc", ".git-credentials", ".env"} {
			denyLiteral(filepath.Join(home, f))
		}
		for _, d := range []string{".ssh", ".aws", ".gnupg", ".kube", ".docker", ".config/gcloud"} {
			denySubpath(filepath.Join(home, d))
		}
	}
	// 兄弟工作区：逐个枚举（deny 父目录会连本工作区一起封掉）
	root := filepath.Dir(wsDir)
	if entries, err := os.ReadDir(root); err == nil {
		self := filepath.Base(wsDir)
		for _, e := range entries {
			p := filepath.Join(root, e.Name())
			// 指向目录的 symlink 同样屏蔽（ReadDir.IsDir 为 Lstat 语义会漏判）
			if e.Name() == self || !maskableDir(p) {
				continue
			}
			// venv 模式：任务环境根豁免（解释器与依赖包需只读可执行）。
			// filepath.Abs 不解析 symlink，与 resolvedPath 形式可能不一致，双比较兜底
			if envRoot != "" {
				p := filepath.Join(root, e.Name())
				if p == envRoot || resolvedPath(p) == resolvedPath(envRoot) {
					continue
				}
			}
			denySubpath(p)
		}
	}
	return b.String()
}

// buildBwrapArgv 构造 bubblewrap 沙箱 argv（Linux 侧，边界对齐 darwin Seatbelt；需 Linux 实测）：
//   - --ro-bind / /：根文件系统只读起底（对应 allow default 的读侧放行）
//   - --bind wsDir wsDir：任务工作区为唯一可写目录
//   - --tmpfs /tmp：系统临时目录独立挂载（可写但与宿主机 /tmp 不通）
//   - --proc /proc --dev /dev --unshare-pid：重建伪文件系统 + 独立 PID 命名空间，
//     脚本看不到宿主机/其他任务进程，也无处可发信号
//   - --unshare-net：禁外网（等价 deny network*）。unshare 后 lo 存在但 DOWN，
//     回环可用需先拉起：bwrap 无原生参数，在沙箱内首个进程前以 ip link set lo up
//     启用（unshare 后无外网路由，外网仍不可达；无 ip 命令的环境回环不可用，可接受）
//   - 读侧屏蔽：兄弟工作区与 HOME 敏感目录挂空 tmpfs；.env 类凭据文件以 /dev/null 只读覆盖。
//     mask 前判存在（bwrap 对缺失路径直接报错退出，与 Seatbelt deny 对缺失路径无害不同），
//     并解析 symlink 后再挂，保证屏蔽作用于真实数据
//   - envRoot 非空（venv 模式）：追加 --ro-bind envRoot envRoot（后绑定覆盖根只读绑定，
//     使独立挂载的任务环境在沙箱内只读可见；兄弟 tmpfs 屏蔽对其豁免）
func buildBwrapArgv(wsDir, envRoot string) []string {
	argv := []string{
		"--die-with-parent", "--new-session",
		"--ro-bind", "/", "/",
		"--bind", wsDir, wsDir,
		"--tmpfs", "/tmp",
		"--proc", "/proc",
		"--dev", "/dev",
		"--unshare-net", "--unshare-pid",
	}
	// 拉起回环接口（unshare 后 lo DOWN）：以 sh -c 'ip link set lo up; exec ...' 包装，
	// ip 命令缺失/失败不阻断脚本执行（失败时回环不可用，与旧行为一致）
	argv = append(argv,
		"/bin/sh", "-c", "ip link set lo up 2>/dev/null || true; exec \"$@\"", "--",
	)
	// venv 模式：任务环境根只读进入沙箱（--ro-bind / 之下独立挂载路径不可见）。
	// envRoot 含其他任务的环境目录（venv/node_modules，仅依赖包可见，不含任务数据）与工具链 _uv/，均无需可写
	if envRoot != "" {
		argv = append(argv, "--ro-bind", envRoot, envRoot)
	}
	// 兄弟工作区挂空屏蔽（任务间数据隔离；本工作区已单独 bind 为可写）
	root := filepath.Dir(wsDir)
	if entries, err := os.ReadDir(root); err == nil {
		self := filepath.Base(wsDir)
		for _, e := range entries {
			p := filepath.Join(root, e.Name())
			// 指向目录的 symlink 同样屏蔽（ReadDir.IsDir 为 Lstat 语义会漏判）
			if e.Name() == self || !maskableDir(p) {
				continue
			}
			// venv 模式：任务环境根豁免（已显式 ro-bind，不参与 tmpfs 屏蔽）。
			// filepath.Abs 不解析 symlink，与 resolvedPath 形式可能不一致，双比较兜底
			if envRoot != "" {
				if p == envRoot || resolvedPath(p) == resolvedPath(envRoot) {
					continue
				}
			}
			argv = maskTmpfs(argv, p)
		}
	}
	// HOME：敏感目录挂空 + 凭据文件以 /dev/null 覆盖
	if home, err := os.UserHomeDir(); err == nil {
		for _, d := range []string{".ssh", ".aws", ".gnupg", ".kube", ".docker", ".config/gcloud"} {
			argv = maskTmpfs(argv, filepath.Join(home, d))
		}
		for _, f := range []string{".netrc", ".git-credentials", ".env"} {
			argv = maskFile(argv, filepath.Join(home, f))
		}
	}
	// 服务进程 cwd：.env 类配置以 /dev/null 覆盖
	if cwd, err := os.Getwd(); err == nil {
		for _, f := range []string{".env", ".env.local", ".env.production"} {
			argv = maskFile(argv, filepath.Join(cwd, f))
		}
	}
	return argv
}

// maskableDir 路径是否为需屏蔽的目录：真实目录或指向目录的符号链接。
// os.ReadDir 的 DirEntry.IsDir 为 Lstat 语义，symlink 目录会漏判
func maskableDir(path string) bool {
	fi, err := os.Stat(path) // 跟随链接
	return err == nil && fi.IsDir()
}

// maskTmpfs 以空 tmpfs 屏蔽目录（存在性检查 + symlink 解析，见 maskTarget）
func maskTmpfs(argv []string, path string) []string {
	if real := maskTarget(path); real != "" {
		return append(argv, "--tmpfs", real)
	}
	return argv
}

// maskFile 以 /dev/null 只读覆盖敏感文件（存在性检查 + symlink 解析，见 maskTarget）
func maskFile(argv []string, path string) []string {
	if real := maskTarget(path); real != "" {
		return append(argv, "--ro-bind", "/dev/null", real)
	}
	return argv
}

// maskTarget mask 目标规整：目标不存在时返回 ""（bwrap 对缺失路径直接报错退出，
// 与 Seatbelt deny 子句对缺失路径无害不同）；存在时解析 symlink 后返回——
// mount 语义虽会跟随链接，但以真实路径表达更确定，避免 dest 解析依赖沙箱内
// 逐层路径形态（如 /var→/private/var 类链接在不同发行版/命名空间布局下的差异）
func maskTarget(path string) string {
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return resolvedPath(path)
}

// resolvedPath 解析符号链接后的真实路径（Seatbelt 对 /var→/private/var 类链接需用真实路径匹配）
func resolvedPath(dir string) string {
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		return real
	}
	return dir
}

// plainSandbox 无沙箱直跑后端：脚本直接在宿主机运行（仅有超时与输出截断）。
// 默认作为 auto 链末位兜底（全部沙箱后端不可用时保证任务不中断）；
// AGENT_SANDBOX_ALLOW_PLAIN=false 可关闭兜底（fail-closed），也可显式锁定该后端
type plainSandbox struct{}

func (plainSandbox) Name() string { return "plain" }

func (plainSandbox) Available() bool { return true }

func (plainSandbox) SupportsVenv() bool { return true }

func (plainSandbox) ProfileLine() string {
	return "当前宿主机未配置执行沙箱，execute_script 将直接在宿主机运行（仍有超时与输出截断）。请避免在脚本中访问任务工作区以外的系统路径"
}

func (plainSandbox) Exec(ctx context.Context, spec ExecSpec) string {
	absSpec(&spec)
	envRoot := envRootOf(spec.VenvDir, spec.NodeEnvDir)
	resolved, errMsg := resolveInterpreter(spec.Interpreter, envRoot)
	if errMsg != "" {
		return errMsg
	}
	tmpDir := prepareWsTmp(spec.WorkDir)
	// runCaptured 的 name 已是解释器本体，argv 只需脚本与参数（重复放解释器会被当作脚本读取）
	argv := append([]string{spec.Script}, spec.Args...)
	return runCaptured(ctx, resolved, argv, spec.WorkDir, buildScriptEnv(tmpDir, spec.WorkDir, spec.SkillRoot, spec.VenvDir, spec.NodeEnvDir, ""))
}
