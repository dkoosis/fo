#!/bin/sh
for i in $(seq 1 100); do # Reduced for faster tests, focus on limit message
  printf "STDOUT: Line %04d - This is test content to generate output.\n" $i
done
echo "Script complete"
exit 0