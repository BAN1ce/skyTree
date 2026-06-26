#!/bin/bash

# QuestDB 性能测试脚本
# 用于测试容器化 vs 实体机性能差异

set -e

# 配置参数
QUESTDB_HOST=${QUESTDB_HOST:-localhost}
QUESTDB_PORT=${QUESTDB_PORT:-8812}
QUESTDB_USER=${QUESTDB_USER:-admin}
QUESTDB_PASSWORD=${QUESTDB_PASSWORD:-quest}
QUESTDB_DATABASE=${QUESTDB_DATABASE:-qdb}

# 测试参数
BATCH_SIZE=${BATCH_SIZE:-1000}
TOTAL_MESSAGES=${TOTAL_MESSAGES:-100000}
CONCURRENT_CLIENTS=${CONCURRENT_CLIENTS:-10}
TEST_DURATION=${TEST_DURATION:-60}

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

# 检查QuestDB连接
check_questdb_connection() {
    log_info "检查QuestDB连接..."
    
    if ! command -v psql &> /dev/null; then
        log_error "psql 未安装，请先安装PostgreSQL客户端"
        exit 1
    fi
    
    if ! PGPASSWORD=$QUESTDB_PASSWORD psql -h $QUESTDB_HOST -p $QUESTDB_PORT -U $QUESTDB_USER -d $QUESTDB_DATABASE -c "SELECT 1" &> /dev/null; then
        log_error "无法连接到QuestDB服务器"
        exit 1
    fi
    
    log_success "QuestDB连接正常"
}

# 创建测试表
create_test_table() {
    log_info "创建测试表..."
    
    PGPASSWORD=$QUESTDB_PASSWORD psql -h $QUESTDB_HOST -p $QUESTDB_PORT -U $QUESTDB_USER -d $QUESTDB_DATABASE -c "
        CREATE TABLE IF NOT EXISTS benchmark_messages (
            topic SYMBOL,
            ts_ms TIMESTAMP,
            payload STRING,
            message_id SYMBOL
        ) TIMESTAMP(ts_ms) PARTITION BY DAY
    "
    
    log_success "测试表创建完成"
}

# 清理测试数据
cleanup_test_data() {
    log_info "清理测试数据..."
    
    PGPASSWORD=$QUESTDB_PASSWORD psql -h $QUESTDB_HOST -p $QUESTDB_PORT -U $QUESTDB_USER -d $QUESTDB_DATABASE -c "
        DROP TABLE IF EXISTS benchmark_messages
    "
    
    log_success "测试数据清理完成"
}

# 插入性能测试
benchmark_insert() {
    log_info "开始插入性能测试..."
    log_info "参数: 批量大小=$BATCH_SIZE, 总消息数=$TOTAL_MESSAGES, 并发客户端=$CONCURRENT_CLIENTS"
    
    local start_time=$(date +%s.%N)
    
    # 生成测试数据并插入
    for ((i=1; i<=$TOTAL_MESSAGES; i+=$BATCH_SIZE)); do
        local batch_end=$((i + BATCH_SIZE - 1))
        if [ $batch_end -gt $TOTAL_MESSAGES ]; then
            batch_end=$TOTAL_MESSAGES
        fi
        
        # 生成批量插入SQL
        local sql="INSERT INTO benchmark_messages VALUES "
        local values=""
        
        for ((j=i; j<=batch_end; j++)); do
            local topic="test/topic/$((j % 100))"
            local ts_ms=$(date -d "@$((1609459200 + j))" '+%Y-%m-%dT%H:%M:%S.%3N')
            local payload="test_payload_$j"
            local message_id="msg_$j"
            
            if [ -n "$values" ]; then
                values="$values, "
            fi
            values="$values('$topic', '$ts_ms', '$payload', '$message_id')"
        done
        
        sql="$sql$values"
        
        # 执行插入
        PGPASSWORD=$QUESTDB_PASSWORD psql -h $QUESTDB_HOST -p $QUESTDB_PORT -U $QUESTDB_USER -d $QUESTDB_DATABASE -c "$sql" &
        
        # 控制并发数
        if [ $((i % (BATCH_SIZE * CONCURRENT_CLIENTS))) -eq 0 ]; then
            wait
        fi
    done
    
    # 等待所有插入完成
    wait
    
    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    local messages_per_second=$(echo "scale=2; $TOTAL_MESSAGES / $duration" | bc)
    
    log_success "插入测试完成"
    log_info "总耗时: ${duration}秒"
    log_info "插入速度: ${messages_per_second} 消息/秒"
}

