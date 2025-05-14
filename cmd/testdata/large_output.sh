#!/bin/sh # For buffer limit tests (reduced for speed)
for i in $(seq 1 100); do
  printf "STDOUT: Line %04d - This is test content to generate output.\n" $i
done
echo "Script complete"
exit 0