#!/usr/bin/env bash
set -euo pipefail

echo "DEVICES"
ls -l /dev/video* /dev/media* /dev/v4l* 2>/dev/null || true

echo "VIDEO_NAMES"
for d in /sys/class/video4linux/*; do
  [ -e "$d" ] || continue
  echo "===$d==="
  cat "$d/name" 2>/dev/null || true
  readlink -f "$d/device" 2>/dev/null || true
done

echo "MEDIA_TOPOLOGY"
media-ctl -p -d /dev/media0 2>&1 || true

echo "V4L2_FORMATS"
v4l2-ctl -d /dev/video-camera0 --list-formats-ext 2>&1 || true

echo "V4L2_STREAM_TEST"
rm -f /userdata/vision/frame.raw
v4l2-ctl -d /dev/video-camera0 \
  --set-fmt-video=width=800,height=600,pixelformat=NV12 \
  --stream-mmap=3 \
  --stream-count=1 \
  --stream-to=/userdata/vision/frame.raw 2>&1 || true
ls -lh /userdata/vision/frame.raw 2>/dev/null || true
