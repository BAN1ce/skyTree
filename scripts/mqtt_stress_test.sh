#!/bin/bash

# MQTT压力测试脚本
# 用于测试SkyTree MQTT代理在极限负载下的表现

set -e

# 配置参数
BROKER_HOST=${BROKER_HOST:-localhost}
BROKER_PORT=${BROKER_PORT:-1883}
STRESS_DURATION=${STRESS_DURATION:-1800}  # 30分钟
MAX_CONNECTIONS=${MAX_CONNECTIONS:-10000}
MAX_MESSAGE_RATE=${MAX_MESSAGE_RATE:-50000}
MESSAGE_SIZE=${MESSAGE_SIZE:-1024}

# 压力测试结果
STRESS_RESULTS="/tmp/mqtt_stress_results.json"

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

# 系统监控
monitor_system() {
    local duration=$1
    local log_file="/tmp/system_monitor.log"
    
    log_info "开始系统监控 (${duration}秒)..."
    
    {
        echo "timestamp,cpu_usage,memory_usage,disk_io,network_io,connections"
        
        for i in $(seq 1 $duration); do
            local timestamp=$(date +%s)
            local cpu_usage=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1)
            local memory_usage=$(free | grep Mem | awk '{printf "%.2f", $3/$2 * 100.0}')
            local disk_io=$(iostat -x 1 1 | tail -n +4 | awk '{sum+=$10} END {print sum}')
            local network_io=$(cat /proc/net/dev | grep eth0 | awk '{print $2+$10}')
            local connections=$(netstat -an | grep :$BROKER_PORT | wc -l)
            
            echo "$timestamp,$cpu_usage,$memory_usage,$disk_io,$network_io,$connections"
            sleep 1
        done
    } > $log_file &
    
    echo $! > /tmp/monitor_pid
}

# 停止系统监控
stop_monitor() {
    if [ -f /tmp/monitor_pid ]; then
        local monitor_pid=$(cat /tmp/monitor_pid)
        kill $monitor_pid 2>/dev/null || true
        rm -f /tmp/monitor_pid
    fi
}

# 连接压力测试
test_connection_stress() {
    log_info "开始连接压力测试..."
    
    local connections=0
    local pids=()
    local start_time=$(date +%s)
    local max_connections=$MAX_CONNECTIONS
    
    # 创建大量连接
    for i in $(seq 1 $max_connections); do
        mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "stress/connections" -i "stress_client_$i" &
        pids+=($!)
        connections=$((connections + 1))
        
        # 每100个连接暂停一下，避免系统过载
        if [ $((i % 100)) -eq 0 ]; then
            sleep 0.1
        fi
        
        # 每1000个连接检查一次系统状态
        if [ $((i % 1000)) -eq 0 ]; then
            local current_time=$(date +%s)
            local elapsed=$((current_time - start_time))
            log_info "已创建 $i 个连接，耗时 ${elapsed}秒"
        fi
    done
    
    local end_time=$(date +%s)
    local connection_time=$((end_time - start_time))
    
    # 等待连接稳定
    sleep 10
    
    # 检查活跃连接数
    local active_connections=0
    for pid in "${pids[@]}"; do
        if kill -0 $pid 2>/dev/null; then
            active_connections=$((active_connections + 1))
        fi
    done
    
    # 保持连接一段时间
    log_info "保持 $active_connections 个连接 ${STRESS_DURATION}秒..."
    sleep $STRESS_DURATION
    
    # 清理连接
    log_info "清理连接..."
    for pid in "${pids[@]}"; do
        kill $pid 2>/dev/null || true
    done
    
    local connection_rate=$((connections / connection_time))
    local success_rate=$((active_connections * 100 / connections))
    
    log_info "连接压力测试结果:"
    log_info "  尝试连接数: $connections"
    log_info "  成功连接数: $active_connections"
    log_info "  连接成功率: ${success_rate}%"
    log_info "  连接建立时间: ${connection_time}秒"
    log_info "  连接建立速率: ${connection_rate}连接/秒"
    log_info "  稳定运行时间: ${STRESS_DURATION}秒"
    
    # 保存结果
    echo "{
        \"test\": \"connection_stress\",
        \"attempted_connections\": $connections,
        \"successful_connections\": $active_connections,
        \"success_rate\": $success_rate,
        \"connection_time\": $connection_time,
        \"connection_rate\": $connection_rate,
        \"stable_duration\": $STRESS_DURATION
    }" > $STRESS_RESULTS
}

