#!/bin/bash
set -e

# Stop the service before removal
if command -v systemctl &> /dev/null; then
    if systemctl is-active --quiet docker-volume-netshare 2>/dev/null; then
        systemctl stop docker-volume-netshare || true
    fi
    if systemctl is-enabled --quiet docker-volume-netshare 2>/dev/null; then
        systemctl disable docker-volume-netshare || true
    fi
else
    if [ -x /etc/init.d/docker-volume-netshare ]; then
        /etc/init.d/docker-volume-netshare stop || true
        update-rc.d -f docker-volume-netshare remove || true
    fi
fi
