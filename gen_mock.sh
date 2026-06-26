#!/bin/bash

# 设置要生成 mock 的根目录
ROOT_DIR="./pkg"

# 设置生成的 mock 文件保存路径
DESTINATION="./mocks"

# 函数来遍历目录
generate_mocks() {
  local dir="$1"
  for file in "$dir"/*; do
    # 跳过 mocks 目录和隐藏文件
    if [[ "$file" == *"/mocks"* ]] || [[ "$(basename "$file")" == .* ]]; then
      continue
    fi
    
    if [ -f "$file" ] && [[ "$file" == *.go ]]; then
      # 使用 grep 检查文件是否包含 "interface" 关键字
      if grep -q "interface" "$file"; then
        # 获取包名
        package_name=$(grep "package " "$file" | awk '{print $2}')
        
        # 构造生成的 mock 文件路径
        relative_path=$(echo "$file" | sed "s|^$ROOT_DIR/||")
        dir_path=$(dirname "$relative_path")
        file_name=$(basename "$file" .go)
        
        # 创建目标目录
        target_dir="${DESTINATION}/${dir_path}"
        mkdir -p "$target_dir"
        
        mock_file="${target_dir}/${file_name}_mock.go"
        echo "Generating mock for $file => $mock_file"
        
        # 生成 mock 文件，使用 mocks 作为包名
        mockgen -source="$file" -destination="$mock_file" -package="mocks"
      fi
    elif [ -d "$file" ]; then
      # 如果是目录，递归进入
      generate_mocks "$file"
    fi
  done
}

# 创建目标文件夹（如果不存在）
mkdir -p "$DESTINATION"

# 开始遍历 pkg 目录
generate_mocks "$ROOT_DIR"
