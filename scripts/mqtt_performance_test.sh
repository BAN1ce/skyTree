#!/bin/bash

# MQTT性能测试脚本
# 用于测试SkyTree MQTT代理的性能表现

set -e

# 配置参数
BROKER_HOST=${BROKER_HOST:-localhost}
BROKER_PORT=${BROKER_PORT:-1883}
TEST_DURATION=${TEST_DURATION:-300}
CONCURRENT_CLIENTS=${CONCURRENT_CLIENTS:-100}
MESSAGE_RATE=${MESSAGE_RATE:-1000}
MESSAGE_SIZE=${MESSAGE_SIZE:-1024}
QOS_LEVEL=${QOS_LEVEL:-0}

# 性能测试结果
PERFORMANCE_RESULTS="/tmp/mqtt_performance_results.json"

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
    
    if ! command -v mosquitto_pub &> /dev/null; then
        log_error "mosquitto_pub 未安装"
        exit 1
    fi
    
    if ! command -v mosquitto_sub &> /dev/null; then
        log_error "mosquitto_sub 未安装"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        log_warning "jq 未安装，将使用基础JSON输出"
    fi
    
    log_success "依赖检查完成"
}

# 检查broker连接
check_broker() {
    log_info "检查broker连接..."
    
    if ! timeout 5 mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "test/connection" -m "test" -q 0; then
        log_error "无法连接到broker: $BROKER_HOST:$BROKER_PORT"
        exit 1
    fi
    
    log_success "Broker连接正常"
}

# 获取系统信息
get_system_info() {
    log_info "获取系统信息..."
    
    local cpu_info=$(lscpu | grep "Model name" | cut -d: -f2 | xargs)
    local memory_info=$(free -h | grep "Mem:" | awk '{print $2}')
    local os_info=$(lsb_release -d | cut -d: -f2 | xargs)
    
    echo "系统信息:"
    echo "  CPU: $cpu_info"
    echo "  内存: $memory_info"
    echo "  操作系统: $os_info"
    echo "  Broker: $BROKER_HOST:$BROKER_PORT"
}

# 并发连接测试
test_concurrent_connections() {
    log_info "测试并发连接..."
    
    local connections=0
    local pids=()
    local start_time=$(date +%s)
    
    # 创建并发连接
    for i in $(seq 1 $CONCURRENT_CLIENTS); do
        mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "perf/connections" -i "perf_client_$i" &
        pids+=($!)
        connections=$((connections + 1))
        
        # 每10个连接暂停一下
        if [ $((i % 10)) -eq 0 ]; then
            sleep 0.1
        fi
    done
    
    local end_time=$(date +%s)
    local connection_time=$((end_time - start_time))
    
    # 等待连接稳定
    sleep 5
    
    # 检查活跃连接数
    local active_connections=0
    for pid in "${pids[@]}"; do
        if kill -0 $pid 2>/dev/null; then
            active_connections=$((active_connections + 1))
        fi
    done
    
    # 清理连接
    for pid in "${pids[@]}"; do
        kill $pid 2>/dev/null || true
    done
    
    local connection_rate=$((connections / connection_time))
    local success_rate=$((active_connections * 100 / connections))
    
    log_info "并发连接测试结果:"
    log_info "  尝试连接数: $connections"
    log_info "  成功连接数: $active_connections"
    log_info "  连接成功率: ${success_rate}%"
    log_info "  连接建立时间: ${connection_time}秒"
    log_info "  连接建立速率: ${connection_rate}连接/秒"
    
    # 保存结果
    echo "{
        \"test\": \"concurrent_connections\",
        \"attempted_connections\": $connections,
        \"successful_connections\": $active_connections,
        \"success_rate\": $success_rate,
        \"connection_time\": $connection_time,
        \"connection_rate\": $connection_rate
    }" > $PERFORMANCE_RESULTS
}

