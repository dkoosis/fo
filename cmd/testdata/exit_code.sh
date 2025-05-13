#!/bin/sh
echo "STDOUT: Testing exit code $1"
echo "STDERR: Will exit with $1" >&2
exit "$1"