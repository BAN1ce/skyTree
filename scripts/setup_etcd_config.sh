#!/bin/bash

# SkyTree ETCD配置设置脚本
# 用于在ETCD中设置SkyTree的配置

set -e

ETCD_ENDPOINTS=${ETCD_ENDPOINTS:-"localhost:2379"}
CONFIG_PREFIX=${CONFIG_PREFIX:-"/skyTree/config"}

echo "🔧 正在设置 SkyTree ETCD 配置..."
echo "ETCD 地址: $ETCD_ENDPOINTS"
echo "配置前缀: $CONFIG_PREFIX"

# 检查etcdctl是否可用
if ! command -v etcdctl &> /dev/null; then
    echo "❌ etcdctl 未找到，请先安装 etcd"
    exit 1
fi

# 检查ETCD连接
if ! etcdctl --endpoints=$ETCD_ENDPOINTS endpoint health &> /dev/null; then
    echo "❌ 无法连接到 ETCD，请确保 ETCD 服务正在运行"
    echo "提示: 可以运行 'etcd' 命令启动本地 ETCD 服务"
    exit 1
fi

echo "✅ ETCD 连接正常"

# 设置服务器配置
echo "📝 设置服务器配置..."
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/server/port "9526"

# 设置MQTT代理配置
echo "📝 设置 MQTT 代理配置..."
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/listeners/0 "tcp://localhost:1883"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/keep_alive_seconds "180"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/read_batch_size "200"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/persist_qos0 "true"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/no_sub_topic_behavior "0"

# 设置连接确认属性
echo "📝 设置连接确认属性..."
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/connack/receive_maximum "65535"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/connack/max_qos "2"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/connack/retain_available "1"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/connack/maximum_packet_size "1048576"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/connack/wildcard_subscription_available "true"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/connack/shared_subscription_available "true"

# 设置消息重试配置
echo "📝 设置消息重试配置..."
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/message_retry/max_retry_count "3"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/message_retry/interval "30s"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/message_retry/max_timeout "300s"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/broker/message_retry/scheduler_interval "1s"

# 设置存储配置
echo "📝 设置存储配置..."
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/storage/driver "redis"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/storage/message_expire_days "1"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/storage/redis/address "localhost:6379"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/storage/redis/password ""
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/storage/redis/db "0"

# 设置集群配置
echo "📝 设置集群配置..."
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/cluster/enable "false"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/cluster/join "false"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/cluster/data_dir "./data/cluster"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/cluster/local_node_address "localhost:63001"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/cluster/local_node_id "1"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/cluster/write_timeout "5s"

# 设置GRPC配置
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/cluster/grpc/addr "0.0.0.0:53001"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/cluster/grpc/endpoint "127.0.0.1:53001"

# 设置日志配置
echo "📝 设置日志配置..."
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/logging/level "info"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/logging/file ""
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/logging/max_size "100"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/logging/max_age "30"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/logging/max_backups "10"
etcdctl --endpoints=$ETCD_ENDPOINTS put $CONFIG_PREFIX/logging/compress "true"

echo ""
echo "✅ ETCD 配置设置完成！"
echo ""
echo "📋 查看已设置的配置:"
etcdctl --endpoints=$ETCD_ENDPOINTS get $CONFIG_PREFIX/ --prefix

echo ""
echo "🚀 现在可以使用以下命令启动 SkyTree:"
echo "   ./skyTree -config-source etcd -etcd-endpoints $ETCD_ENDPOINTS -etcd-key-prefix $CONFIG_PREFIX"
echo ""
echo "💡 或者设置环境变量:"
echo "   export ETCD_ENDPOINTS=$ETCD_ENDPOINTS"
echo "   export ETCD_KEY_PREFIX=$CONFIG_PREFIX"
echo "   ./skyTree -config-source etcd"