# 消息压力测试
test_message_stress() {
    log_info "开始消息压力测试..."
    
    local topic="stress/messages"
    local message_count=0
    local start_time=$(date +%s.%N)
    
    # 启动多个订阅者
    local subscriber_count=100
    local sub_pids=()
    
    for i in $(seq 1 $subscriber_count); do
        timeout $STRESS_DURATION mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "$topic" -i "stress_sub_$i" -C 0 > /tmp/stress_sub_$i.log &
        sub_pids+=($!)
    done
    
    sleep 5
    
    # 启动多个发布者
    local publisher_count=50
    local pub_pids=()
    
    for i in $(seq 1 $publisher_count); do
        timeout $STRESS_DURATION bash -c "
            local message_num=0
            while true; do
                mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t '$topic' -m '$(head -c $MESSAGE_SIZE /dev/zero | tr '\0' 'A')' -q 0
                message_num=\$((message_num + 1))
                if [ \$((message_num % 1000)) -eq 0 ]; then
                    echo \"Publisher $i: \$message_num messages\"
                fi
                sleep 0.001
            done
        " &
        pub_pids+=($!)
    done
    
    # 等待测试完成
    wait ${pub_pids[0]} 2>/dev/null || true
    local end_time=$(date +%s.%N)
    
    # 停止所有进程
    for pid in "${sub_pids[@]}"; do
        kill $pid 2>/dev/null || true
    done
    
    for pid in "${pub_pids[@]}"; do
        kill $pid 2>/dev/null || true
    done
    
    # 统计结果
    local test_duration=$(echo "$end_time - $start_time" | bc -l)
    local total_messages=0
    
    for i in $(seq 1 $subscriber_count); do
        if [ -f "/tmp/stress_sub_$i.log" ]; then
            local sub_messages=$(wc -l < "/tmp/stress_sub_$i.log")
            total_messages=$((total_messages + sub_messages))
        fi
    done
    
    local throughput=$(echo "scale=2; $total_messages / $test_duration" | bc -l)
    local messages_per_publisher=$(echo "scale=2; $total_messages / $publisher_count" | bc -l)
    
    log_info "消息压力测试结果:"
    log_info "  测试时长: ${test_duration}秒"
    log_info "  发布者数量: $publisher_count"
    log_info "  订阅者数量: $subscriber_count"
    log_info "  总消息数: $total_messages"
    log_info "  消息大小: ${MESSAGE_SIZE}字节"
    log_info "  吞吐量: ${throughput}消息/秒"
    log_info "  每发布者消息数: ${messages_per_publisher}"
    
    # 更新结果文件
    if [ -f $STRESS_RESULTS ]; then
        local existing_results=$(cat $STRESS_RESULTS)
        echo "$existing_results" | jq ". + {
            \"message_stress_test\": {
                \"duration\": $test_duration,
                \"publisher_count\": $publisher_count,
                \"subscriber_count\": $subscriber_count,
                \"total_messages\": $total_messages,
                \"message_size\": $MESSAGE_SIZE,
                \"throughput\": $throughput,
                \"messages_per_publisher\": $messages_per_publisher
            }
        }" > $STRESS_RESULTS
    else
        echo "{
            \"message_stress_test\": {
                \"duration\": $test_duration,
                \"publisher_count\": $publisher_count,
                \"subscriber_count\": $subscriber_count,
                \"total_messages\": $total_messages,
                \"message_size\": $MESSAGE_SIZE,
                \"throughput\": $throughput,
                \"messages_per_publisher\": $messages_per_publisher
            }
        }" > $STRESS_RESULTS
    fi
    
    # 清理临时文件
    rm -f /tmp/stress_sub_*.log
}

