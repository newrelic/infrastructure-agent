FROM ubuntu:22.04 as ci

ARG TAG=1.27.0
# TODO: accept aws credentials

RUN apt update
RUN DEBIAN_FRONTEND=noninteractive apt install -y make ansible wget unzip git

RUN wget "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip"  && unzip awscli-exe-linux-x86_64.zip && ./aws/install

RUN wget https://go.dev/dl/go1.18.3.linux-amd64.tar.gz && rm -rf /usr/local/go && tar -C /usr/local -xzf go1.18.3.linux-amd64.tar.gz
ENV PATH=$PATH:/usr/local/go/bin

RUN mkdir -p /srv/newrelic

RUN git clone https://github.com/newrelic/infrastructure-agent.git /srv/newrelic/infrastructure-agent
WORKDIR /srv/newrelic/infrastructure-agent

RUN git checkout $TAG

CMD ["echo", "command is missing"]