# 消息吞吐量测试
test_message_throughput() {
    log_info "测试消息吞吐量..."
    
    local topic="perf/throughput"
    local message_count=0
    local start_time=$(date +%s.%N)
    
    # 启动订阅者
    timeout $TEST_DURATION mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "$topic" -C 0 > /tmp/throughput_output &
    SUB_PID=$!
    
    sleep 2
    
    # 启动发布者
    local publish_start=$(date +%s.%N)
    timeout $TEST_DURATION bash -c "
        while true; do
            mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t '$topic' -m '$(head -c $MESSAGE_SIZE /dev/zero | tr '\0' 'A')' -q $QOS_LEVEL
            sleep 0.001
        done
    " &
    PUB_PID=$!
    
    # 等待测试完成
    wait $PUB_PID 2>/dev/null || true
    local publish_end=$(date +%s.%N)
    
    # 停止订阅者
    kill $SUB_PID 2>/dev/null || true
    sleep 2
    
    # 统计结果
    local publish_duration=$(echo "$publish_end - $publish_start" | bc -l)
    local received_messages=$(wc -l < /tmp/throughput_output)
    local throughput=$(echo "scale=2; $received_messages / $publish_duration" | bc -l)
    
    log_info "消息吞吐量测试结果:"
    log_info "  测试时长: ${publish_duration}秒"
    log_info "  接收消息数: $received_messages"
    log_info "  消息大小: ${MESSAGE_SIZE}字节"
    log_info "  QoS级别: $QOS_LEVEL"
    log_info "  吞吐量: ${throughput}消息/秒"
    
    # 更新结果文件
    if [ -f $PERFORMANCE_RESULTS ]; then
        local existing_results=$(cat $PERFORMANCE_RESULTS)
        echo "$existing_results" | jq ". + {
            \"throughput_test\": {
                \"duration\": $publish_duration,
                \"messages_received\": $received_messages,
                \"message_size\": $MESSAGE_SIZE,
                \"qos_level\": $QOS_LEVEL,
                \"throughput\": $throughput
            }
        }" > $PERFORMANCE_RESULTS
    else
        echo "{
            \"throughput_test\": {
                \"duration\": $publish_duration,
                \"messages_received\": $received_messages,
                \"message_size\": $MESSAGE_SIZE,
                \"qos_level\": $QOS_LEVEL,
                \"throughput\": $throughput
            }
        }" > $PERFORMANCE_RESULTS
    fi
}

# 延迟测试
test_latency() {
    log_info "测试消息延迟..."
    
    local topic="perf/latency"
    local latencies=()
    local total_latency=0
    local message_count=100
    
    # 启动延迟测试订阅者
    timeout 30 mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "$topic" -C $message_count > /tmp/latency_output &
    SUB_PID=$!
    
    sleep 2
    
    # 发送带时间戳的消息
    for i in $(seq 1 $message_count); do
        local send_time=$(date +%s.%N)
        mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "$topic" -m "$send_time" -q $QOS_LEVEL
        sleep 0.1
    done
    
    # 等待订阅者完成
    wait $SUB_PID 2>/dev/null || true
    
    # 计算延迟
    local min_latency=999999
    local max_latency=0
    
    while IFS= read -r line; do
        local receive_time=$(date +%s.%N)
        local send_time=$line
        local latency=$(echo "scale=6; ($receive_time - $send_time) * 1000" | bc -l)
        
        latencies+=($latency)
        total_latency=$(echo "$total_latency + $latency" | bc -l)
        
        if (( $(echo "$latency < $min_latency" | bc -l) )); then
            min_latency=$latency
        fi
        
        if (( $(echo "$latency > $max_latency" | bc -l) )); then
            max_latency=$latency
        fi
    done < /tmp/latency_output
    
    local avg_latency=$(echo "scale=6; $total_latency / ${#latencies[@]}" | bc -l)
    
    log_info "延迟测试结果:"
    log_info "  测试消息数: ${#latencies[@]}"
    log_info "  平均延迟: ${avg_latency}ms"
    log_info "  最小延迟: ${min_latency}ms"
    log_info "  最大延迟: ${max_latency}ms"
    
    # 更新结果文件
    if [ -f $PERFORMANCE_RESULTS ]; then
        local existing_results=$(cat $PERFORMANCE_RESULTS)
        echo "$existing_results" | jq ". + {
            \"latency_test\": {
                \"message_count\": ${#latencies[@]},
                \"average_latency\": $avg_latency,
                \"min_latency\": $min_latency,
                \"max_latency\": $max_latency
            }
        }" > $PERFORMANCE_RESULTS
    else
        echo "{
            \"latency_test\": {
                \"message_count\": ${#latencies[@]},
                \"average_latency\": $avg_latency,
                \"min_latency\": $min_latency,
                \"max_latency\": $max_latency
            }
        }" > $PERFORMANCE_RESULTS
    fi
}

