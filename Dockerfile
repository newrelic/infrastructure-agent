FROM golang:1.14.4-buster

WORKDIR /tmp

RUN apt-get update \
    && apt-get -y install \
        locales \
        ruby \
        ruby-dev \
        build-essential \
        curl \
        rpm \
        libffi-dev \
        netcat \
        less \
        groff \
        unzip \
        jq \
        tree \
    && mkdir -p /build-utils \
    && echo "lc_all=en_us.utf-8" >> /etc/environment \
    && echo "en_us.utf-8 utf-8" >> /etc/locale.gen \
    && echo "lang=en_us.utf-8" > /etc/locale.conf \
    && locale-gen en_us.utf-8 \
    && go get golang.org/dl/go1.9.4 \
    && /go/bin/go1.9.4 download \
    && gem install --no-ri --no-rdoc ffi -v 1.10.0 \
    && gem install --no-ri --no-rdoc fpm -v 1.11.0 \
    && curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" \
    && unzip awscliv2.zip \
    && ./aws/install \
    && rm -rf awscliv2.zip \
        aws \
        /var/lib/apt/lists/*

WORKDIR /go/src/github.com/newrelic/infrastructure-agent

ENTRYPOINT []
