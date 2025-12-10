FROM golang:1.25.1@sha256:bb979b278ffb8d31c8b07336fd187ef8fafc8766ebeaece524304483ea137e96 AS go_compiler

COPY src/ /app/
WORKDIR /app

RUN go build -o /inbound_parser


FROM debian:12.12@sha256:7dc1e2b39b0147079a16347915e9583cb2f239d4896fe2beac396b979e5c06a9
RUN apt-get update && \
    apt-get -y install \
        # dth-renovate-deb-bookworm-amd64:
        clamav-daemon=1.0.9+dfsg-1~deb12u1 \
        # dth-renovate-deb-bookworm-amd64:
        curl=7.88.1-10+deb12u14 && \
    apt-get clean

COPY --from=go_compiler /inbound_parser /var/lib/inbound_parser
RUN chmod +x /var/lib/inbound_parser

ENV PORT=443
EXPOSE 443
ENTRYPOINT ["/var/lib/inbound_parser"]

LABEL org.opencontainers.image.source=forgejo.ibmgcloud.net/dth/restic-rest-server
