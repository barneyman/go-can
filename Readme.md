# Canbus

## Bench testing

Create a fake canbus - on the host

```
sudo modprobe vcan
sudo ip link add dev vcan0 type vcan
sudo ip link set up can0
```

In this code, choose `Launch Package - run on bench` run configuration

## Raspberry Pi image

The `Build Raspberry Pi image` GitHub Actions workflow builds a minimal ARM64
Raspberry Pi OS (Debian trixie) image on every push. Download the
`go-can-rpi-image` artifact from the workflow run. The image enables the
`go-can` systemd service at boot and configures `can0` for 500 kbit/s.

The CAN controller must be enabled in the Pi's device-tree/boot configuration
so that the `can0` interface exists. The service runs as the `go-can` user and
stores CAN archives under `/var/lib/go-can/data`.

### cangen 

```
# Throttle: 50% encoded as one byte (0x32); repeat every 20 ms
cangen vcan0 -I 180 -L 8 -D 3200000000000000 -g 20

# Steering: +12.34° encoded as signed centidegrees in little-endian (0x04D2)
cangen vcan0 -I 181 -L 8 -D D204000000000000 -g 20

# Steering: -12.34° (0xFB2E in signed 16-bit two's complement)
cangen vcan0 -I 181 -L 8 -D 2EFB000000000000 -g 20
```

### cansend

```
# request Engine RPM
cansend vcan0 7DF#02010C0000000000
cangen vcan0 -I 7E8 -L 8 -D 04410C0FA0000000 -n 1 -v

# response Speed
cansend vcan0 7E8#04410C0FA0000000
cangen vcan0 -I 7E8 -L 8 -D 03410D4300000000 -n 1 -v
```