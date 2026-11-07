# syntax=docker/dockerfile:1
# Go build, diagnostics toolbox and network-facing lab gateway are separate.
FROM golang:1.27.3-alpine3.23 AS build
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod ./
COPY core ./core
COPY internal ./internal
COPY cmd ./cmd
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mrkvpn ./cmd/mrkvpn

FROM alpine:3.23 AS tooling
RUN apk add --no-cache \
    bash bind-tools ca-certificates curl iproute2 iptables iputils jq \
    nftables openssl su-exec tcpdump tini wireguard-tools
COPY --from=build /out/mrkvpn /usr/local/bin/mrkvpn
COPY --chmod=0755 docker/bootstrap-gateway.sh /usr/local/bin/bootstrap-gateway
COPY --chmod=0755 docker/cli-entrypoint.sh /usr/local/bin/cli-entrypoint
ENTRYPOINT ["/usr/local/bin/mrkvpn"]

FROM alpine:3.23 AS gateway
RUN apk add --no-cache bash ca-certificates iproute2 iptables tini wireguard-tools
COPY --chmod=0755 docker/gateway-entrypoint.sh /usr/local/bin/gateway-entrypoint
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/gateway-entrypoint"]


# Standalone management API + embedded static web panel.
FROM golang:1.27.3-alpine3.23 AS control-build
WORKDIR /control
COPY control/go.mod ./
COPY control/internal ./internal
COPY control/cmd ./cmd
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mrkvpn-control ./cmd/mrkvpn-control

FROM alpine:3.23 AS control
RUN apk add --no-cache ca-certificates tini \
    && addgroup -S -g 10001 mrkvpn \
    && adduser -S -D -H -u 10001 -G mrkvpn mrkvpn \
    && mkdir -p /data \
    && chown 10001:10001 /data \
    && chmod 0700 /data
COPY --from=control-build /out/mrkvpn-control /usr/local/bin/mrkvpn-control
USER 10001:10001
EXPOSE 8787
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/mrkvpn-control"]
CMD ["-listen", ":8787", "-data", "/data"]
