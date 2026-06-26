# Runtime image for the locally compiled SkyTree broker.
FROM ubuntu:24.04

# 安装必要的运行时依赖
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    wget \
    && rm -rf /var/lib/apt/lists/*

# 创建非root用户
RUN groupadd -g 1001 skytree && \
    useradd -u 1001 -g skytree -s /bin/bash -m skytree

# 设置工作目录
WORKDIR /app

# 复制宿主机预编译二进制文件
COPY .docker-build/skytree /app/skytree

# 复制宿主机预构建控制台前端资源
COPY web/console/dist /app/web/console/dist

# 复制配置文件
COPY etc/config.yaml /app/config.yaml

# 创建数据目录
RUN mkdir -p /app/data && \
    chmod +x /app/skytree && \
    chown -R skytree:skytree /app

# 切换到非root用户
USER skytree

# 暴露端口
# 1883: MQTT TCP
# 8080: MQTT WebSocket  
# 8081: MQTT WebSocket Secure
# 9526: HTTP API
# 63001: Cluster Raft
# 8091: gRPC
EXPOSE 1883 8080 8081 9526 63001 8091

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9526/health || exit 1

# 启动应用
ENTRYPOINT ["/app/skytree", "-config", "/app/config.yaml"]
