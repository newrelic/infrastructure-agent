FROM golang:1.14.4-buster

ARG GH_VERSION='2.0.0'
ARG AWS_VERSION='2.2.45'

RUN apt-get update \
    && apt-get -y install \
        rpm \
        gnupg2 \
        gpg-agent \
        debsigs \
        unzip \
        zip

RUN curl -L https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_amd64.deb -o gh_${GH_VERSION}_linux_amd64.deb
RUN dpkg -i gh_${GH_VERSION}_linux_amd64.deb

RUN curl -L https://awscli.amazonaws.com/awscli-exe-linux-x86_64-${AWS_VERSION}.zip -o awscliv2.zip \
    && unzip awscliv2.zip \
    && ./aws/install
