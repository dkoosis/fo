#!/bin/sh # For signal propagation tests
echo "Starting signal test script (PID: $$)"
echo "Will sleep for 5 seconds unless interrupted" # Shorter for tests
trap 'echo "Caught signal, exiting cleanly"; exit 42' INT TERM
sleep 5
echo "Finished sleeping"
exit 0
