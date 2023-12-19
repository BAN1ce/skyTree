# 使用官方的 Golang 镜像作为基础镜像
FROM golang:latest

# 设置工作目录
WORKDIR /app

# 将本地的 Go 应用程序拷贝到容器中
COPY .. .

# 安装net-tools和curl和其他工具
RUN apt-get update && \
    apt-get install -y net-tools curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# 暴露应用程序监听的多个端口
EXPOSE 1883
EXPOSE 9526

# 运行应用程序
CMD ["./app"]