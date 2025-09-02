# Use Ubuntu 16.04 as the base image
FROM ubuntu:16.04

# Define Go version
ARG GO_VERSION=1.25.0
# Define build-time arguments for the GitHub CLI version and architecture
ARG GH_VERSION='2.0.0'
ARG GH_ARCH='amd64'


# Install dependencies
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    expect \
    git \
    tar \
    gcc \
    g++ \
    gnupg2 \
    gnupg-agent \
    debsigs \
    rpm \
    build-essential \
    software-properties-common \
    python-software-properties \
    gcc-arm-linux-gnueabi \
    dpkg-sig \
    gcc-aarch64-linux-gnu


# # Manually install cross-compilers
# RUN apt-get install -y \
# gcc-5-multilib-mips-linux-gnu

# Install Go 1.25.0
RUN curl -sSL https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz -o go${GO_VERSION}.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz && \
    rm go${GO_VERSION}.linux-amd64.tar.gz

# Set Go environment variables
ENV PATH="/usr/local/go/bin:/go/bin:${PATH}"
ENV GOPATH="/go"

# Download and install the GitHub CLI
RUN curl -L https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${GH_ARCH}.deb -o gh_${GH_VERSION}_linux_${GH_ARCH}.deb && \
    dpkg -i gh_${GH_VERSION}_linux_${GH_ARCH}.deb && \
    rm gh_${GH_VERSION}_linux_${GH_ARCH}.deb

# Optional: Set Go environment flags
ENV GOFLAGS="-buildvcs=false"

# Optional: Configure git
RUN git config --global --add safe.directory /go/src/github.com/newrelic/infrastructure-agent
