# RK3568 Embedded Vision Training Project

## Build

```bash
./scripts/build_in_docker.sh
```

Outputs are generated in `output/`.

## Publish

```bash
cp -f output/* /usr/share/nginx/html/rk/
chmod 644 /usr/share/nginx/html/rk/*
```

The RK3568 board can download binaries from `http://8.129.31.210/rk/`.

Direct SSH/SCP is not the main path now. Use the cloud bridge to tell the board
to download and run binaries when the board heartbeat is online.

## Run

LCD:
```bash
/userdata/vision/bin/lcd_demo gradient
/userdata/vision/bin/lcd_demo split
/userdata/vision/bin/lcd_demo solid '#0080ff'
/userdata/vision/bin/lcd_demo bmp /userdata/vision/res/test.bmp
/userdata/vision/bin/vision_panel --seconds 10 /dev/input/event6
```

Touch:
```bash
/userdata/vision/bin/touch_test /dev/input/event6 1024 600
/userdata/vision/bin/touch_test --seconds 10 /dev/input/event6 1024 600
```

Camera:
```bash
/userdata/vision/bin/camera_capture --probe /dev/video-camera0
/userdata/vision/bin/camera_capture --timeout 5 /dev/video-camera0 /userdata/vision/capture.ppm 640 480
```

## Bridge Command

Use the cloud-side bridge directly:

```bash
./scripts/bridge_command.py 'echo hello && uname -m && hostname -i'
./scripts/bridge_command.py 'cd /userdata/vision/bin && wget -O lcd_demo http://8.129.31.210/rk/lcd_demo && chmod +x lcd_demo && ./lcd_demo split'
./scripts/bridge_command.py 'cd /userdata/vision/bin && wget -O camera_capture http://8.129.31.210/rk/camera_capture && chmod +x camera_capture && ./camera_capture --probe /dev/video-camera0'
./scripts/board_smoke_test.py
```

## Acceptance Mapping

- LCD UI: `lcd_demo` supports solid color, split color, gradient, and 24/32-bit BMP. `vision_panel` draws a three-zone icon panel.
- Touch interaction: `touch_test` reads touch coordinates, maps three regions, reports swipes, and supports timed bridge tests. `vision_panel` combines LCD regions with touch hit detection.
- Camera capture: `camera_capture` opens V4L2, probes RKISP nodes, captures one NV12/YUYV frame when the camera pipeline accepts the format, saves PPM, and closes the device.
- Vision recognition: next stage, RKNN runtime integration after base hardware modules are verified.

## Current Board Notes

- Last known framebuffer: `/dev/fb0`, 1024x600, 32 bpp.
- Last known touch input: `/dev/input/event6`.
- Camera symlink: `/dev/video-camera0 -> /dev/video0`.
- RKISP accepts NV12 mplane format, but stream start currently fails with `VIDIOC_STREAMON: No such device`. `v4l2-ctl` fails the same way, and the media graph has no sensor entity feeding `rockchip-csi2-dphy0`. See `docs/camera_diagnosis.md`.
