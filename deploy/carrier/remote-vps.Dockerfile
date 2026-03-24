FROM node:22-bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git \
    jq \
    openssh-server \
    rsync \
    sudo \
    tini \
  && rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash carrier \
  && echo "carrier ALL=(ALL) NOPASSWD:ALL" >/etc/sudoers.d/carrier \
  && chmod 0440 /etc/sudoers.d/carrier

RUN mkdir -p /var/run/sshd /home/carrier/.ssh /home/carrier/.npm-global/bin /home/carrier/.npm-global/lib \
  && chown -R carrier:carrier /home/carrier/.ssh /home/carrier/.npm-global \
  && chmod 700 /home/carrier/.ssh \
  && ssh-keygen -A

COPY deploy/carrier/remote-vps-entrypoint.sh /usr/local/bin/entrypoint.sh
COPY deploy/carrier/setup-remote-vps-home.sh /usr/local/bin/setup-remote-vps-home.sh
RUN chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/setup-remote-vps-home.sh

EXPOSE 22

ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/entrypoint.sh"]