# 查询性能测试
benchmark_query() {
    log_info "开始查询性能测试..."
    
    local queries=(
        "SELECT COUNT(*) FROM benchmark_messages"
        "SELECT COUNT(*) FROM benchmark_messages WHERE topic = 'test/topic/1'"
        "SELECT * FROM benchmark_messages WHERE ts_ms >= '2021-01-01T00:00:00.000Z' ORDER BY ts_ms DESC LIMIT 100"
        "SELECT topic, COUNT(*) as count FROM benchmark_messages GROUP BY topic ORDER BY count DESC LIMIT 10"
        "SELECT to_start_of_hour(ts_ms) as hour, COUNT(*) as count FROM benchmark_messages GROUP BY hour ORDER BY hour LIMIT 24"
    )
    
    for query in "${queries[@]}"; do
        log_info "执行查询: $query"
        
        local start_time=$(date +%s.%N)
        local result=$(PGPASSWORD=$QUESTDB_PASSWORD psql -h $QUESTDB_HOST -p $QUESTDB_PORT -U $QUESTDB_USER -d $QUESTDB_DATABASE -t -c "$query")
        local end_time=$(date +%s.%N)
        local duration=$(echo "$end_time - $start_time" | bc)
        
        log_info "查询结果: $result"
        log_info "查询耗时: ${duration}秒"
        echo "---"
    done
}

# 并发测试
benchmark_concurrent() {
    log_info "开始并发测试..."
    
    local start_time=$(date +%s.%N)
    
    # 启动多个并发客户端
    for ((i=1; i<=CONCURRENT_CLIENTS; i++)); do
        (
            for ((j=1; j<=1000; j++)); do
                local topic="concurrent/topic/$i"
                local ts_ms=$(date -u '+%Y-%m-%dT%H:%M:%S.%3NZ')
                local payload="concurrent_payload_${i}_${j}"
                local message_id="concurrent_msg_${i}_${j}"
                
                PGPASSWORD=$QUESTDB_PASSWORD psql -h $QUESTDB_HOST -p $QUESTDB_PORT -U $QUESTDB_USER -d $QUESTDB_DATABASE -c "
                    INSERT INTO benchmark_messages VALUES ('$topic', '$ts_ms', '$payload', '$message_id')
                " &
                
                # 控制并发
                if [ $((j % 10)) -eq 0 ]; then
                    wait
                fi
            done
            wait
        ) &
    done
    
    # 等待所有并发客户端完成
    wait
    
    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    local total_concurrent_messages=$((CONCURRENT_CLIENTS * 1000))
    local messages_per_second=$(echo "scale=2; $total_concurrent_messages / $duration" | bc)
    
    log_success "并发测试完成"
    log_info "并发客户端数: $CONCURRENT_CLIENTS"
    log_info "总消息数: $total_concurrent_messages"
    log_info "总耗时: ${duration}秒"
    log_info "并发插入速度: ${messages_per_second} 消息/秒"
}

# 获取系统信息
get_system_info() {
    log_info "获取QuestDB系统信息..."
    
    echo "=== QuestDB版本信息 ==="
    PGPASSWORD=$QUESTDB_PASSWORD psql -h $QUESTDB_HOST -p $QUESTDB_PORT -U $QUESTDB_USER -d $QUESTDB_DATABASE -c "SELECT version()"
    
    echo "=== 数据库信息 ==="
    PGPASSWORD=$QUESTDB_PASSWORD psql -h $QUESTDB_HOST -p $QUESTDB_PORT -U $QUESTDB_USER -d $QUESTDB_DATABASE -c "SELECT name FROM tables()"
    
    echo "=== 表信息 ==="
    PGPASSWORD=$QUESTDB_PASSWORD psql -h $QUESTDB_HOST -p $QUESTDB_PORT -U $QUESTDB_USER -d $QUESTDB_DATABASE -c "SELECT name, type FROM tables() WHERE name = 'benchmark_messages'"
    
    echo "=== 系统设置 ==="
    PGPASSWORD=$QUESTDB_PASSWORD psql -h $QUESTDB_HOST -p $QUESTDB_PORT -U $QUESTDB_USER -d $QUESTDB_DATABASE -c "SELECT name, value FROM sys.settings"
}

