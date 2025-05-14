#!/bin/sh # For output order tests
echo "STDOUT: Message 1"
echo "STDERR: Message 1" >&2
echo "STDOUT: Message 2"
echo "STDERR: Message 2" >&2
exit 0