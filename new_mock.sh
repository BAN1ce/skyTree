a#!/bin/bash

 # 函数来遍历目录
 generate_mocks() {
   local dir="$1"
   for file in "$dir"/*; do
     if [ -f "$file" ] && [[ "$file" == *.go ]]; then
       # 使用 grep 检查文件是否包含 "interface" 关键字
       if grep -q "interface" "$file"; then
         # 获取包名并生成 mock 文件
         package_name=$(grep "package " "$file" | awk '{print $2}')
         mock_dir="${dir}/mocks"
         mock_file="${mock_dir}/$(basename "$file" .go)_mock.go"

         # 创建 mocks 目录（如果不存在）
         mkdir -p "$mock_dir"

         echo "Generating mock for $file => $mock_file"
         mockgen -source="$file" -destination="$mock_file" -package="$package_name"
       fi
     elif [ -d "$file" ]; then
       # 如果是目录，递归进入
       generate_mocks "$file"
     fi
   done
 }

 # 设置要生成 mock 的根目录
 ROOT_DIR="./"

 # 开始遍历
 generate_mocks "$ROOT_DIR"