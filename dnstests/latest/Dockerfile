FROM golang:1.18 as builder


WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o dnstest main.go

######## Start a new stage from scratch #######
FROM alpine:3.16

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/dnstest .

RUN mv dnstest /bin

RUN apk --no-cache upgrade

RUN apk add --no-cache --upgrade \
    ca-certificates \
    # Embed required dlls:
    # ldd /usr/bin/newrelic-infra
    #   /lib64/ld-linux-x86-64.so.2 (0x7f2bbbd0f000)
    #   libpthread.so.0 => /lib64/ld-linux-x86-64.so.2 (0x7f2bbbd0f000)
    #   libc.so.6 => /lib64/ld-linux-x86-64.so.2 (0x7f2bbbd0f000)
    # As musl and glibc are compatible, this symlink fixes the missing dependency
    && mkdir /lib64 \
    && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2 \
    && apk add --no-cache tini curl bind-tools coreutils netcat-openbsd

ENV GODEBUG=netdns=2

# Tini is now available at /sbin/tini
ENTRYPOINT ["/bin/sh", "-c"]
CMD ["/sbin/tini -- dnstest"]