FROM opensuse/leap:15.3

# Run a system update so the system doesn't overwrite the fake systemctl later
RUN zyp -y update

RUN zyp -y install python3 sudo

# Adding fake systemctl
RUN curl https://raw.githubusercontent.com/gdraheim/docker-systemctl-replacement/master/files/docker/systemctl.py -o /usr/local/bin/systemctl