# 内存使用测试
test_memory_usage() {
    log_info "测试内存使用..."
    
    # 获取broker进程ID
    local broker_pid=$(pgrep -f "skytree" | head -1)
    if [ -z "$broker_pid" ]; then
        log_warning "未找到broker进程，跳过内存测试"
        return 0
    fi
    
    # 记录初始内存使用
    local initial_memory=$(ps -o rss= -p $broker_pid)
    
    # 创建大量连接
    local connection_count=1000
    local pids=()
    
    for i in $(seq 1 $connection_count); do
        mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "perf/memory" -i "memory_client_$i" &
        pids+=($!)
        
        if [ $((i % 100)) -eq 0 ]; then
            sleep 0.1
        fi
    done
    
    sleep 5
    
    # 记录峰值内存使用
    local peak_memory=$(ps -o rss= -p $broker_pid)
    
    # 清理连接
    for pid in "${pids[@]}"; do
        kill $pid 2>/dev/null || true
    done
    
    sleep 5
    
    # 记录清理后内存使用
    local final_memory=$(ps -o rss= -p $broker_pid)
    
    local memory_per_connection=$(echo "scale=2; ($peak_memory - $initial_memory) / $connection_count" | bc -l)
    
    log_info "内存使用测试结果:"
    log_info "  初始内存: ${initial_memory}KB"
    log_info "  峰值内存: ${peak_memory}KB"
    log_info "  最终内存: ${final_memory}KB"
    log_info "  连接数: $connection_count"
    log_info "  每连接内存: ${memory_per_connection}KB"
    
    # 更新结果文件
    if [ -f $PERFORMANCE_RESULTS ]; then
        local existing_results=$(cat $PERFORMANCE_RESULTS)
        echo "$existing_results" | jq ". + {
            \"memory_test\": {
                \"initial_memory_kb\": $initial_memory,
                \"peak_memory_kb\": $peak_memory,
                \"final_memory_kb\": $final_memory,
                \"connection_count\": $connection_count,
                \"memory_per_connection_kb\": $memory_per_connection
            }
        }" > $PERFORMANCE_RESULTS
    else
        echo "{
            \"memory_test\": {
                \"initial_memory_kb\": $initial_memory,
                \"peak_memory_kb\": $peak_memory,
                \"final_memory_kb\": $final_memory,
                \"connection_count\": $connection_count,
                \"memory_per_connection_kb\": $memory_per_connection
            }
        }" > $PERFORMANCE_RESULTS
    fi
}

