#!/bin/bash

# Root directory to scan for mock generation.
ROOT_DIR="./"

# Destination directory for generated mock files.
DESTINATION="./mocks"

# Walk directories recursively and generate mocks for Go interfaces.
generate_mocks() {
  local dir="$1"
  for file in "$dir"/*; do
    echo "Checking $dir"
    if [ -f "$file" ] && [[ "$file" == *.go ]]; then
      # Use grep as a quick pre-filter for files containing interfaces.
      if grep -q "interface" "$file"; then
        # Build the generated mock file path.

        # Read the package name and generate the mock file.
        package_name=$(grep "package " "$file" | awk '{print $2}')
        mock_file="${DESTINATION}/$package_name/$(basename "$file" .go)_mock.go"
        echo "Generating mock for $file => $mock_file"
        mockgen -source="$file" -destination="$mock_file" -package="$package_name"
      fi
    elif [ -d "$file" ]; then
      # Recurse into subdirectories.
      generate_mocks "$file"
    fi
  done
}

# Create destination directory if needed.
mkdir -p "$DESTINATION"

# Start traversal.
generate_mocks "$ROOT_DIR"
