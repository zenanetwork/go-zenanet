#!/bin/bash
# This is a postinstallation script so the service can be configured and started when requested
#
if [ -d "/var/lib/zena" ]
then
    echo "Directory /var/lib/zena exists."
else
    mkdir -p /var/lib/zena
    sudo chown -R zena /var/lib/zena
fi
sudo systemctl daemon-reload
