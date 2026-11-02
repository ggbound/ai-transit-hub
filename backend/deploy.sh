#!/bin/bash
# AI Transit Hub 后端部署脚本
# 部署路径: /data/sites/aitb/backend

# 配置
PROJECT_DIR="/data/sites/aitb/backend"
SERVICE_NAME="aitb-backend"
GO_VERSION="1.21"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 日志函数
log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 检查命令是否存在
check_command() {
    if ! command -v "$1" &> /dev/null; then
        log_error "命令 $1 未找到，请先安装"
        exit 1
    fi
}

# 部署主函数
main() {
    log_info "=========================================="
    log_info "AI Transit Hub 后端部署脚本"
    log_info "=========================================="
    echo ""

    # 1. 检查目录
    log_info "1. 检查部署目录..."
    if [ ! -d "$PROJECT_DIR" ]; then
        log_error "目录 $PROJECT_DIR 不存在，请先创建并上传代码"
        exit 1
    fi
    cd "$PROJECT_DIR" || { log_error "无法进入目录 $PROJECT_DIR"; exit 1; }
    log_info "✓ 目录检查完成"
    echo ""

    # 2. 检查Go环境
    log_info "2. 检查Go环境..."
    check_command go
    go_version=$(go version)
    log_info "✓ $go_version"
    echo ""

    # 3. 检查.env文件
    log_info "3. 检查环境配置..."
    if [ ! -f ".env" ]; then
        log_warn ".env 文件不存在，正在从 .env.example 复制..."
        if [ -f ".env.example" ]; then
            cp .env.example .env
            log_warn "✓ 已复制 .env.example 到 .env"
            log_warn "⚠️  请务必编辑 .env 文件配置正确的数据库连接信息！"
        else
            log_error ".env.example 文件也不存在"
            exit 1
        fi
    else
        log_info "✓ .env 文件已存在"
    fi
    echo ""

    # 4. 下载依赖
    log_info "4. 下载Go依赖..."
    go mod download
    if [ $? -ne 0 ]; then
        log_error "依赖下载失败"
        exit 1
    fi
    log_info "✓ 依赖下载完成"
    echo ""

    # 5. 编译项目
    log_info "5. 编译项目..."
    if [ -f "server" ]; then
        rm -f server
        log_info "  - 清理旧编译文件"
    fi
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server cmd/server/main.go
    if [ $? -ne 0 ]; then
        log_error "编译失败"
        exit 1
    fi
    chmod +x server
    log_info "✓ 编译完成，文件大小: $(du -h server)"
    echo ""

    # 6. 停止旧服务
    log_info "6. 停止旧服务..."
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        systemctl stop "$SERVICE_NAME"
        log_info "✓ 服务已停止"
    else
        log_info "✓ 服务未运行"
    fi
    echo ""

    # 7. 检查systemd服务文件
    log_info "7. 检查systemd服务配置..."
    SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME.service"
    if [ ! -f "$SERVICE_FILE" ]; then
        log_warn "systemd服务文件不存在，正在创建..."
        create_service_file
        log_warn "✓ 服务文件已创建，请运行: systemctl enable $SERVICE_NAME"
    else
        log_info "✓ 服务文件已存在"
    fi
    echo ""

    # 8. 启动新服务
    log_info "8. 启动服务..."
    systemctl daemon-reload
    systemctl start "$SERVICE_NAME"
    if [ $? -ne 0 ]; then
        log_error "服务启动失败"
        log_error "查看日志: journalctl -u $SERVICE_NAME -n 50"
        exit 1
    fi
    log_info "✓ 服务已启动"
    echo ""

    # 9. 检查服务状态
    log_info "9. 检查服务状态..."
    sleep 3
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        log_info "✓ 服务运行状态: 正常"
    else
        log_error "✗ 服务运行状态: 异常"
        log_error "查看日志: journalctl -u $SERVICE_NAME -n 50"
        exit 1
    fi
    echo ""

    # 10. 验证健康检查
    log_info "10. 验证服务健康..."
    # 读取端口配置
    PORT=$(grep -E '^SERVER_PORT=' .env | cut -d'=' -f2 | tr -d '[:space:]' || echo "8080")
    HEALTH_URL="http://localhost:$PORT/health"
    log_info "  - 健康检查地址: $HEALTH_URL"
    for i in {1..10}; do
        if curl -s "$HEALTH_URL" &> /dev/null; then
            log_info "✓ 健康检查通过"
            break
        fi
        if [ "$i" -eq 10 ]; then
            log_error "✗ 健康检查失败"
            log_error "查看日志: journalctl -u $SERVICE_NAME -n 50"
            exit 1
        fi
        log_info "  - 等待服务启动... ($i/10)"
        sleep 2
    done
    echo ""

    log_info "=========================================="
    log_info "✅ 部署完成！"
    log_info "=========================================="
    log_info "服务状态: systemctl status $SERVICE_NAME"
    log_info "查看日志: journalctl -u $SERVICE_NAME -f"
    log_info "停止服务: systemctl stop $SERVICE_NAME"
    log_info "重启服务: systemctl restart $SERVICE_NAME"
    log_info "=========================================="
}

