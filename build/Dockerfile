FROM golang:1.19.5-buster

ARG GH_VERSION='2.0.0'
ARG GH_ARCH='amd64'

RUN apt-get update \
    && apt-get -y install \
        rpm \
        gnupg2 \
        gpg-agent \
        debsigs \
        unzip \
        zip

RUN curl -L https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${GH_ARCH}.deb -o gh_${GH_VERSION}_linux_${GH_ARCH}.deb
RUN dpkg -i gh_${GH_VERSION}_linux_${GH_ARCH}.deb
