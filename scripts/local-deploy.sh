#!/bin/bash

# OrbStack 本地 K8s 部署脚本
# 使用方法: ./scripts/local-deploy.sh [namespace]

set -e

# 默认参数
NAMESPACE=${1:-"default"}
IMAGE_NAME="skytree:latest"

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
    
    # 检查OrbStack K8s是否运行
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Kubernetes 集群未运行，请启动 OrbStack"
        exit 1
    fi
    
    log_success "依赖检查通过"
}

# 构建镜像
build_image() {
    log_info "构建Docker镜像: ${IMAGE_NAME}"
    
    docker build -t "${IMAGE_NAME}" .
    
    if [ $? -eq 0 ]; then
        log_success "镜像构建成功"
    else
        log_error "镜像构建失败"
        exit 1
    fi
}

# 加载镜像到OrbStack
load_image_to_orbstack() {
    log_info "加载镜像到 OrbStack..."
    
    # OrbStack 会自动使用本地构建的镜像
    # 不需要额外加载步骤
    log_success "镜像已准备就绪"
}

# 部署到K8s
deploy_to_k8s() {
    log_info "部署到本地 Kubernetes 集群..."
    
    # 应用本地配置
    kubectl apply -f deploy/k8s/standalone/local-configmap.yaml -n ${NAMESPACE}
    kubectl apply -f deploy/k8s/standalone/local-deployment.yaml -n ${NAMESPACE}
    kubectl apply -f deploy/k8s/standalone/local-service.yaml -n ${NAMESPACE}
    
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
    
    # 获取NodePort信息
    NODEPORT_1883=$(kubectl get service skytree-service -n ${NAMESPACE} -o jsonpath='{.spec.ports[0].nodePort}')
    NODEPORT_8080=$(kubectl get service skytree-service -n ${NAMESPACE} -o jsonpath='{.spec.ports[1].nodePort}')
    NODEPORT_9526=$(kubectl get service skytree-service -n ${NAMESPACE} -o jsonpath='{.spec.ports[3].nodePort}')
    
    echo "访问信息:"
    echo "  MQTT TCP:   localhost:${NODEPORT_1883}"
    echo "  MQTT WS:    ws://localhost:${NODEPORT_8080}"
    echo "  HTTP API:   http://localhost:${NODEPORT_9526}"
    echo ""
    echo "集群内部访问:"
    echo "  MQTT TCP:   skytree-internal:1883"
    echo "  MQTT WS:    ws://skytree-internal:8080"
    echo "  HTTP API:   http://skytree-internal:9526"
    echo "=========================================="
}

# 显示管理命令
show_management_commands() {
    echo ""
    echo "管理命令:"
    echo "  查看Pod:     kubectl get pods -l app=skytree -n ${NAMESPACE}"
    echo "  查看日志:    kubectl logs -l app=skytree -n ${NAMESPACE}"
    echo "  进入Pod:     kubectl exec -it skytree-0 -n ${NAMESPACE} -- /bin/bash"
    echo "  删除部署:    kubectl delete -f deploy/k8s/standalone/local-*.yaml -n ${NAMESPACE}"
    echo "  重启部署:    kubectl rollout restart statefulset skytree -n ${NAMESPACE}"
    echo ""
    echo "测试命令:"
    echo "  健康检查:    curl http://localhost:${NODEPORT_9526}/health"
    echo "  MQTT测试:    mosquitto_pub -h localhost -p ${NODEPORT_1883} -t 'test' -m 'hello'"
    echo ""
}

# 测试服务
test_service() {
    log_info "测试服务连接..."
    
    # 获取NodePort
    NODEPORT_9526=$(kubectl get service skytree-service -n ${NAMESPACE} -o jsonpath='{.spec.ports[3].nodePort}')
    
    # 等待服务启动
    sleep 10
    
    # 测试健康检查
    if curl -s http://localhost:${NODEPORT_9526}/health > /dev/null; then
        log_success "HTTP API 服务正常"
    else
        log_warning "HTTP API 服务可能未就绪"
    fi
    
    # 测试MQTT端口
    if nc -z localhost ${NODEPORT_1883} 2>/dev/null; then
        log_success "MQTT TCP 端口正常"
    else
        log_warning "MQTT TCP 端口可能未就绪"
    fi
}

# 主函数
main() {
    echo "SkyTree OrbStack 本地部署脚本"
    echo "=============================="
    echo "镜像: ${IMAGE_NAME}"
    echo "命名空间: ${NAMESPACE}"
    echo "=============================="
    echo ""
    
    check_dependencies
    build_image
    load_image_to_orbstack
    deploy_to_k8s
    wait_for_deployment
    show_status
    test_service
    show_management_commands
    
    log_success "本地部署完成！"
}

# 清理函数
cleanup() {
    log_info "清理临时文件..."
    # 这里可以添加清理逻辑
}

# 设置错误时清理
trap cleanup EXIT

# 显示帮助
show_help() {
    echo "使用方法: $0 [namespace]"
    echo ""
    echo "参数:"
    echo "  namespace   K8s命名空间 (默认: default)"
    echo ""
    echo "示例:"
    echo "  $0 default"
    echo "  $0 skytree"
    echo ""
    echo "选项:"
    echo "  -h, --help  显示此帮助信息"
    echo ""
    echo "前置条件:"
    echo "  1. OrbStack 已安装并运行"
    echo "  2. Kubernetes 集群已启动"
    echo "  3. Docker 已安装"
    echo "  4. kubectl 已配置"
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
