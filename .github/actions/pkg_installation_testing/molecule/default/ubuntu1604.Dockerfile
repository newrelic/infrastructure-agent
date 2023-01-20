FROM ubuntu:16.04

RUN apt-get update \
    && apt-get install -y init ca-certificates sudo curl python3 apt-transport-https\
    && apt-get clean all

# Adding fake systemctl
RUN curl https://raw.githubusercontent.com/gdraheim/docker-systemctl-replacement/master/files/docker/systemctl.py -o /usr/local/bin/systemctl

CMD ["/usr/local/bin/systemctl"]