# 生成测试报告
generate_report() {
    log_info "生成测试报告..."
    
    if [ ! -f $PERFORMANCE_RESULTS ]; then
        log_error "未找到测试结果文件"
        return 1
    fi
    
    local report_file="mqtt_performance_report_$(date +%Y%m%d_%H%M%S).json"
    
    # 添加测试元数据
    local test_metadata="{
        \"test_metadata\": {
            \"broker_host\": \"$BROKER_HOST\",
            \"broker_port\": $BROKER_PORT,
            \"test_duration\": $TEST_DURATION,
            \"concurrent_clients\": $CONCURRENT_CLIENTS,
            \"message_rate\": $MESSAGE_RATE,
            \"message_size\": $MESSAGE_SIZE,
            \"qos_level\": $QOS_LEVEL,
            \"test_time\": \"$(date -Iseconds)\"
        }
    }"
    
    if command -v jq &> /dev/null; then
        echo "$test_metadata" | jq ". + $(cat $PERFORMANCE_RESULTS)" > $report_file
    else
        echo "$test_metadata" > $report_file
        cat $PERFORMANCE_RESULTS >> $report_file
    fi
    
    log_success "测试报告已生成: $report_file"
    
    # 显示摘要
    echo "---"
    log_info "性能测试摘要:"
    if command -v jq &> /dev/null; then
        jq -r '
            "并发连接测试:",
            "  连接成功率: " + (.concurrent_connections.success_rate | tostring) + "%",
            "  连接建立速率: " + (.concurrent_connections.connection_rate | tostring) + " 连接/秒",
            "",
            "消息吞吐量测试:",
            "  吞吐量: " + (.throughput_test.throughput | tostring) + " 消息/秒",
            "  消息大小: " + (.throughput_test.message_size | tostring) + " 字节",
            "",
            "延迟测试:",
            "  平均延迟: " + (.latency_test.average_latency | tostring) + " ms",
            "  最大延迟: " + (.latency_test.max_latency | tostring) + " ms",
            "",
            "内存使用测试:",
            "  每连接内存: " + (.memory_test.memory_per_connection_kb | tostring) + " KB"
        ' $report_file
    fi
}

# 清理函数
cleanup() {
    log_info "清理测试环境..."
    
    # 清理临时文件
    rm -f /tmp/throughput_output /tmp/latency_output
    
    # 清理可能的残留进程
    pkill -f "mosquitto_sub" 2>/dev/null || true
    pkill -f "mosquitto_pub" 2>/dev/null || true
}

# 主函数
main() {
    log_info "开始MQTT性能测试"
    log_info "测试参数:"
    log_info "  - Broker地址: $BROKER_HOST:$BROKER_PORT"
    log_info "  - 测试时长: ${TEST_DURATION}秒"
    log_info "  - 并发客户端: $CONCURRENT_CLIENTS"
    log_info "  - 消息速率: $MESSAGE_RATE消息/秒"
    log_info "  - 消息大小: ${MESSAGE_SIZE}字节"
    log_info "  - QoS级别: $QOS_LEVEL"
    echo "---"
    
    # 设置清理陷阱
    trap cleanup EXIT
    
    # 检查环境
    check_dependencies
    check_broker
    get_system_info
    echo "---"
    
    # 运行性能测试
    test_concurrent_connections
    echo "---"
    
    test_message_throughput
    echo "---"
    
    test_latency
    echo "---"
    
    test_memory_usage
    echo "---"
    
    # 生成报告
    generate_report
    
    log_success "性能测试完成"
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
        --duration)
            TEST_DURATION="$2"
            shift 2
            ;;
        --clients)
            CONCURRENT_CLIENTS="$2"
            shift 2
            ;;
        --rate)
            MESSAGE_RATE="$2"
            shift 2
            ;;
        --size)
            MESSAGE_SIZE="$2"
            shift 2
            ;;
        --qos)
            QOS_LEVEL="$2"
            shift 2
            ;;
        --help)
            echo "用法: $0 [选项]"
            echo "选项:"
            echo "  --host HOST        Broker主机地址 (默认: localhost)"
            echo "  --port PORT        Broker端口 (默认: 1883)"
            echo "  --duration SECONDS 测试时长 (默认: 300)"
            echo "  --clients NUM      并发客户端数 (默认: 100)"
            echo "  --rate NUM         消息速率 (默认: 1000)"
            echo "  --size BYTES       消息大小 (默认: 1024)"
            echo "  --qos LEVEL        QoS级别 (默认: 0)"
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
