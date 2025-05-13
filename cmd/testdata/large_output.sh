#!/bin/sh
# Generate enough output to exceed a 1MB buffer limit
# Each line is ~200 bytes, so we need ~5000 lines to generate >1MB
for i in $(seq 1 5000); do
  printf "STDOUT: Line %04d - This is test content to generate output that will exceed our buffer limit of 1MB. We're making each line reasonably long to reach the limit quickly.\n" $i
done
echo "Script complete - generated approximately 1MB of output"
exit 0