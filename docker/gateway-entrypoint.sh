#!/bin/sh
# Linux lab gateway only. NET_ADMIN is restricted to this container namespace.
set -eu
umask 077

CONF=/etc/wireguard/wg0.conf
EGRESS="${MRKVPN_EGRESS_IFACE:-eth0}"
UP=0
TUN_NET=""

die() { echo "mrkvpn gateway: $*" >&2; exit 1; }
case "$EGRESS" in
    ""|*[!a-zA-Z0-9_.:-]*) die "invalid egress interface name" ;;
esac
[ "$EGRESS" != "wg0" ] || die "egress interface cannot be wg0"
[ -f "$CONF" ] || die "missing generated wg0.conf; render it first"
[ "$(stat -c '%a' "$CONF")" = "600" ] || die "wg0.conf must have permission 0600"
# Restrict obvious shell-hook surprises; always use the trusted renderer.
if grep -Eiq '^[[:space:]]*(PreUp|PostUp|PreDown|PostDown)[[:space:]]*=' "$CONF"; then
    die "wg-quick hook directives are not accepted by this lab gateway"
fi
ip link show dev "$EGRESS" >/dev/null 2>&1 || die "egress interface not found"

cleanup() {
    if [ -n "$TUN_NET" ]; then
        iptables -w -t nat -D POSTROUTING -s "$TUN_NET" -o "$EGRESS" -j MASQUERADE 2>/dev/null || :
        iptables -w -D FORWARD -i "$EGRESS" -o wg0 -d "$TUN_NET" -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT 2>/dev/null || :
        iptables -w -D FORWARD -i wg0 -o "$EGRESS" -s "$TUN_NET" -j ACCEPT 2>/dev/null || :
    fi
    if [ "$UP" -eq 1 ]; then
        wg-quick down wg0 >/dev/null 2>&1 || :
    fi
}
trap cleanup EXIT
trap 'exit 0' INT TERM

wg-quick up wg0 || die "wg-quick up failed (check Linux kernel WireGuard support)"
UP=1
TUN_NET="$(ip -4 -o route show dev wg0 scope link | awk 'NR==1 { print $1 }')"
case "$TUN_NET" in
    */*) : ;;
    *) die "unable to detect WireGuard tunnel subnet" ;;
esac

# The NAT rules apply only in the isolated container network namespace.
iptables -w -A FORWARD -i wg0 -o "$EGRESS" -s "$TUN_NET" -j ACCEPT
iptables -w -A FORWARD -i "$EGRESS" -o wg0 -d "$TUN_NET" -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
iptables -w -t nat -A POSTROUTING -s "$TUN_NET" -o "$EGRESS" -j MASQUERADE

echo "mrkvpn lab gateway started (IPv4 only). No client kill switch is provided."
while :; do
    sleep 3600 &
    wait "$!" || :
done
