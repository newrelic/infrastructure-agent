FROM opensuse/leap:15.3

# Run a system update so the system doesn't overwrite the fake systemctl later
RUN zypper -n update

RUN zypper -n install python3 sudo curl

# Adding fake systemctl
RUN curl https://raw.githubusercontent.com/gdraheim/docker-systemctl-replacement/master/files/docker/systemctl.py -o /usr/local/bin/systemctl

CMD ["/usr/local/bin/systemctl"]