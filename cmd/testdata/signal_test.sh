#!/bin/sh
echo "Starting signal test script (PID: $$)"
echo "Will sleep for 10 seconds unless interrupted"
trap 'echo "Caught signal, exiting cleanly"; exit 42' INT TERM
sleep 10
echo "Finished sleeping"
exit 0