#!/bin/sh

echo "Enabling $1 bus ..."
ip link set "$1" type can bitrate 500000
ip link set "$1" up