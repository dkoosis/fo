#!/bin/bash
# scripts/fix_formatting.sh
# Automatically fixes common formatting issues in YAML and Makefiles

set -e

echo "ℹ️ Scanning YAML files..."
yaml_files=$(find . -name "*.yaml" -o -name "*.yml" | grep -v "vendor/" | grep -v ".git/")

for file in $yaml_files; do
  echo "Processing: $file"
  # Create backup
  cp "$file" "${file}.bak"
  
  # Check if file needs YAML header
  if [ -s "$file" ] && ! grep -q "^---" "$file"; then
    { echo "---"; cat "$file"; } > "${file}.tmp" && mv "${file}.tmp" "$file"
    echo "  • Added YAML header"
  fi
  
  # Fix spaces before comments
  sed -i '' -E 's/([^ ])[[:space:]]*#/\1  #/g' "$file"
  
  # Fix spaces inside braces
  sed -i '' -E 's/{[[:space:]]+/{/g; s/[[:space:]]+}/}/g' "$file"
  
  # Check if changes were made
  if ! cmp -s "${file}.bak" "$file"; then
    echo "✅ Fixed: $file"
  fi
  
  # Remove backup
  rm "${file}.bak"
done

echo "ℹ️ Scanning Makefiles..."
make_files=$(find . -name "Makefile" -o -name "*.mk" | grep -v "vendor/" | grep -v ".git/")

for file in $make_files; do
  echo "Processing: $file"
  # Create backup
  cp "$file" "${file}.bak"
  
  # Fix tab issues in Makefiles with awk
  awk 'BEGIN {prev_line_target=0; prev_line_recipe=0}
  {
    if ($0 ~ /^[ ]+[^ ]/ && (prev_line_target || prev_line_recipe)) {
      match($0, /^[ ]+/);
      print "\t" substr($0, RLENGTH+1);
    } else {
      print $0;
    }
    
    if ($0 ~ /:([^\\]*)$/) {
      prev_line_target = 1;
      prev_line_recipe = 0;
    }
    else if ($0 ~ /^\t/) {
      prev_line_target = 0;
      prev_line_recipe = 1;
    }
    else {
      prev_line_target = 0;
      prev_line_recipe = 0;
    }
  }' "$file" > "${file}.tmp"
  
  if [ -f "${file}.tmp" ]; then
    mv "${file}.tmp" "$file"
  fi
  
  # Check if changes were made
  if ! cmp -s "${file}.bak" "$file"; then
    echo "✅ Fixed: $file"
  fi
  
  # Remove backup
  rm "${file}.bak"
done

echo "✅ Format fixing complete!"