# 内存压力测试
test_memory_stress() {
    log_info "开始内存压力测试..."
    
    # 获取broker进程ID
    local broker_pid=$(pgrep -f "skytree" | head -1)
    if [ -z "$broker_pid" ]; then
        log_warning "未找到broker进程，跳过内存压力测试"
        return 0
    fi
    
    # 记录初始内存使用
    local initial_memory=$(ps -o rss= -p $broker_pid)
    local initial_time=$(date +%s)
    
    # 创建大量连接和消息
    local connection_count=5000
    local pids=()
    
    for i in $(seq 1 $connection_count); do
        mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "stress/memory" -i "memory_stress_$i" &
        pids+=($!)
        
        if [ $((i % 100)) -eq 0 ]; then
            sleep 0.1
        fi
    done
    
    # 发送大量消息
    local message_count=10000
    for i in $(seq 1 $message_count); do
        mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "stress/memory" -m "$(head -c 1024 /dev/zero | tr '\0' 'B')" -q 0
        if [ $((i % 1000)) -eq 0 ]; then
            sleep 0.1
        fi
    done
    
    # 记录峰值内存使用
    local peak_memory=$(ps -o rss= -p $broker_pid)
    local peak_time=$(date +%s)
    
    # 保持压力一段时间
    sleep $STRESS_DURATION
    
    # 记录稳定期内存使用
    local stable_memory=$(ps -o rss= -p $broker_pid)
    local stable_time=$(date +%s)
    
    # 清理连接
    for pid in "${pids[@]}"; do
        kill $pid 2>/dev/null || true
    done
    
    # 等待内存回收
    sleep 30
    
    # 记录清理后内存使用
    local final_memory=$(ps -o rss= -p $broker_pid)
    local final_time=$(date +%s)
    
    local memory_per_connection=$(echo "scale=2; ($peak_memory - $initial_memory) / $connection_count" | bc -l)
    local memory_growth=$(echo "scale=2; $peak_memory - $initial_memory" | bc -l)
    local memory_recovery=$(echo "scale=2; $peak_memory - $final_memory" | bc -l)
    
    log_info "内存压力测试结果:"
    log_info "  初始内存: ${initial_memory}KB"
    log_info "  峰值内存: ${peak_memory}KB"
    log_info "  稳定期内存: ${stable_memory}KB"
    log_info "  最终内存: ${final_memory}KB"
    log_info "  连接数: $connection_count"
    log_info "  消息数: $message_count"
    log_info "  内存增长: ${memory_growth}KB"
    log_info "  每连接内存: ${memory_per_connection}KB"
    log_info "  内存回收: ${memory_recovery}KB"
    
    # 更新结果文件
    if [ -f $STRESS_RESULTS ]; then
        local existing_results=$(cat $STRESS_RESULTS)
        echo "$existing_results" | jq ". + {
            \"memory_stress_test\": {
                \"initial_memory_kb\": $initial_memory,
                \"peak_memory_kb\": $peak_memory,
                \"stable_memory_kb\": $stable_memory,
                \"final_memory_kb\": $final_memory,
                \"connection_count\": $connection_count,
                \"message_count\": $message_count,
                \"memory_growth_kb\": $memory_growth,
                \"memory_per_connection_kb\": $memory_per_connection,
                \"memory_recovery_kb\": $memory_recovery
            }
        }" > $STRESS_RESULTS
    else
        echo "{
            \"memory_stress_test\": {
                \"initial_memory_kb\": $initial_memory,
                \"peak_memory_kb\": $peak_memory,
                \"stable_memory_kb\": $stable_memory,
                \"final_memory_kb\": $final_memory,
                \"connection_count\": $connection_count,
                \"message_count\": $message_count,
                \"memory_growth_kb\": $memory_growth,
                \"memory_per_connection_kb\": $memory_per_connection,
                \"memory_recovery_kb\": $memory_recovery
            }
        }" > $STRESS_RESULTS
    fi
}

# 网络压力测试
test_network_stress() {
    log_info "开始网络压力测试..."
    
    local topic="stress/network"
    local message_size=65536  # 64KB大消息
    local start_time=$(date +%s.%N)
    
    # 启动订阅者
    timeout $STRESS_DURATION mosquitto_sub -h $BROKER_HOST -p $BROKER_PORT -t "$topic" -C 0 > /tmp/network_output &
    SUB_PID=$!
    
    sleep 2
    
    # 发送大消息
    local message_count=0
    while [ $message_count -lt 1000 ]; do
        mosquitto_pub -h $BROKER_HOST -p $BROKER_PORT -t "$topic" -m "$(head -c $message_size /dev/zero | tr '\0' 'C')" -q 0
        message_count=$((message_count + 1))
        
        if [ $((message_count % 100)) -eq 0 ]; then
            log_info "已发送 $message_count 个大消息"
        fi
    done
    
    local end_time=$(date +%s.%N)
    local test_duration=$(echo "$end_time - $start_time" | bc -l)
    
    # 停止订阅者
    kill $SUB_PID 2>/dev/null || true
    sleep 2
    
    # 统计结果
    local received_messages=$(wc -l < /tmp/network_output)
    local total_bytes=$(echo "$received_messages * $message_size" | bc -l)
    local bandwidth=$(echo "scale=2; $total_bytes / $test_duration / 1024 / 1024" | bc -l)
    
    log_info "网络压力测试结果:"
    log_info "  测试时长: ${test_duration}秒"
    log_info "  消息大小: ${message_size}字节"
    log_info "  发送消息数: $message_count"
    log_info "  接收消息数: $received_messages"
    log_info "  总数据量: ${total_bytes}字节"
    log_info "  带宽: ${bandwidth}MB/s"
    
    # 更新结果文件
    if [ -f $STRESS_RESULTS ]; then
        local existing_results=$(cat $STRESS_RESULTS)
        echo "$existing_results" | jq ". + {
            \"network_stress_test\": {
                \"duration\": $test_duration,
                \"message_size\": $message_size,
                \"sent_messages\": $message_count,
                \"received_messages\": $received_messages,
                \"total_bytes\": $total_bytes,
                \"bandwidth_mbps\": $bandwidth
            }
        }" > $STRESS_RESULTS
    else
        echo "{
            \"network_stress_test\": {
                \"duration\": $test_duration,
                \"message_size\": $message_size,
                \"sent_messages\": $message_count,
                \"received_messages\": $received_messages,
                \"total_bytes\": $total_bytes,
                \"bandwidth_mbps\": $bandwidth
            }
        }" > $STRESS_RESULTS
    fi
}

