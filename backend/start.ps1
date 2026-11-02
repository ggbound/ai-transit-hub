# AI Transit Hub - Go Backend 启动脚本
Write-Host "========================================" -ForegroundColor Cyan
Write-Host " AI Transit Hub - Go Backend" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 检查Go是否安装
Write-Host "[1/5] 检查 Go 环境..." -ForegroundColor Yellow
try {
    $goVersion = go version 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "    ✓ Go 已安装: $goVersion" -ForegroundColor Green
    } else {
        Write-Host "    ✗ Go 未安装" -ForegroundColor Red
        Write-Host ""
        Write-Host "请先安装 Go：" -ForegroundColor Yellow
        Write-Host "1. 访问 https://golang.org/dl/"
        Write-Host "2. 下载并安装 Windows 版本"
        Write-Host "3. 重新打开终端，然后再次运行此脚本"
        Write-Host ""
        Read-Host "按任意键退出"
        exit 1
    }
} catch {
    Write-Host "    ✗ Go 未安装" -ForegroundColor Red
    Write-Host ""
    Write-Host "请先安装 Go：" -ForegroundColor Yellow
    Write-Host "1. 访问 https://golang.org/dl/"
    Write-Host "2. 下载并安装 Windows 版本"
    Write-Host "3. 重新打开终端，然后再次运行此脚本"
    Write-Host ""
    Read-Host "按任意键退出"
    exit 1
}

Write-Host ""

# 检查.env文件
Write-Host "[2/5] 检查配置文件..." -ForegroundColor Yellow
if (Test-Path ".env") {
    Write-Host "    ✓ .env 文件已存在" -ForegroundColor Green
} else {
    Write-Host "    ✗ .env 文件不存在，从 .env.example 复制..." -ForegroundColor Yellow
    Copy-Item ".env.example" ".env"
    Write-Host "    ✓ .env 文件已创建，请配置数据库连接" -ForegroundColor Green
}
Write-Host ""

# 下载依赖
Write-Host "[3/5] 下载 Go 依赖..." -ForegroundColor Yellow
Write-Host "    正在运行 go mod download..." -ForegroundColor Gray
try {
    go mod download
    if ($LASTEXITCODE -eq 0) {
        Write-Host "    ✓ 依赖下载完成" -ForegroundColor Green
    } else {
        Write-Host "    ⚠ 依赖下载可能有问题，但继续尝试..." -ForegroundColor Yellow
    }
} catch {
    Write-Host "    ⚠ 依赖下载可能有问题，但继续尝试..." -ForegroundColor Yellow
}
Write-Host ""

# 检查主程序文件
Write-Host "[4/5] 检查项目文件..." -ForegroundColor Yellow
if (Test-Path "cmd/server/main.go") {
    Write-Host "    ✓ 主程序文件存在" -ForegroundColor Green
} else {
    Write-Host "    ✗ 主程序文件不存在!" -ForegroundColor Red
    Read-Host "按任意键退出"
    exit 1
}
Write-Host ""

# 启动服务
Write-Host "[5/5] 启动服务器..." -ForegroundColor Green
Write-Host ""
Write-Host "服务器将在 http://localhost:8080 启动" -ForegroundColor Cyan
Write-Host ""
Write-Host "按 Ctrl+C 停止服务器" -ForegroundColor Gray
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 运行服务器
go run cmd/server/main.go
