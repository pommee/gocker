#!/bin/bash

echo "[1/2] Downloading gocker"
curl -L -o gocker https://github.com/pommee/gocker/raw/main/gocker

echo "[2/2] Installing"
sudo mv gocker /usr/bin/
sudo chmod +x /usr/bin/gocker

echo ""
echo "Installation complete. You can now use the 'gocker' command."