# 容器性能测试
benchmark_container_performance() {
    log_info "开始容器性能测试..."
    
    # 测试容器资源使用情况
    if command -v docker &> /dev/null; then
        log_info "Docker容器资源使用情况:"
        docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}"
    fi
    
    # 测试容器网络延迟
    log_info "测试容器网络延迟..."
    local start_time=$(date +%s.%N)
    for i in {1..10}; do
        PGPASSWORD=$QUESTDB_PASSWORD psql -h $QUESTDB_HOST -p $QUESTDB_PORT -U $QUESTDB_USER -d $QUESTDB_DATABASE -c "SELECT 1" &> /dev/null
    done
    local end_time=$(date +%s.%N)
    local avg_latency=$(echo "scale=3; ($end_time - $start_time) / 10" | bc)
    log_info "平均网络延迟: ${avg_latency}秒"
}

# 主函数
main() {
    log_info "开始QuestDB性能测试"
    log_info "测试参数:"
    log_info "  - QuestDB地址: $QUESTDB_HOST:$QUESTDB_PORT"
    log_info "  - 批量大小: $BATCH_SIZE"
    log_info "  - 总消息数: $TOTAL_MESSAGES"
    log_info "  - 并发客户端: $CONCURRENT_CLIENTS"
    log_info "  - 测试时长: ${TEST_DURATION}秒"
    echo "---"
    
    # 检查连接
    check_questdb_connection
    
    # 获取系统信息
    get_system_info
    echo "---"
    
    # 创建测试表
    create_test_table
    
    # 执行性能测试
    benchmark_insert
    echo "---"
    
    benchmark_query
    echo "---"
    
    benchmark_concurrent
    echo "---"
    
    # 容器性能测试
    benchmark_container_performance
    echo "---"
    
    # 清理测试数据
    cleanup_test_data
    
    log_success "QuestDB性能测试完成"
}

# 处理命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --host)
            QUESTDB_HOST="$2"
            shift 2
            ;;
        --port)
            QUESTDB_PORT="$2"
            shift 2
            ;;
        --user)
            QUESTDB_USER="$2"
            shift 2
            ;;
        --password)
            QUESTDB_PASSWORD="$2"
            shift 2
            ;;
        --batch-size)
            BATCH_SIZE="$2"
            shift 2
            ;;
        --total-messages)
            TOTAL_MESSAGES="$2"
            shift 2
            ;;
        --concurrent-clients)
            CONCURRENT_CLIENTS="$2"
            shift 2
            ;;
        --cleanup-only)
            check_questdb_connection
            cleanup_test_data
            exit 0
            ;;
        --help)
            echo "QuestDB性能测试脚本"
            echo ""
            echo "用法: $0 [选项]"
            echo ""
            echo "选项:"
            echo "  --host HOST                QuestDB主机地址 (默认: localhost)"
            echo "  --port PORT                QuestDB端口 (默认: 8812)"
            echo "  --user USER                QuestDB用户名 (默认: admin)"
            echo "  --password PASSWORD        QuestDB密码 (默认: quest)"
            echo "  --batch-size SIZE          批量大小 (默认: 1000)"
            echo "  --total-messages COUNT     总消息数 (默认: 100000)"
            echo "  --concurrent-clients NUM   并发客户端数 (默认: 10)"
            echo "  --cleanup-only             仅清理测试数据"
            echo "  --help                     显示此帮助信息"
            exit 0
            ;;
        *)
            log_error "未知参数: $1"
            echo "使用 --help 查看帮助信息"
            exit 1
            ;;
    esac
done

# 运行主函数
main
