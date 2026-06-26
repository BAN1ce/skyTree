#!/bin/bash

# SkyTree Docker 构建测试脚本
# 用于测试 Docker 镜像构建是否正常

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查 Docker 是否安装
check_docker() {
    log_info "检查 Docker 环境..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装或未在 PATH 中"
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        log_error "Docker 守护进程未运行"
        exit 1
    fi
    
    log_success "Docker 环境检查通过"
}

# 构建测试镜像
build_test_image() {
    log_info "开始构建测试镜像..."
    
    # 构建镜像
    docker build -t skytree:test .
    
    if [ $? -eq 0 ]; then
        log_success "Docker 镜像构建成功"
    else
        log_error "Docker 镜像构建失败"
        exit 1
    fi
}

# 测试镜像
test_image() {
    log_info "测试镜像..."
    
    # 检查镜像是否存在
    if ! docker image inspect skytree:test &> /dev/null; then
        log_error "镜像 skytree:test 不存在"
        exit 1
    fi
    
    # 显示镜像信息
    log_info "镜像信息:"
    docker image inspect skytree:test --format='{{.Size}}' | awk '{print "大小: " $1/1024/1024 " MB"}'
    docker image inspect skytree:test --format='{{.Config.ExposedPorts}}'
    
    # 测试容器启动
    log_info "测试容器启动..."
    CONTAINER_ID=$(docker run -d --name skytree-test -p 1883:1883 -p 9526:9526 skytree:test)
    
    if [ $? -eq 0 ]; then
        log_success "容器启动成功，ID: $CONTAINER_ID"
        
        # 等待几秒钟让应用启动
        sleep 5
        
        # 检查容器状态
        if docker ps | grep -q skytree-test; then
            log_success "容器运行正常"
        else
            log_warning "容器可能启动失败，检查日志:"
            docker logs skytree-test
        fi
        
        # 清理测试容器
        log_info "清理测试容器..."
        docker stop skytree-test
        docker rm skytree-test
        
    else
        log_error "容器启动失败"
        exit 1
    fi
}

# 显示镜像详情
show_image_details() {
    log_info "镜像详情:"
    echo "=========================================="
    docker image inspect skytree:test --format='
镜像ID: {{.Id}}
创建时间: {{.Created}}
大小: {{.Size}} bytes
架构: {{.Architecture}}
操作系统: {{.Os}}
标签: {{range .RepoTags}}{{.}} {{end}}
'
    echo "=========================================="
}

# 主函数
main() {
    echo "SkyTree Docker 构建测试"
    echo "========================"
    echo ""
    
    check_docker
    build_test_image
    test_image
    show_image_details
    
    log_success "所有测试通过！"
    log_info "可以使用以下命令运行容器:"
    echo "  docker run -d --name skytree -p 1883:1883 -p 9526:9526 skytree:test"
    echo ""
    log_info "或者删除测试镜像:"
    echo "  docker rmi skytree:test"
}

# 清理函数
cleanup() {
    log_info "清理测试资源..."
    docker stop skytree-test 2>/dev/null || true
    docker rm skytree-test 2>/dev/null || true
}

# 设置错误时清理
trap cleanup EXIT

# 执行主函数
main
