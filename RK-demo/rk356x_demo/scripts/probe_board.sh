#!/bin/sh
set -eu

echo "== system =="
uname -a || true
cat /etc/os-release 2>/dev/null || true
cat /proc/device-tree/model 2>/dev/null || true
printf '\n'

echo "== cpu =="
cat /proc/cpuinfo | sed -n '1,80p' || true
printf '\n'

echo "== compiler/tools =="
for t in gcc g++ make pkg-config v4l2-ctl ffmpeg gst-launch-1.0; do
  printf '%-16s' "$t"
  command -v "$t" || true
done
printf '\n'

echo "== framebuffer =="
ls -l /dev/fb* 2>/dev/null || true
for fb in /sys/class/graphics/fb*; do
  [ -e "$fb" ] || continue
  echo "-- $fb"
  cat "$fb/name" 2>/dev/null || true
  cat "$fb/virtual_size" 2>/dev/null || true
  cat "$fb/bits_per_pixel" 2>/dev/null || true
done
printf '\n'

echo "== input =="
ls -l /dev/input 2>/dev/null || true
cat /proc/bus/input/devices 2>/dev/null || true
printf '\n'

echo "== video =="
ls -l /dev/video* /dev/media* 2>/dev/null || true
if command -v v4l2-ctl >/dev/null 2>&1; then
  v4l2-ctl --list-devices || true
  for v in /dev/video*; do
    [ -e "$v" ] || continue
    echo "-- $v formats"
    v4l2-ctl -d "$v" --list-formats-ext || true
  done
fi
printf '\n'

echo "== rknn/npu =="
ls -l /dev/rknpu* /dev/dri* 2>/dev/null || true
ldconfig -p 2>/dev/null | grep -Ei 'rknn|rknpu|opencv' || true
find /usr /lib /opt -maxdepth 4 \( -name 'librknnrt.so*' -o -name 'rknn_api.h' -o -name '*opencv*' \) 2>/dev/null | head -80 || true
