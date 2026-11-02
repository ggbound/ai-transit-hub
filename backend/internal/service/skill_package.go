package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/your-org/ai-transit-hub/internal/models"
)

// SKILL.md 技能包的上传/下载约束
const (
	skillMaxArchiveSize  = 20 << 20 // 上传 zip 上限 20MB
	skillMaxTotalUnpack  = 50 << 20 // 解压后总大小上限 50MB
	skillMaxFileCount    = 200      // 包内文件数量上限
	skillMaxFileSize     = 10 << 20 // 单个文件上限 10MB
	skillEntryNameMaxLen = 64       // frontmatter name 最大长度（规范要求）
	skillDescMaxLen      = 1024     // frontmatter description 最大长度（规范要求）
)

// skillNamePattern 遵循 Agent Skills 规范：小写字母/数字/连字符，1-64 字符
var skillNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// SkillMeta 从 SKILL.md frontmatter 解析出的元数据
type SkillMeta struct {
	Name         string
	Description  string
	Version      string
	License      string
	AllowedTools string
	Frontmatter  models.JSON
}

// PackageFile 技能包内的一个文件
type PackageFile struct {
	Path string
	Data []byte
}

// ExtractedPackage 上传解析后的技能包
type ExtractedPackage struct {
	Meta     SkillMeta
	Body     string // SKILL.md frontmatter 之后的 Markdown 正文
	MainFile PackageFile
	Assets   []PackageFile // 除 SKILL.md 外的附件
}

// ValidateSkillName 校验技能标识格式
func ValidateSkillName(name string) error {
	if name == "" {
		return fmt.Errorf("name 不能为空")
	}
	if len(name) > skillEntryNameMaxLen {
		return fmt.Errorf("name 长度不能超过 %d 字符", skillEntryNameMaxLen)
	}
	if !skillNamePattern.MatchString(name) {
		return fmt.Errorf("name 仅允许小写字母、数字和连字符，且需以字母或数字开头")
	}
	return nil
}

// ParseSkillMarkdown 解析 SKILL.md：拆分 YAML frontmatter 与 Markdown 正文
func ParseSkillMarkdown(content string) (meta SkillMeta, body string, err error) {
	fmBytes, body, err := splitFrontmatter(content)
	if err != nil {
		return meta, "", err
	}

	if len(fmBytes) > 0 {
		var fm map[string]interface{}
		if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
			return meta, "", fmt.Errorf("frontmatter YAML 解析失败: %w", err)
		}

		meta.Name = yamlToString(fm["name"])
		meta.Description = yamlToString(fm["description"])
		meta.Version = yamlToString(fm["version"])
		meta.License = yamlToString(fm["license"])
		meta.AllowedTools = yamlListToString(fm["allowed-tools"])
		if meta.AllowedTools == "" {
			meta.AllowedTools = yamlListToString(fm["allowed_tools"])
		}

		// 原始 frontmatter 转成 JSON 存储
		if fmJSON, jsonErr := json.Marshal(fm); jsonErr == nil {
			meta.Frontmatter = models.JSON(fmJSON)
		}
	}

	return meta, body, nil
}

// ExtractSkillPackage 解析上传的 zip 技能包。
// 兼容两种结构：SKILL.md 位于 zip 根目录，或位于唯一的一级目录内。
func ExtractSkillPackage(data []byte) (*ExtractedPackage, error) {
	if len(data) > skillMaxArchiveSize {
		return nil, fmt.Errorf("上传文件超过大小限制（%dMB）", skillMaxArchiveSize>>20)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("无效的 zip 文件: %w", err)
	}
	if len(zr.File) == 0 || len(zr.File) > skillMaxFileCount {
		return nil, fmt.Errorf("包内文件数量不在允许范围（1-%d）", skillMaxFileCount)
	}

	// 读取全部条目（安全校验 + 找到 SKILL.md）
	type entry struct {
		relPath string
		data    []byte
	}
	entries := make([]entry, 0, len(zr.File))
	prefix := detectSkillRootPrefix(zr.File)
	var mainFile *entry

	var totalUnpacked int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || isSkillJunkEntry(f.Name) {
			continue
		}
		relPath, err := safeSkillEntryPath(f.Name, prefix)
		if err != nil {
			return nil, err
		}
		if relPath == "" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("读取包内文件失败: %w", err)
		}
		data, err := io.ReadAll(io.LimitReader(rc, skillMaxFileSize+1))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("读取包内文件失败: %w", err)
		}
		if int64(len(data)) > skillMaxFileSize {
			return nil, fmt.Errorf("包内文件 %s 超过单文件大小限制", relPath)
		}

		totalUnpacked += int64(len(data))
		if totalUnpacked > skillMaxTotalUnpack {
			return nil, fmt.Errorf("包内解压总大小超过限制（%dMB）", skillMaxTotalUnpack>>20)
		}

		e := entry{relPath: relPath, data: data}
		entries = append(entries, e)
		if relPath == "SKILL.md" {
			mainFile = &entries[len(entries)-1]
		}
	}

	if mainFile == nil {
		return nil, fmt.Errorf("包内未找到 SKILL.md 文件")
	}

	meta, body, err := ParseSkillMarkdown(string(mainFile.data))
	if err != nil {
		return nil, err
	}
	if err := ValidateSkillName(meta.Name); err != nil {
		return nil, fmt.Errorf("SKILL.md frontmatter 校验失败: %w", err)
	}
	if meta.Description == "" {
		return nil, fmt.Errorf("SKILL.md frontmatter 缺少 description")
	}
	if len(meta.Description) > skillDescMaxLen {
		return nil, fmt.Errorf("description 长度不能超过 %d 字符", skillDescMaxLen)
	}

	pkg := &ExtractedPackage{
		Meta:     meta,
		Body:     body,
		MainFile: PackageFile{Path: "SKILL.md", Data: mainFile.data},
	}
	for _, e := range entries {
		if e.relPath != "SKILL.md" {
			pkg.Assets = append(pkg.Assets, PackageFile{Path: e.relPath, Data: e.data})
		}
	}
	return pkg, nil
}

