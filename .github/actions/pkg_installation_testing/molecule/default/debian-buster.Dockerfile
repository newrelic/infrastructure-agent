FROM debian:buster

RUN apt-get update \
    && apt-get install -y init gpg ca-certificates sudo curl python3 \
    && apt-get clean all

# Adding fake systemctl
RUN curl https://raw.githubusercontent.com/gdraheim/docker-systemctl-replacement/master/files/docker/systemctl.py -o /usr/local/bin/systemctl

CMD ["/usr/local/bin/systemctl"]