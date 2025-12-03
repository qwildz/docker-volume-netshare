#!/bin/bash
set -e

# Create sysconfig directory if it doesn't exist
mkdir -p /etc/sysconfig

# Reload systemd to pick up new service file
if command -v systemctl &> /dev/null; then
    systemctl daemon-reload
    echo "Systemd service installed. Enable with: systemctl enable docker-volume-netshare"
    echo "Start with: systemctl start docker-volume-netshare"
else
    echo "SysVinit script installed. Enable with: update-rc.d docker-volume-netshare defaults"
    echo "Start with: /etc/init.d/docker-volume-netshare start"
fi

echo ""
echo "Configure the plugin by editing /etc/default/docker-volume-netshare"
echo "or /etc/sysconfig/docker-volume-netshare"
