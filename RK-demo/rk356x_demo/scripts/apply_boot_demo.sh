#!/bin/sh
set -eu

install -m 755 /tmp/rk356x_demo /root/rk356x_demo/build/rk356x_demo
install -m 755 /tmp/install_autostart.sh /root/rk356x_demo/scripts/install_autostart.sh
install -m 644 /tmp/camera.bmp /root/rk356x_demo/assets/camera.bmp

if [ -f /etc/init.d/S50launch_demo ]; then
  cp /etc/init.d/S50launch_demo /etc/init.d/S50launch_demo.bak.rk356x
  mv /etc/init.d/S50launch_demo /etc/init.d/K50launch_demo.disabled
fi

killall rk356x-demo qcamera qv4l2 qlauncher launcher weston 2>/dev/null || true
/root/rk356x_demo/scripts/install_autostart.sh

echo "--- status"
ls -l /etc/init.d/S50launch_demo /etc/init.d/K50launch_demo.disabled /etc/init.d/S50launch_demo.bak.rk356x /etc/init.d/S99rk356x_demo 2>/dev/null || true
ps w | grep -E 'rk356x_demo|rk356x-demo|weston|launcher' | grep -v grep || true
tail -40 /tmp/rk356x_demo.log 2>/dev/null || true
