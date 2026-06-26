#!/bin/bash

# 阿里云K8s部署脚本
# 使用方法: ./scripts/aliyun-deploy.sh [namespace] [tag]

set -e

# 默认参数
NAMESPACE=${1:-"skytree"}
TAG=${2:-"latest"}
REGISTRY="registry.cn-hangzhou.aliyuncs.com"
IMAGE_NAME="skytree/skytree"
FULL_IMAGE_NAME="${REGISTRY}/${IMAGE_NAME}:${TAG}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

# 检查依赖
check_dependencies() {
    log_info "检查依赖..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装"
        exit 1
    fi
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl 未安装"
        exit 1
    fi
    
    # 检查Docker登录状态
    if ! docker info &> /dev/null; then
        log_error "Docker 守护进程未运行"
        exit 1
    fi
    
    log_success "依赖检查通过"
}

# 构建镜像
build_image() {
    log_info "构建Docker镜像: ${FULL_IMAGE_NAME}"
    
    docker build -t "${FULL_IMAGE_NAME}" .
    
    if [ $? -eq 0 ]; then
        log_success "镜像构建成功"
    else
        log_error "镜像构建失败"
        exit 1
    fi
}

# 推送镜像
push_image() {
    log_info "推送镜像到阿里云仓库..."
    
    # 检查是否已登录
    if ! docker info | grep -q "Username"; then
        log_warning "请先登录阿里云容器镜像服务:"
        echo "docker login ${REGISTRY}"
        read -p "按回车键继续..."
    fi
    
    docker push "${FULL_IMAGE_NAME}"
    
    if [ $? -eq 0 ]; then
        log_success "镜像推送成功"
    else
        log_error "镜像推送失败，请检查登录状态"
        exit 1
    fi
}

# 更新K8s配置
update_k8s_config() {
    log_info "更新K8s配置文件..."
    
    # 备份原文件
    cp deploy/k8s/standalone/deployment.yaml deploy/k8s/standalone/deployment.yaml.bak
    
    # 更新镜像地址
    sed -i.tmp "s|image: skytree:latest|image: ${FULL_IMAGE_NAME}|g" deploy/k8s/standalone/deployment.yaml
    
    # 更新命名空间
    sed -i.tmp "s|namespace: default|namespace: ${NAMESPACE}|g" deploy/k8s/standalone/*.yaml
    
    # 清理临时文件
    rm -f deploy/k8s/standalone/*.yaml.tmp
    
    log_success "K8s配置更新完成"
}

# 部署到K8s
deploy_to_k8s() {
    log_info "部署到Kubernetes集群..."
    
    # 创建命名空间
    kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -
    
    # 应用配置
    kubectl apply -f deploy/k8s/standalone/configmap.yaml -n ${NAMESPACE}
    kubectl apply -f deploy/k8s/standalone/deployment.yaml -n ${NAMESPACE}
    kubectl apply -f deploy/k8s/standalone/service.yaml -n ${NAMESPACE}
    
    log_success "K8s部署完成"
}

# 等待部署完成
wait_for_deployment() {
    log_info "等待Pod启动..."
    
    # 等待StatefulSet就绪
    kubectl wait --for=condition=ready pod -l app=skytree -n ${NAMESPACE} --timeout=300s
    
    if [ $? -eq 0 ]; then
        log_success "所有Pod已就绪"
    else
        log_warning "Pod启动超时，请检查状态"
    fi
}

# 显示部署状态
show_status() {
    log_info "部署状态:"
    echo "=========================================="
    
    # 显示Pod状态
    echo "Pod状态:"
    kubectl get pods -l app=skytree -n ${NAMESPACE}
    echo ""
    
    # 显示服务状态
    echo "服务状态:"
    kubectl get services -l app=skytree -n ${NAMESPACE}
    echo ""
    
    # 显示StatefulSet状态
    echo "StatefulSet状态:"
    kubectl get statefulset -l app=skytree -n ${NAMESPACE}
    echo ""
    
    # 获取外部IP
    EXTERNAL_IP=$(kubectl get service skytree-service -n ${NAMESPACE} -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
    
    echo "访问信息:"
    echo "  LoadBalancer IP: ${EXTERNAL_IP}"
    echo "  MQTT TCP:   ${EXTERNAL_IP}:1883"
    echo "  MQTT WS:    ws://${EXTERNAL_IP}:8080"
    echo "  HTTP API:   http://${EXTERNAL_IP}:9526"
    echo "=========================================="
}

# 显示管理命令
show_management_commands() {
    echo ""
    echo "管理命令:"
    echo "  查看Pod:     kubectl get pods -l app=skytree -n ${NAMESPACE}"
    echo "  查看日志:    kubectl logs -l app=skytree -n ${NAMESPACE}"
    echo "  进入Pod:     kubectl exec -it skytree-0 -n ${NAMESPACE} -- /bin/bash"
    echo "  删除部署:    kubectl delete -f deploy/k8s/standalone/ -n ${NAMESPACE}"
    echo "  重启部署:    kubectl rollout restart statefulset skytree -n ${NAMESPACE}"
    echo ""
}

# 主函数
main() {
    echo "SkyTree 阿里云K8s部署脚本"
    echo "=========================="
    echo "镜像: ${FULL_IMAGE_NAME}"
    echo "命名空间: ${NAMESPACE}"
    echo "=========================="
    echo ""
    
    check_dependencies
    build_image
    push_image
    update_k8s_config
    deploy_to_k8s
    wait_for_deployment
    show_status
    show_management_commands
    
    log_success "部署完成！"
}

# 清理函数
cleanup() {
    log_info "清理临时文件..."
    rm -f deploy/k8s/standalone/*.yaml.tmp
}

# 设置错误时清理
trap cleanup EXIT

# 显示帮助
show_help() {
    echo "使用方法: $0 [namespace] [tag]"
    echo ""
    echo "参数:"
    echo "  namespace   K8s命名空间 (默认: skytree)"
    echo "  tag         镜像标签 (默认: latest)"
    echo ""
    echo "示例:"
    echo "  $0 skytree v1.0.0"
    echo "  $0 production latest"
    echo ""
    echo "选项:"
    echo "  -h, --help  显示此帮助信息"
}

# 处理命令行参数
case "${1:-}" in
    -h|--help)
        show_help
        exit 0
        ;;
    *)
        main
        ;;
esac
