#!/bin/bash -e

: "${ROOTFS_DIR:?ROOTFS_DIR must be set by pi-gen}"
STAGE_DIR="$(cd -- "$(dirname -- "$0")" && pwd)"

install -D -m 0755 "$STAGE_DIR/files/usr/local/bin/go-can" \
	"$ROOTFS_DIR/usr/local/bin/go-can"
install -D -m 0644 "$STAGE_DIR/files/etc/systemd/system/go-can.service" \
	"$ROOTFS_DIR/etc/systemd/system/go-can.service"
install -D -m 0644 "$STAGE_DIR/files/etc/systemd/system/go-can-canbus@.service" \
	"$ROOTFS_DIR/etc/systemd/system/go-can-canbus@.service"

chroot "$ROOTFS_DIR" useradd --system --user-group \
	--home-dir /var/lib/go-can --create-home \
	--shell /usr/sbin/nologin go-can
mkdir -p "$ROOTFS_DIR/var/lib/go-can/data"
chroot "$ROOTFS_DIR" chown -R go-can:go-can /var/lib/go-can
systemctl --root="$ROOTFS_DIR" enable go-can.service go-can-canbus@can0.service
