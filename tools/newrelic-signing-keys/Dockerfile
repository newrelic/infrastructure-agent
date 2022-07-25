FROM ruby:3-alpine3.15

RUN apk add --update ruby-dev gcc make musl-dev libffi-dev xz-dev tar && \
    gem install fpm


RUN mkdir -p /fpm/opt/newrelic-signing-keys/keys
COPY .fpm /fpm
COPY keys /fpm/opt/newrelic-signing-keys/keys
COPY newrelic-signing-keys /fpm/newrelic-signing-keys

WORKDIR /fpm

ENTRYPOINT ["fpm"]
CMD ["."]