// BuildSkillZip 将技能打包为 zip（SKILL.md 以数据库内容为准，附件从磁盘读取）
func BuildSkillZip(skill *models.Skill, files []models.SkillFile, storageRoot string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	if err := writeZipEntry(zw, "SKILL.md", []byte(skill.Content)); err != nil {
		return nil, err
	}
	for _, f := range files {
		data, err := readSkillAsset(storageRoot, skill.StoragePath, f.Path)
		if err != nil {
			continue // 附件缺失时不阻断打包
		}
		if err := writeZipEntry(zw, f.Path, data); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// skillPackageRoot 技能包根目录：附件读取与 SKILL_ROOT 注入共用同一拼法
// （storage_path 本身即包根相对路径，为空时 fallback 到附件存储根）
func skillPackageRoot(storageRoot, skillStoragePath string) string {
	if skillStoragePath == "" {
		return storageRoot
	}
	return skillStoragePath
}

// SkillFilePath 计算附件在磁盘上的绝对路径（含 zip-slip 防护）
func SkillFilePath(storageRoot, skillStoragePath, relPath string) (string, error) {
	base := skillPackageRoot(storageRoot, skillStoragePath)
	clean := filepath.Clean(filepath.Join("/", relPath)) // 以 / 为根规整，去掉 ..
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." {
		return "", fmt.Errorf("非法的文件路径")
	}
	return filepath.Join(base, clean), nil
}

// readSkillAsset 读取磁盘上的附件内容
func readSkillAsset(storageRoot, skillStoragePath, relPath string) ([]byte, error) {
	p, err := SkillFilePath(storageRoot, skillStoragePath, relPath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

// splitFrontmatter 拆开文件头部的 --- YAML frontmatter 块
func splitFrontmatter(content string) (frontmatter []byte, body string, err error) {
	norm := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(norm, "---\n") {
		return nil, strings.TrimSpace(norm), nil
	}
	rest := norm[4:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, "", fmt.Errorf("frontmatter 未正确闭合（缺少结束的 ---）")
	}
	fm := rest[:idx]
	after := rest[idx+4:]
	after = strings.TrimPrefix(after, "\n")
	return []byte(fm), strings.TrimSpace(after), nil
}

// detectSkillRootPrefix 判断包内文件所在的一级目录（如 my-skill/SKILL.md）。
// 优先定位包含 SKILL.md 的一级目录；若根目录直接存在 SKILL.md 则无需前缀。
func detectSkillRootPrefix(files []*zip.File) string {
	// 根目录直接存在 SKILL.md
	for _, f := range files {
		if !f.FileInfo().IsDir() && !isSkillJunkEntry(f.Name) && filepath.ToSlash(f.Name) == "SKILL.md" {
			return ""
		}
	}

	// SKILL.md 位于某个一级目录内（如 taste-skill/SKILL.md）
	var prefix string
	for _, f := range files {
		if f.FileInfo().IsDir() || isSkillJunkEntry(f.Name) {
			continue
		}
		name := filepath.ToSlash(f.Name)
		if !strings.HasSuffix(name, "/SKILL.md") {
			continue
		}
		dir := name[:len(name)-len("/SKILL.md")]
		if strings.Contains(dir, "/") {
			continue
		}
		if prefix == "" {
			prefix = dir
		} else if prefix != dir {
			return "" // 多个目录都含 SKILL.md，无法判定
		}
	}
	return prefix
}

// isSkillJunkEntry 判断是否为 macOS 打包产生的垃圾条目（__MACOSX 资源分叉、.DS_Store 等）
func isSkillJunkEntry(name string) bool {
	slashName := filepath.ToSlash(name)
	if slashName == "__MACOSX" || strings.HasPrefix(slashName, "__MACOSX/") {
		return true
	}
	base := path.Base(slashName)
	return base == ".DS_Store" || strings.HasPrefix(base, "._")
}

// safeSkillEntryPath 校验并规整包内路径，返回相对 prefix 的路径（zip-slip 防护）
func safeSkillEntryPath(name, prefix string) (string, error) {
	slashName := filepath.ToSlash(name)
	if strings.Contains(slashName, "..") || strings.HasPrefix(slashName, "/") {
		return "", fmt.Errorf("包内包含非法路径: %s", name)
	}
	clean := path.Clean(slashName)
	if clean == "." || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("包内包含非法路径: %s", name)
	}
	if prefix != "" {
		if !strings.HasPrefix(clean, prefix+"/") {
			// 顶层散文件（如 README.md）与技能目录并存时忽略目录外文件
			return "", nil
		}
		clean = strings.TrimPrefix(clean, prefix+"/")
	}
	return clean, nil
}

func yamlToString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	}
}

// yamlListToString 将 YAML 列表/逗号分隔字符串规整为逗号分隔字符串
func yamlListToString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		parts := strings.Split(t, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return strings.Join(nonEmpty(parts), ",")
	case []interface{}:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s := yamlToString(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ",")
	default:
		return yamlToString(t)
	}
}

func nonEmpty(parts []string) []string {
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func writeZipEntry(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func skillMimeType(ext string) string {
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}
