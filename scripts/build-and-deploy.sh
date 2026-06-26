#!/bin/bash

# SkyTree Docker 构建和 K8s 部署脚本
# 使用方法: ./scripts/build-and-deploy.sh [registry] [tag] [namespace]

set -e

# 默认参数
REGISTRY=${1:-"registry.cn-hangzhou.aliyuncs.com/your-namespace"}
TAG=${2:-"latest"}
NAMESPACE=${3:-"default"}
IMAGE_NAME="skytree"
FULL_IMAGE_NAME="${REGISTRY}/${IMAGE_NAME}:${TAG}"

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

# 检查依赖
check_dependencies() {
    log_info "检查依赖..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装或未在 PATH 中"
        exit 1
    fi
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl 未安装或未在 PATH 中"
        exit 1
    fi
    
    log_success "依赖检查通过"
}

# 构建 Docker 镜像
build_image() {
    log_info "开始构建 Docker 镜像: ${FULL_IMAGE_NAME}"
    
    # 构建镜像
    docker build -t "${FULL_IMAGE_NAME}" .
    
    if [ $? -eq 0 ]; then
        log_success "Docker 镜像构建成功"
    else
        log_error "Docker 镜像构建失败"
        exit 1
    fi
}

# 推送镜像到仓库
push_image() {
    log_info "推送镜像到仓库: ${FULL_IMAGE_NAME}"
    
    # 登录阿里云容器镜像服务（如果需要）
    # docker login registry.cn-hangzhou.aliyuncs.com
    
    # 推送镜像
    docker push "${FULL_IMAGE_NAME}"
    
    if [ $? -eq 0 ]; then
        log_success "镜像推送成功"
    else
        log_error "镜像推送失败"
        exit 1
    fi
}

# 更新 K8s 部署配置
update_k8s_config() {
    log_info "更新 K8s 部署配置..."
    
    # 更新 deployment.yaml 中的镜像
    sed -i.bak "s|image: skytree:latest|image: ${FULL_IMAGE_NAME}|g" deploy/k8s/standalone/deployment.yaml
    
    # 更新 namespace
    sed -i.bak "s|namespace: default|namespace: ${NAMESPACE}|g" deploy/k8s/standalone/*.yaml
    
    log_success "K8s 配置更新完成"
}

# 部署到 K8s
deploy_to_k8s() {
    log_info "部署到 Kubernetes..."
    
    # 创建命名空间（如果不存在）
    kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -
    
    # 应用配置
    kubectl apply -f deploy/k8s/standalone/configmap.yaml -n ${NAMESPACE}
    kubectl apply -f deploy/k8s/standalone/deployment.yaml -n ${NAMESPACE}
    kubectl apply -f deploy/k8s/standalone/service.yaml -n ${NAMESPACE}
    
    log_success "K8s 部署完成"
}

# 检查部署状态
check_deployment() {
    log_info "检查部署状态..."
    
    # 等待 Pod 就绪
    kubectl wait --for=condition=ready pod -l app=skytree -n ${NAMESPACE} --timeout=300s
    
    # 显示 Pod 状态
    kubectl get pods -l app=skytree -n ${NAMESPACE}
    
    # 显示服务状态
    kubectl get services -l app=skytree -n ${NAMESPACE}
    
    log_success "部署状态检查完成"
}

# 显示访问信息
show_access_info() {
    log_info "获取访问信息..."
    
    # 获取 LoadBalancer 外部 IP
    EXTERNAL_IP=$(kubectl get service skytree-service -n ${NAMESPACE} -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
    
    echo ""
    echo "=========================================="
    echo "SkyTree MQTT Broker 部署完成！"
    echo "=========================================="
    echo "镜像: ${FULL_IMAGE_NAME}"
    echo "命名空间: ${NAMESPACE}"
    echo ""
    echo "访问地址:"
    echo "  MQTT TCP:   ${EXTERNAL_IP}:1883"
    echo "  MQTT WS:    ws://${EXTERNAL_IP}:8080"
    echo "  MQTT WSS:   wss://${EXTERNAL_IP}:8081"
    echo "  HTTP API:   http://${EXTERNAL_IP}:9526"
    echo ""
    echo "NodePort 访问 (如果 LoadBalancer 不可用):"
    echo "  MQTT TCP:   <node-ip>:30183"
    echo "  MQTT WS:    ws://<node-ip>:30080"
    echo "  MQTT WSS:   wss://<node-ip>:30081"
    echo "  HTTP API:   http://<node-ip>:30526"
    echo ""
    echo "管理命令:"
    echo "  查看 Pod:   kubectl get pods -l app=skytree -n ${NAMESPACE}"
    echo "  查看日志:   kubectl logs -l app=skytree -n ${NAMESPACE}"
    echo "  删除部署:   kubectl delete -f deploy/k8s/standalone/ -n ${NAMESPACE}"
    echo "=========================================="
}

# 清理函数
cleanup() {
    log_info "清理临时文件..."
    rm -f deploy/k8s/standalone/*.yaml.bak
}

# 主函数
main() {
    echo "SkyTree Docker 构建和 K8s 部署脚本"
    echo "=================================="
    echo "镜像仓库: ${REGISTRY}"
    echo "镜像标签: ${TAG}"
    echo "命名空间: ${NAMESPACE}"
    echo "完整镜像名: ${FULL_IMAGE_NAME}"
    echo "=================================="
    echo ""
    
    # 设置错误时清理
    trap cleanup EXIT
    
    # 执行步骤
    check_dependencies
    build_image
    
    # 询问是否推送镜像
    read -p "是否推送镜像到仓库? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        push_image
    else
        log_warning "跳过镜像推送"
    fi
    
    # 询问是否部署到 K8s
    read -p "是否部署到 Kubernetes? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        update_k8s_config
        deploy_to_k8s
        check_deployment
        show_access_info
    else
        log_warning "跳过 K8s 部署"
    fi
    
    log_success "脚本执行完成！"
}

# 显示帮助信息
show_help() {
    echo "使用方法: $0 [registry] [tag] [namespace]"
    echo ""
    echo "参数:"
    echo "  registry   镜像仓库地址 (默认: registry.cn-hangzhou.aliyuncs.com/your-namespace)"
    echo "  tag        镜像标签 (默认: latest)"
    echo "  namespace  K8s 命名空间 (默认: default)"
    echo ""
    echo "示例:"
    echo "  $0 registry.cn-hangzhou.aliyuncs.com/my-namespace v1.0.0 production"
    echo "  $0 my-registry.com/skytree dev staging"
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
