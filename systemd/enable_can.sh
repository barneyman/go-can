#!/bin/sh

echo "Enabling $1 bus ..."
# listen only - no sending
ip link set "$1" type can bitrate 500000 
# and bring it up
ip link set "$1" up