# 创建systemd服务文件
create_service_file() {
    sudo cat > "/etc/systemd/system/$SERVICE_NAME.service" <<EOF
[Unit]
Description=AI Transit Hub Backend Service
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=$PROJECT_DIR
ExecStart=$PROJECT_DIR/server -mode release
Restart=on-failure
RestartSec=5
StartLimitInterval=60s
StartLimitBurst=3

# 环境变量
EnvironmentFile=$PROJECT_DIR/.env

# 资源限制
NoFile=65536
LimitNOFILE=65536

# 日志配置
StandardOutput=journal
StandardError=journal
SyslogIdentifier=$SERVICE_NAME

[Install]
WantedBy=multi-user.target
EOF

    log_info "服务文件已创建: /etc/systemd/system/$SERVICE_NAME.service"
    echo ""
    log_info "服务文件内容:"
    cat "/etc/systemd/system/$SERVICE_NAME.service"
    echo ""
}

# 显示帮助信息
show_help() {
    cat <<EOF
AI Transit Hub 后端部署脚本

使用方法:
  ./deploy.sh          - 完整部署
  ./deploy.sh quick    - 快速部署（跳过依赖下载，仅重新编译和重启）
  ./deploy.sh help     - 显示帮助

部署前准备:
  1. 确保代码已上传到: $PROJECT_DIR
  2. 确保已安装 Go $GO_VERSION
  3. 确保已配置好 .env 文件
  4. 确保 MySQL 数据库已创建并可连接
EOF
}

# 快速部署模式
quick_deploy() {
    log_info "=========================================="
    log_info "AI Transit Hub 快速部署模式"
    log_info "=========================================="
    echo ""

    cd "$PROJECT_DIR" || { log_error "无法进入目录 $PROJECT_DIR"; exit 1; }

    log_info "1. 重新编译..."
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server cmd/server/main.go
    if [ $? -ne 0 ]; then
        log_error "编译失败"
        exit 1
    fi
    chmod +x server
    log_info "✓ 编译完成"
    echo ""

    log_info "2. 重启服务..."
    systemctl restart "$SERVICE_NAME"
    log_info "✓ 服务已重启"
    echo ""

    log_info "3. 检查状态..."
    sleep 3
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        log_info "✓ 服务正常运行"
    else
        log_error "✗ 服务异常"
        exit 1
    fi
    echo ""

    log_info "✅ 快速部署完成！"
}

# 执行
if [ "$1" = "quick" ]; then
    quick_deploy
elif [ "$1" = "help" ] || [ "$1" = "-h" ]; then
    show_help
else
    main
fi
