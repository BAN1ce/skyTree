#!/bin/bash

# 设置要生成 mock 的根目录
ROOT_DIR="./"

# 设置生成的 mock 文件保存路径
DESTINATION="./mocks"

# 函数来遍历目录
generate_mocks() {
  local dir="$1"
  for file in "$dir"/*; do
    echo "Checking $dir"
    if [ -f "$file" ] && [[ "$file" == *.go ]]; then
      # 使用 grep 检查文件是否包含 "interface" 关键字
      if grep -q "interface" "$file"; then
        # 构造生成的 mock 文件名

        # 获取包名并生成 mock 文件
        package_name=$(grep "package " "$file" | awk '{print $2}')
        mock_file="${DESTINATION}/$package_name/$(basename "$file" .go)_mock.go"
        echo "Generating mock for $file => $mock_file"
        mockgen -source="$file" -destination="$mock_file" -package="$package_name"
      fi
    elif [ -d "$file" ]; then
      # 如果是目录，递归进入
      generate_mocks "$file"
    fi
  done
}

# 创建目标文件夹（如果不存在）
mkdir -p "$DESTINATION"

# 开始遍历
generate_mocks "$ROOT_DIR"
