#!/bin/sh

echo "Enabling $1 bus ..."
ip link set "$1" type can bitrate 500000 listen-only on
ip link set "$1" up