# 生成压力测试报告
generate_stress_report() {
    log_info "生成压力测试报告..."
    
    if [ ! -f $STRESS_RESULTS ]; then
        log_error "未找到压力测试结果文件"
        return 1
    fi
    
    local report_file="mqtt_stress_report_$(date +%Y%m%d_%H%M%S).json"
    
    # 添加测试元数据
    local test_metadata="{
        \"test_metadata\": {
            \"broker_host\": \"$BROKER_HOST\",
            \"broker_port\": $BROKER_PORT,
            \"stress_duration\": $STRESS_DURATION,
            \"max_connections\": $MAX_CONNECTIONS,
            \"max_message_rate\": $MAX_MESSAGE_RATE,
            \"message_size\": $MESSAGE_SIZE,
            \"test_time\": \"$(date -Iseconds)\"
        }
    }"
    
    if command -v jq &> /dev/null; then
        echo "$test_metadata" | jq ". + $(cat $STRESS_RESULTS)" > $report_file
    else
        echo "$test_metadata" > $report_file
        cat $STRESS_RESULTS >> $report_file
    fi
    
    log_success "压力测试报告已生成: $report_file"
    
    # 显示摘要
    echo "---"
    log_info "压力测试摘要:"
    if command -v jq &> /dev/null; then
        jq -r '
            "连接压力测试:",
            "  最大连接数: " + (.connection_stress.successful_connections | tostring),
            "  连接成功率: " + (.connection_stress.success_rate | tostring) + "%",
            "",
            "消息压力测试:",
            "  吞吐量: " + (.message_stress_test.throughput | tostring) + " 消息/秒",
            "  总消息数: " + (.message_stress_test.total_messages | tostring),
            "",
            "内存压力测试:",
            "  峰值内存: " + (.memory_stress_test.peak_memory_kb | tostring) + " KB",
            "  每连接内存: " + (.memory_stress_test.memory_per_connection_kb | tostring) + " KB",
            "",
            "网络压力测试:",
            "  带宽: " + (.network_stress_test.bandwidth_mbps | tostring) + " MB/s"
        ' $report_file
    fi
}

# 清理函数
cleanup() {
    log_info "清理测试环境..."
    
    # 停止系统监控
    stop_monitor
    
    # 清理临时文件
    rm -f /tmp/network_output /tmp/stress_sub_*.log /tmp/monitor_pid
    
    # 清理可能的残留进程
    pkill -f "mosquitto_sub" 2>/dev/null || true
    pkill -f "mosquitto_pub" 2>/dev/null || true
}

# 主函数
main() {
    log_info "开始MQTT压力测试"
    log_info "测试参数:"
    log_info "  - Broker地址: $BROKER_HOST:$BROKER_PORT"
    log_info "  - 压力测试时长: ${STRESS_DURATION}秒"
    log_info "  - 最大连接数: $MAX_CONNECTIONS"
    log_info "  - 最大消息速率: $MAX_MESSAGE_RATE消息/秒"
    log_info "  - 消息大小: ${MESSAGE_SIZE}字节"
    echo "---"
    
    # 设置清理陷阱
    trap cleanup EXIT
    
    # 开始系统监控
    monitor_system $((STRESS_DURATION + 600))  # 监控时间比测试时间长10分钟
    
    # 运行压力测试
    test_connection_stress
    echo "---"
    
    test_message_stress
    echo "---"
    
    test_memory_stress
    echo "---"
    
    test_network_stress
    echo "---"
    
    # 生成报告
    generate_stress_report
    
    log_success "压力测试完成"
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
            STRESS_DURATION="$2"
            shift 2
            ;;
        --max-connections)
            MAX_CONNECTIONS="$2"
            shift 2
            ;;
        --max-rate)
            MAX_MESSAGE_RATE="$2"
            shift 2
            ;;
        --size)
            MESSAGE_SIZE="$2"
            shift 2
            ;;
        --help)
            echo "用法: $0 [选项]"
            echo "选项:"
            echo "  --host HOST           Broker主机地址 (默认: localhost)"
            echo "  --port PORT           Broker端口 (默认: 1883)"
            echo "  --duration SECONDS    压力测试时长 (默认: 1800)"
            echo "  --max-connections NUM 最大连接数 (默认: 10000)"
            echo "  --max-rate NUM        最大消息速率 (默认: 50000)"
            echo "  --size BYTES          消息大小 (默认: 1024)"
            echo "  --help                显示帮助信息"
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
