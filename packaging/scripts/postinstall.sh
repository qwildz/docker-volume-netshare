#!/bin/bash
set -e

# Create sysconfig directory if it doesn't exist
mkdir -p /etc/sysconfig

# Reload systemd to pick up new service file
if command -v systemctl &> /dev/null; then
    systemctl daemon-reload
    echo "Systemd service installed."
    echo ""
    echo "The service is configured to:"
    echo "  - Start after docker.service is ready"
    echo "  - Stop when docker.service stops (BindsTo)"
    echo "  - Automatically restart on failure"
    echo ""
    echo "To enable automatic startup on boot:"
    echo "  systemctl enable docker-volume-netshare"
    echo ""
    echo "To start the service now:"
    echo "  systemctl start docker-volume-netshare"
else
    echo "SysVinit script installed."
    echo "Enable with: update-rc.d docker-volume-netshare defaults"
    echo "Start with: /etc/init.d/docker-volume-netshare start"
fi

echo ""
echo "Configure the plugin by editing /etc/default/docker-volume-netshare"
echo "or /etc/sysconfig/docker-volume-netshare"
echo ""
echo "Note: /etc/default/ takes precedence over /etc/sysconfig/"
