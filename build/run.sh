#!/bin/bash

current_dir=$(pwd)

echo "current_dir: $current_dir"

rm -rf $current_dir/../data

go run $current_dir/../cmd/main.go