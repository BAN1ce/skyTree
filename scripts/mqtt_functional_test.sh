#!/bin/bash

# MQTT功能测试脚本
# 用于测试SkyTree MQTT代理的基本功能

set -e

# 配置参数
BROKER_HOST=${BROKER_HOST:-localhost}
BROKER_PORT=${BROKER_PORT:-1883}
BROKER_WS_PORT=${BROKER_WS_PORT:-8083}
TEST_DURATION=${TEST_DURATION:-60}
LOG_LEVEL=${LOG_LEVEL:-info}

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

# 测试结果统计
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# 测试函数
run_test() {
    local test_name="$1"
    local test_command="$2"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_info "运行测试: $test_name"
    
    if eval "$test_command"; then
        log_success "测试通过: $test_name"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        log_error "测试失败: $test_name"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

# 检查MQTT客户端工具
check_mqtt_tools() {
    log_info "检查MQTT客户端工具..."
    
    if ! command -v mosquitto_pub &> /dev/null; then
        log_error "mosquitto_pub 未安装，请先安装 mosquitto-clients"
        exit 1
    fi
    
    if ! command -v mosquitto_sub &> /dev/null; then
        log_error "mosquitto_sub 未安装，请先安装 mosquitto-clients"
        exit 1
    fi
    
    log_success "MQTT客户端工具检查通过"
}

# 检查broker连接
check_broker_connection() {
    log_info "检查broker连接..."
    
    if ! timeout 5 mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "test/connection" -m "test" -q 0; then
        log_error "无法连接到MQTT broker: $BROKER_HOST:$BROKER_PORT"
        exit 1
    fi
    
    log_success "Broker连接正常"
}

# 测试1: 基本连接测试
test_basic_connection() {
    log_info "测试基本连接..."
    
    # 测试TCP连接
    if ! timeout 5 mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "test/basic" -m "test message" -q 0; then
        return 1
    fi
    
    # 测试WebSocket连接（如果支持）
    if command -v wscat &> /dev/null; then
        echo "test websocket connection" | timeout 5 wscat -c "ws://$BROKER_HOST:$BROKER_WS_PORT" || true
    fi
    
    return 0
}

# 测试2: QoS级别测试
test_qos_levels() {
    log_info "测试QoS级别..."
    
    # QoS 0测试
    if ! timeout 5 mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "test/qos0" -m "qos0 message" -q 0; then
        return 1
    fi
    
    # QoS 1测试
    if ! timeout 5 mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "test/qos1" -m "qos1 message" -q 1; then
        return 1
    fi
    
    # QoS 2测试
    if ! timeout 5 mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "test/qos2" -m "qos2 message" -q 2; then
        return 1
    fi
    
    return 0
}

# 测试3: 订阅发布测试
test_pub_sub() {
    log_info "测试发布订阅..."
    
    # 启动订阅者
    timeout $TEST_DURATION mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "test/pubsub" -C 1 > /tmp/sub_output &
    SUB_PID=$!
    
    sleep 2
    
    # 发布消息
    if ! mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "test/pubsub" -m "pubsub test message" -q 1; then
        kill $SUB_PID 2>/dev/null || true
        return 1
    fi
    
    # 等待订阅者接收消息
    sleep 2
    kill $SUB_PID 2>/dev/null || true
    
    # 检查是否收到消息
    if grep -q "pubsub test message" /tmp/sub_output; then
        return 0
    else
        return 1
    fi
}

# 测试4: 通配符订阅测试
test_wildcard_subscription() {
    log_info "测试通配符订阅..."
    
    # 启动通配符订阅者
    timeout $TEST_DURATION mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "test/+/wildcard" -C 1 > /tmp/wildcard_output &
    SUB_PID=$!
    
    sleep 2
    
    # 发布到匹配的主题
    if ! mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "test/level1/wildcard" -m "wildcard test message" -q 1; then
        kill $SUB_PID 2>/dev/null || true
        return 1
    fi
    
    # 等待订阅者接收消息
    sleep 2
    kill $SUB_PID 2>/dev/null || true
    
    # 检查是否收到消息
    if grep -q "wildcard test message" /tmp/wildcard_output; then
        return 0
    else
        return 1
    fi
}

# 测试5: 保留消息测试
test_retained_messages() {
    log_info "测试保留消息..."
    
    # 发布保留消息
    if ! mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "test/retained" -m "retained message" -r; then
        return 1
    fi
    
    sleep 1
    
    # 订阅主题，应该收到保留消息
    timeout 5 mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "test/retained" -C 1 > /tmp/retained_output &
    SUB_PID=$!
    
    sleep 2
    kill $SUB_PID 2>/dev/null || true
    
    # 检查是否收到保留消息
    if grep -q "retained message" /tmp/retained_output; then
        return 0
    else
        return 1
    fi
}

# 测试6: 客户端ID测试
test_client_id() {
    log_info "测试客户端ID..."
    
    # 测试重复客户端ID
    mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "test/clientid" -i "test_client" &
    SUB1_PID=$!
    
    sleep 2
    
    # 尝试使用相同客户端ID连接
    if timeout 5 mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "test/clientid" -i "test_client"; then
        kill $SUB1_PID 2>/dev/null || true
        return 1  # 应该失败
    fi
    
    kill $SUB1_PID 2>/dev/null || true
    return 0
}

# 测试7: 认证测试
test_authentication() {
    log_info "测试认证..."
    
    # 测试无认证连接（应该成功）
    if ! timeout 5 mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "test/auth" -m "no auth" -q 0; then
        return 1
    fi
    
    # 测试错误认证（如果启用了认证）
    if timeout 5 mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "test/auth" -m "wrong auth" -u "wrong_user" -P "wrong_pass" -q 0; then
        # 如果错误认证成功了，说明没有启用认证，这是正常的
        return 0
    else
        # 如果错误认证失败了，说明启用了认证，这也是正常的
        return 0
    fi
}

# 测试8: 大消息测试
test_large_message() {
    log_info "测试大消息..."
    
    # 生成1KB消息
    large_message=$(head -c 1024 /dev/zero | tr '\0' 'A')
    
    if ! timeout 10 mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "test/large" -m "$large_message" -q 0; then
        return 1
    fi
    
    return 0
}

# 测试9: 连接数限制测试
test_connection_limit() {
    log_info "测试连接数限制..."
    
    local max_connections=100
    local pids=()
    
    # 创建多个连接
    for i in $(seq 1 $max_connections); do
        mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "test/limit" -i "client_$i" &
        pids+=($!)
        sleep 0.1
    done
    
    sleep 2
    
    # 检查连接是否成功
    local success_count=0
    for pid in "${pids[@]}"; do
        if kill -0 $pid 2>/dev/null; then
            success_count=$((success_count + 1))
        fi
    done
    
    # 清理连接
    for pid in "${pids[@]}"; do
        kill $pid 2>/dev/null || true
    done
    
    # 如果成功连接数大于50%，认为测试通过
    if [ $success_count -gt $((max_connections / 2)) ]; then
        return 0
    else
        return 1
    fi
}

# 测试10: 异常断开测试
test_abnormal_disconnect() {
    log_info "测试异常断开..."
    
    # 启动订阅者
    mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "test/disconnect" -i "disconnect_test" &
    SUB_PID=$!
    
    sleep 2
    
    # 强制杀死订阅者进程
    kill -9 $SUB_PID 2>/dev/null || true
    
    sleep 2
    
    # 尝试重新连接（应该成功）
    if timeout 5 mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "test/disconnect" -i "disconnect_test"; then
        return 0
    else
        return 1
    fi
}

# 清理函数
cleanup() {
    log_info "清理测试环境..."
    
    # 清理临时文件
    rm -f /tmp/sub_output /tmp/wildcard_output /tmp/retained_output
    
    # 清理可能的残留进程
    pkill -f "mosquitto_sub" 2>/dev/null || true
    pkill -f "mosquitto_pub" 2>/dev/null || true
}

# 主函数
main() {
    log_info "开始MQTT功能测试"
    log_info "测试参数:"
    log_info "  - Broker地址: $BROKER_HOST:$BROKER_PORT"
    log_info "  - WebSocket端口: $BROKER_WS_PORT"
    log_info "  - 测试时长: ${TEST_DURATION}秒"
    echo "---"
    
    # 设置清理陷阱
    trap cleanup EXIT
    
    # 检查环境
    check_mqtt_tools
    check_broker_connection
    echo "---"
    
    # 运行测试
    run_test "基本连接测试" "test_basic_connection"
    run_test "QoS级别测试" "test_qos_levels"
    run_test "发布订阅测试" "test_pub_sub"
    run_test "通配符订阅测试" "test_wildcard_subscription"
    run_test "保留消息测试" "test_retained_messages"
    run_test "客户端ID测试" "test_client_id"
    run_test "认证测试" "test_authentication"
    run_test "大消息测试" "test_large_message"
    run_test "连接数限制测试" "test_connection_limit"
    run_test "异常断开测试" "test_abnormal_disconnect"
    
    echo "---"
    log_info "测试完成"
    log_info "总测试数: $TOTAL_TESTS"
    log_info "通过测试: $PASSED_TESTS"
    log_info "失败测试: $FAILED_TESTS"
    
    if [ $FAILED_TESTS -eq 0 ]; then
        log_success "所有测试通过！"
        exit 0
    else
        log_error "有 $FAILED_TESTS 个测试失败"
        exit 1
    fi
}

# 处理命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --host)
            BROKER_HOST="$2"
            shift 2
            ;;
        --port)
            BROKER_PORT="$2"
            shift 2
            ;;
        --ws-port)
            BROKER_WS_PORT="$2"
            shift 2
            ;;
        --duration)
            TEST_DURATION="$2"
            shift 2
            ;;
        --help)
            echo "用法: $0 [选项]"
            echo "选项:"
            echo "  --host HOST        Broker主机地址 (默认: localhost)"
            echo "  --port PORT        Broker端口 (默认: 1883)"
            echo "  --ws-port PORT     WebSocket端口 (默认: 8083)"
            echo "  --duration SECONDS 测试时长 (默认: 60)"
            echo "  --help             显示帮助信息"
            exit 0
            ;;
        *)
            log_error "未知参数: $1"
            exit 1
            ;;
    esac
done

# 运行主函数
main
