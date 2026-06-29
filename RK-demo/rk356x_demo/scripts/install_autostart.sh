#!/bin/sh
set -eu

APP_DIR="${APP_DIR:-/root/rk356x_demo}"
APP_BIN="$APP_DIR/build/rk356x_demo"
INIT_FILE="/etc/init.d/S99rk356x_demo"

if [ ! -x "$APP_BIN" ]; then
  echo "Missing executable: $APP_BIN" >&2
  exit 1
fi

cat > "$INIT_FILE" <<'EOF'
#!/bin/sh

APP_DIR=/root/rk356x_demo
APP_BIN="$APP_DIR/build/rk356x_demo"
LOG_FILE="/tmp/rk356x_demo.log"
PID_FILE="/var/run/rk356x_demo.pid"
RUNNER="/tmp/rk356x_demo_boot_runner.sh"

case "$1" in
  start)
    /etc/init.d/S31bootanim.sh stop 2>/dev/null || bootanim stop 2>/dev/null || true
    killall rk356x_demo rk356x-demo qcamera qv4l2 qlauncher launcher weston bootanim 2>/dev/null || true
    cd "$APP_DIR" || exit 1
    cat > "$RUNNER" <<'RUNNER_EOF'
#!/bin/sh
APP_DIR=/root/rk356x_demo
APP_BIN="$APP_DIR/build/rk356x_demo"
LOG_FILE="/tmp/rk356x_demo.log"
PID_FILE="/var/run/rk356x_demo.pid"

sleep 10
/etc/init.d/S31bootanim.sh stop 2>/dev/null || bootanim stop 2>/dev/null || true
killall rk356x_demo rk356x-demo qcamera qv4l2 qlauncher launcher weston bootanim 2>/dev/null || true
echo 0 > /sys/class/graphics/fb0/blank 2>/dev/null || true
cd "$APP_DIR" || exit 1
export LD_LIBRARY_PATH="$APP_DIR/lib:${LD_LIBRARY_PATH:-}"
export CAMERA_DEV=/dev/video9
echo "starting rk356x_demo after 10s delay" > "$LOG_FILE"
nohup "$APP_BIN" >> "$LOG_FILE" 2>&1 < /dev/null &
echo $! > "$PID_FILE"
RUNNER_EOF
    chmod +x "$RUNNER"
    nohup "$RUNNER" >/tmp/rk356x_demo_boot_runner.log 2>&1 < /dev/null &
    ;;
  stop)
    if [ -f "$PID_FILE" ]; then
      kill "$(cat "$PID_FILE")" 2>/dev/null || true
      rm -f "$PID_FILE"
    fi
    killall rk356x_demo_boot_runner.sh 2>/dev/null || true
    killall rk356x_demo 2>/dev/null || true
    ;;
  restart)
    "$0" stop
    "$0" start
    ;;
  *) echo "Usage: $0 {start|stop|restart}"; exit 1 ;;
esac
EOF

chmod +x "$INIT_FILE"

echo "Installed autostart: $INIT_FILE"
