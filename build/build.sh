#!/bin/bash

# 设置Go应用程序的目录
APP_DIR="../cmd"

# 设置Dockerfile路径
DOCKERFILE_PATH="$APP_DIR/../build/Dockerfile"

# 设置Docker镜像的名称和标签
IMAGE_NAME="skytree"
IMAGE_TAG="latest"

# 进入应用程序目录
cd "$APP_DIR" || exit

# 编译Go应用程序
GOOS=linux GOARCH=amd64 go build -o app
#
# 构建Docker镜像
docker build -t "$IMAGE_NAME:$IMAGE_TAG" -f "$DOCKERFILE_PATH" .

# 删除本地的旧容器和镜像
docker stop "$IMAGE_NAME" || true && docker rm "$IMAGE_NAME" || true
docker rmi $(docker images -f "dangling=true" -q) || true

# 运行Docker容器
docker run -d --name="$IMAGE_NAME" "$IMAGE_NAME:$IMAGE_TAG"

# 设置开机启动
echo "@reboot cd $APP_DIR && docker run -d --name=$IMAGE_NAME $IMAGE_NAME:$IMAGE_TAG" | crontab -

# 输出成功信息
echo "Go应用程序已经成功编译、打包成Docker镜像，并设置为开机自动执行。"
