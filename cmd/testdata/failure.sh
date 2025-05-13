#!/bin/sh
echo "STDOUT: Output from failure.sh before failing"
echo "STDERR: Error message from failure.sh" >&2
exit 1