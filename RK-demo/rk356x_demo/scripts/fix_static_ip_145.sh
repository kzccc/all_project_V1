#!/bin/sh
set -eu

IFACE="${IFACE:-wlan0}"
IPADDR="${IPADDR:-192.168.1.145/24}"
GATEWAY="${GATEWAY:-192.168.1.1}"

backup="/etc/dhcpcd.conf.bak.$(date +%Y%m%d%H%M%S)"
cp /etc/dhcpcd.conf "$backup"

awk '
  /# BEGIN RK356X_STATIC_IP/ { skip=1; next }
  /# END RK356X_STATIC_IP/ { skip=0; next }
  !skip { print }
' /etc/dhcpcd.conf > /tmp/dhcpcd.conf.new

cat >> /tmp/dhcpcd.conf.new <<EOF_CONF

# BEGIN RK356X_STATIC_IP
interface $IFACE
static ip_address=$IPADDR
static routers=$GATEWAY
static domain_name_servers=$GATEWAY 8.8.8.8
# END RK356X_STATIC_IP
EOF_CONF

mv /tmp/dhcpcd.conf.new /etc/dhcpcd.conf

cat > /etc/init.d/S81static-ip <<'EOF_INIT'
#!/bin/sh

IFACE="wlan0"
IPADDR="192.168.1.145/24"
GATEWAY="192.168.1.1"

case "$1" in
  start|restart|reload)
    for i in 1 2 3 4 5; do
      ip link show "$IFACE" >/dev/null 2>&1 && break
      sleep 1
    done
    ip link set "$IFACE" up 2>/dev/null || true
    ip addr flush dev "$IFACE" scope global 2>/dev/null || true
    ip addr add "$IPADDR" dev "$IFACE" 2>/dev/null || true
    ip route replace default via "$GATEWAY" dev "$IFACE" src 192.168.1.145 metric 303 2>/dev/null || true
    ;;
  stop)
    ;;
  *)
    echo "Usage: $0 {start|stop|restart}"
    exit 1
    ;;
esac

exit 0
EOF_INIT

chmod +x /etc/init.d/S81static-ip

ip addr add "$IPADDR" dev "$IFACE" 2>/dev/null || true
ip route replace default via "$GATEWAY" dev "$IFACE" src 192.168.1.145 metric 303 2>/dev/null || true

echo "Backed up dhcpcd config to $backup"
echo "Installed /etc/init.d/S81static-ip"
ip -4 addr show "$IFACE"
ip route
