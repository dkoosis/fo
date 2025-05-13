#!/bin/sh
echo "STDOUT: Script about to exit with $1"
echo "STDERR: Script stderr message before exiting $1" >&2
exit "$1"