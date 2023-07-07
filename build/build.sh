#!/bin/bash

# Get the current directory
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Set the version and build time
VERSION="1.0.0"
BUILD_TIME=$(date +"%Y-%m-%d_%H:%M:%S")

# Set the output directory
OUTPUT_DIR="$DIR/bin"

# Create the output directory
mkdir -p $OUTPUT_DIR

# Define the platform array
PLATFORMS=("linux/amd64" "linux/386" "windows/amd64" "windows/386" "darwin/amd64")

# Loop through all the platforms and compile
for PLATFORM in "${PLATFORMS[@]}"
do
  # Separate the OS and architecture
  OS=$(echo $PLATFORM | cut -d '/' -f 1)
  ARCH=$(echo $PLATFORM | cut -d '/' -f 2)

  # Set the output filename
  if [ $OS = "windows" ]; then
    OUTPUT_NAME="myapp.exe"
  else
    OUTPUT_NAME="myapp"
  fi

  # Print the platform being compiled
  echo "Compiling for $OS/$ARCH..."

  # Compile the binary file
  GOOS=$OS GOARCH=$ARCH go build -ldflags "-X main.Version=$VERSION -X main.BuildTime=$BUILD_TIME" -o "$OUTPUT_DIR/$OS-$ARCH/$OUTPUT_NAME" "$DIR/main.go"
done

# Print compilation complete
echo "All platforms compiled successfully!"
