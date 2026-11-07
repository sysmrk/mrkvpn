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
    nftables openssl tcpdump tini wireguard-tools
COPY --from=build /out/mrkvpn /usr/local/bin/mrkvpn
COPY --chmod=0755 docker/bootstrap-gateway.sh /usr/local/bin/bootstrap-gateway
ENTRYPOINT ["/usr/local/bin/mrkvpn"]

FROM alpine:3.23 AS gateway
RUN apk add --no-cache bash ca-certificates iproute2 iptables tini wireguard-tools
COPY --chmod=0755 docker/gateway-entrypoint.sh /usr/local/bin/gateway-entrypoint
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/gateway-entrypoint"]
