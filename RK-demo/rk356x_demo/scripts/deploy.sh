#!/bin/sh
set -eu

HOST="${HOST:-192.168.1.145}"
SSH_USER="${SSH_USER:-root}"
PASS="${PASS:-123456}"
REMOTE_DIR="${REMOTE_DIR:-/root/rk356x_demo}"

cd "$(dirname "$0")/.."

if ! command -v expect >/dev/null 2>&1; then
  echo "expect is required on the host machine." >&2
  exit 1
fi

expect <<EOF
set timeout 60
spawn ssh -o StrictHostKeyChecking=accept-new $SSH_USER@$HOST "mkdir -p $REMOTE_DIR"
expect {
  -re "(?i)password:" { send "$PASS\r"; exp_continue }
  eof
}
EOF

expect <<EOF
set timeout 180
spawn scp -o StrictHostKeyChecking=accept-new -r Makefile src scripts assets model lib build $SSH_USER@$HOST:$REMOTE_DIR/
expect {
  -re "(?i)password:" { send "$PASS\r"; exp_continue }
  eof
}
EOF

expect <<EOF
set timeout 60
spawn ssh -o StrictHostKeyChecking=accept-new $SSH_USER@$HOST "cd $REMOTE_DIR && chmod +x scripts/*.sh build/rk356x_demo && mkdir -p /root/rk356x_demo/model /root/rk356x_demo/lib"
expect {
  -re "(?i)password:" { send "$PASS\r"; exp_continue }
  eof
}
EOF

echo "Deployed prebuilt app at $SSH_USER@$HOST:$REMOTE_DIR"
echo "Run with:"
echo "  ssh $SSH_USER@$HOST 'cd $REMOTE_DIR && LD_LIBRARY_PATH=$REMOTE_DIR/lib CAMERA_DEV=/dev/video9 ./build/rk356x_demo'"
