FROM golang:1.18 as builder


WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -tags netgo -installsuffix netgo -o agent_connection_code_no_libc main.go
RUN CGO_ENABLED=1 GOOS=linux go build -o agent_connection_code_libc main.go

######## Start a new stage from scratch #######
FROM alpine:3.16

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/agent_connection_code_no_libc .
COPY --from=builder /app/agent_connection_code_libc .

RUN mv agent_connection_code_no_libc /bin
RUN mv agent_connection_code_libc /bin

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

# Tini is now available at /sbin/tini
ENTRYPOINT ["/bin/sh", "-c"]
CMD ["/sbin/tini -- agent_connection_code_no_libc & while true; do curl https://infra-api.newrelic.com/cdn-cgi/trace; dig https://infra-api.newrelic.com/cdn-cgi/trace;  sleep 30;done"]