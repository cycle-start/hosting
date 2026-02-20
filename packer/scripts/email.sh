#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

# Install Stalwart mail server from official release.
STALWART_VERSION="0.11.8"
STALWART_URL="https://github.com/stalwartlabs/mail-server/releases/download/v${STALWART_VERSION}/stalwart-mail-x86_64-unknown-linux-gnu.tar.gz"

mkdir -p /opt/stalwart-mail/bin /opt/stalwart-mail/etc /opt/stalwart-mail/data
curl -fsSL "$STALWART_URL" | tar -xz -C /opt/stalwart-mail/bin
chmod +x /opt/stalwart-mail/bin/stalwart-mail

# Create stalwart system user.
useradd --system --home /opt/stalwart-mail --shell /usr/sbin/nologin stalwart-mail
chown -R stalwart-mail:stalwart-mail /opt/stalwart-mail

# Create systemd unit.
cat > /etc/systemd/system/stalwart-mail.service << 'EOF'
[Unit]
Description=Stalwart Mail Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=stalwart-mail
Group=stalwart-mail
ExecStart=/opt/stalwart-mail/bin/stalwart-mail --config /opt/stalwart-mail/etc/config.toml
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl disable stalwart-mail  # Enabled via cloud-init after config is written.

# Vector role-specific config.
cp /tmp/vector-email.toml /etc/vector/config.d/email.toml

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*

# Final cloud-init clean — must be last to prevent package triggers from
# recreating /var/lib/cloud state.
cloud-init clean
