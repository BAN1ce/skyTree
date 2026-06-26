#!/bin/bash

# 分布式MQTT压测脚本
# 支持100万客户端和200万topic的智能分发压测

set -e

# 配置参数
TOTAL_CLIENTS=${TOTAL_CLIENTS:-1000000}      # 总客户端数
TOTAL_TOPICS=${TOTAL_TOPICS:-2000000}        # 总topic数
BROKER_CLUSTER=${BROKER_CLUSTER:-"localhost:1883,localhost:1884,localhost:1885"}
TEST_DURATION=${TEST_DURATION:-3600}         # 测试时长(秒)
CLIENTS_PER_MACHINE=${CLIENTS_PER_MACHINE:-50000}  # 每台机器客户端数
TOPICS_PER_MACHINE=${TOPICS_PER_MACHINE:-100000}   # 每台机器topic数

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

# 解析broker集群地址
parse_broker_cluster() {
    IFS=',' read -ra BROKERS <<< "$BROKER_CLUSTER"
    BROKER_COUNT=${#BROKERS[@]}
    log_info "检测到 $BROKER_COUNT 个broker节点: ${BROKERS[*]}"
}

# 获取broker节点健康状态
check_broker_health() {
    log_info "检查broker集群健康状态..."
    
    local healthy_brokers=()
    for broker in "${BROKERS[@]}"; do
        local host=$(echo $broker | cut -d: -f1)
        local port=$(echo $broker | cut -d: -f2)
        
        if timeout 5 bash -c "echo > /dev/tcp/$host/$port" 2>/dev/null; then
            healthy_brokers+=("$broker")
            log_success "Broker $broker 健康"
        else
            log_warning "Broker $broker 不可达"
        fi
    done
    
    if [ ${#healthy_brokers[@]} -eq 0 ]; then
        log_error "没有健康的broker节点"
        exit 1
    fi
    
    BROKERS=("${healthy_brokers[@]}")
    BROKER_COUNT=${#BROKERS[@]}
    log_info "可用broker节点: $BROKER_COUNT"
}

# 计算连接分发策略
calculate_distribution() {
    log_info "计算连接分发策略..."
    
    # 计算需要的机器数量
    MACHINES_NEEDED=$(( (TOTAL_CLIENTS + CLIENTS_PER_MACHINE - 1) / CLIENTS_PER_MACHINE ))
    
    # 计算每个broker的负载
    CLIENTS_PER_BROKER=$(( TOTAL_CLIENTS / BROKER_COUNT ))
    TOPICS_PER_BROKER=$(( TOTAL_TOPICS / BROKER_COUNT ))
    
    log_info "分发策略:"
    log_info "  总客户端数: $TOTAL_CLIENTS"
    log_info "  总topic数: $TOTAL_TOPICS"
    log_info "  需要机器数: $MACHINES_NEEDED"
    log_info "  每台机器客户端: $CLIENTS_PER_MACHINE"
    log_info "  每台机器topic: $TOPICS_PER_MACHINE"
    log_info "  每broker客户端: $CLIENTS_PER_BROKER"
    log_info "  每broker topic: $TOPICS_PER_BROKER"
}

# 生成压测配置
generate_stress_config() {
    local machine_id=$1
    local broker_index=$((machine_id % BROKER_COUNT))
    local broker=${BROKERS[$broker_index]}
    local host=$(echo $broker | cut -d: -f1)
    local port=$(echo $broker | cut -d: -f2)
    
    # 计算该机器的客户端和topic范围
    local client_start=$((machine_id * CLIENTS_PER_MACHINE))
    local client_end=$((client_start + CLIENTS_PER_MACHINE - 1))
    local topic_start=$((machine_id * TOPICS_PER_MACHINE))
    local topic_end=$((topic_start + TOPICS_PER_MACHINE - 1))
    
    cat > "/tmp/stress_config_${machine_id}.json" << EOF
{
    "machine_id": $machine_id,
    "broker": {
        "host": "$host",
        "port": $port,
        "index": $broker_index
    },
    "clients": {
        "start": $client_start,
        "end": $client_end,
        "count": $CLIENTS_PER_MACHINE
    },
    "topics": {
        "start": $topic_start,
        "end": $topic_end,
        "count": $TOPICS_PER_MACHINE
    },
    "test_duration": $TEST_DURATION
}
EOF
    
    log_info "机器 $machine_id 配置: Broker=$broker, 客户端=$client_start-$client_end, Topic=$topic_start-$topic_end"
}

# 启动单机压测
start_single_machine_stress() {
    local machine_id=$1
    local config_file="/tmp/stress_config_${machine_id}.json"
    
    if [ ! -f "$config_file" ]; then
        log_error "配置文件不存在: $config_file"
        return 1
    fi
    
    local broker_host=$(jq -r '.broker.host' "$config_file")
    local broker_port=$(jq -r '.broker.port' "$config_file")
    local client_count=$(jq -r '.clients.count' "$config_file")
    local topic_count=$(jq -r '.topics.count' "$config_file")
    local client_start=$(jq -r '.clients.start' "$config_file")
    local topic_start=$(jq -r '.topics.start' "$config_file")
    
    log_info "启动机器 $machine_id 压测..."
    log_info "  Broker: $broker_host:$broker_port"
    log_info "  客户端数: $client_count"
    log_info "  Topic数: $topic_count"
    
    # 启动emqtt-bench连接测试
    emqtt_bench conn \
        -h "$broker_host" \
        -p "$broker_port" \
        -c "$client_count" \
        --ifaddr 0.0.0.0 \
        --conn-interval 1ms \
        --keepalive 300 \
        --clientid-prefix "stress_${machine_id}_" \
        --log-level error \
        > "/tmp/conn_results_${machine_id}.log" 2>&1 &
    
    local conn_pid=$!
    
    # 等待连接建立
    sleep 30
    
    # 启动发布测试
    emqtt_bench pub \
        -h "$broker_host" \
        -p "$broker_port" \
        -c 1000 \
        -t "stress/test/{{.ClientID}}" \
        -m "stress test message from machine $machine_id" \
        --pub-interval 10ms \
        --qos 0 \
        --clientid-prefix "pub_${machine_id}_" \
        --log-level error \
        > "/tmp/pub_results_${machine_id}.log" 2>&1 &
    
    local pub_pid=$!
    
    # 启动订阅测试
    emqtt_bench sub \
        -h "$broker_host" \
        -p "$broker_port" \
        -c "$client_count" \
        -t "stress/test/{{.ClientID}}" \
        --qos 0 \
        --clientid-prefix "sub_${machine_id}_" \
        --log-level error \
        > "/tmp/sub_results_${machine_id}.log" 2>&1 &
    
    local sub_pid=$!
    
    # 记录进程ID
    echo "$conn_pid $pub_pid $sub_pid" > "/tmp/stress_pids_${machine_id}.txt"
    
    log_success "机器 $machine_id 压测已启动 (PID: $conn_pid, $pub_pid, $sub_pid)"
}

# 监控压测进度
monitor_stress_progress() {
    log_info "开始监控压测进度..."
    
    local start_time=$(date +%s)
    local end_time=$((start_time + TEST_DURATION))
    
    while [ $(date +%s) -lt $end_time ]; do
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        local remaining=$((end_time - current_time))
        
        log_info "压测进度: 已运行 ${elapsed}s, 剩余 ${remaining}s"
        
        # 检查各机器状态
        for i in $(seq 0 $((MACHINES_NEEDED - 1))); do
            if [ -f "/tmp/stress_pids_${i}.txt" ]; then
                local pids=$(cat "/tmp/stress_pids_${i}.txt")
                local running_count=0
                
                for pid in $pids; do
                    if kill -0 "$pid" 2>/dev/null; then
                        running_count=$((running_count + 1))
                    fi
                done
                
                if [ $running_count -eq 3 ]; then
                    echo -n "✓"
                else
                    echo -n "✗"
                fi
            else
                echo -n "?"
            fi
        done
        echo ""
        
        sleep 30
    done
}

# 收集压测结果
collect_results() {
    log_info "收集压测结果..."
    
    local total_connections=0
    local total_messages_sent=0
    local total_messages_received=0
    local results_file="distributed_stress_results_$(date +%Y%m%d_%H%M%S).json"
    
    echo "{" > "$results_file"
    echo "  \"test_metadata\": {" >> "$results_file"
    echo "    \"total_clients\": $TOTAL_CLIENTS," >> "$results_file"
    echo "    \"total_topics\": $TOTAL_TOPICS," >> "$results_file"
    echo "    \"broker_count\": $BROKER_COUNT," >> "$results_file"
    echo "    \"machines_used\": $MACHINES_NEEDED," >> "$results_file"
    echo "    \"test_duration\": $TEST_DURATION," >> "$results_file"
    echo "    \"test_time\": \"$(date -Iseconds)\"" >> "$results_file"
    echo "  }," >> "$results_file"
    echo "  \"broker_distribution\": [" >> "$results_file"
    
    for i in "${!BROKERS[@]}"; do
        local broker=${BROKERS[$i]}
        echo "    {" >> "$results_file"
        echo "      \"index\": $i," >> "$results_file"
        echo "      \"address\": \"$broker\"," >> "$results_file"
        echo "      \"clients_assigned\": $CLIENTS_PER_BROKER," >> "$results_file"
        echo "      \"topics_assigned\": $TOPICS_PER_BROKER" >> "$results_file"
        if [ $i -lt $((${#BROKERS[@]} - 1)) ]; then
            echo "    }," >> "$results_file"
        else
            echo "    }" >> "$results_file"
        fi
    done
    
    echo "  ]," >> "$results_file"
    echo "  \"machine_results\": [" >> "$results_file"
    
    for i in $(seq 0 $((MACHINES_NEEDED - 1))); do
        if [ -f "/tmp/stress_config_${i}.json" ]; then
            local config=$(cat "/tmp/stress_config_${i}.json")
            echo "    $config" >> "$results_file"
            if [ $i -lt $((MACHINES_NEEDED - 1)) ]; then
                echo "," >> "$results_file"
            fi
        fi
    done
    
    echo "  ]" >> "$results_file"
    echo "}" >> "$results_file"
    
    log_success "压测结果已保存到: $results_file"
}

# 清理资源
cleanup() {
    log_info "清理压测资源..."
    
    # 停止所有压测进程
    for i in $(seq 0 $((MACHINES_NEEDED - 1))); do
        if [ -f "/tmp/stress_pids_${i}.txt" ]; then
            local pids=$(cat "/tmp/stress_pids_${i}.txt")
            for pid in $pids; do
                kill "$pid" 2>/dev/null || true
            done
            rm -f "/tmp/stress_pids_${i}.txt"
        fi
    done
    
    # 清理临时文件
    rm -f /tmp/stress_config_*.json
    rm -f /tmp/*_results_*.log
}

# 主函数
main() {
    log_info "开始分布式MQTT压测"
    log_info "目标: $TOTAL_CLIENTS 客户端, $TOTAL_TOPICS topic"
    echo "---"
    
    # 设置清理陷阱
    trap cleanup EXIT
    
    # 检查依赖
    if ! command -v emqtt_bench &> /dev/null; then
        log_error "emqtt_bench 未安装，请先安装: go install github.com/emqx/emqtt-bench@latest"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        log_error "jq 未安装，请先安装: apt-get install jq"
        exit 1
    fi
    
    # 解析和检查broker集群
    parse_broker_cluster
    check_broker_health
    
    # 计算分发策略
    calculate_distribution
    echo "---"
    
    # 生成配置并启动压测
    for i in $(seq 0 $((MACHINES_NEEDED - 1))); do
        generate_stress_config $i
        start_single_machine_stress $i
        sleep 5  # 避免同时启动造成冲击
    done
    
    echo "---"
    
    # 监控进度
    monitor_stress_progress
    
    echo "---"
    
    # 收集结果
    collect_results
    
    log_success "分布式压测完成"
}

# 处理命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --clients)
            TOTAL_CLIENTS="$2"
            shift 2
            ;;
        --topics)
            TOTAL_TOPICS="$2"
            shift 2
            ;;
        --brokers)
            BROKER_CLUSTER="$2"
            shift 2
            ;;
        --duration)
            TEST_DURATION="$2"
            shift 2
            ;;
        --clients-per-machine)
            CLIENTS_PER_MACHINE="$2"
            shift 2
            ;;
        --topics-per-machine)
            TOPICS_PER_MACHINE="$2"
            shift 2
            ;;
        --help)
            echo "用法: $0 [选项]"
            echo "选项:"
            echo "  --clients NUM              总客户端数 (默认: 1000000)"
            echo "  --topics NUM               总topic数 (默认: 2000000)"
            echo "  --brokers HOSTS            Broker集群地址 (默认: localhost:1883,localhost:1884,localhost:1885)"
            echo "  --duration SECONDS         测试时长 (默认: 3600)"
            echo "  --clients-per-machine NUM  每台机器客户端数 (默认: 50000)"
            echo "  --topics-per-machine NUM   每台机器topic数 (默认: 100000)"
            echo "  --help                     显示帮助信息"
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
