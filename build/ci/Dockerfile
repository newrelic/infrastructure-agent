FROM ubuntu:22.04 as ci

ARG TAG_ARG=1.27.0
ARG SSH_KEY_ARG="not a key"

ENV SSH_KEY=$SSH_KEY_ARG
ENV TAG=$TAG_ARG

RUN apt-get update
RUN DEBIAN_FRONTEND=noninteractive apt install -y make ansible wget unzip git gcc

RUN wget "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip"  && unzip awscli-exe-linux-x86_64.zip && ./aws/install

RUN wget https://go.dev/dl/go1.18.3.linux-amd64.tar.gz && rm -rf /usr/local/go && tar -C /usr/local -xzf go1.18.3.linux-amd64.tar.gz
ENV PATH=$PATH:/usr/local/go/bin

RUN mkdir -p /srv/newrelic

RUN git clone https://github.com/newrelic/infrastructure-agent.git /srv/newrelic/infrastructure-agent
WORKDIR /srv/newrelic/infrastructure-agent

RUN git checkout $TAG

RUN mkdir ~/.ssh/

COPY ./entrypoint.sh /srv/newrelic/infrastructure-agent/entrypoint.sh

RUN chmod +x /srv/newrelic/infrastructure-agent/entrypoint.sh

ENTRYPOINT ["./entrypoint.sh"]




