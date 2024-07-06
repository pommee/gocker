#!/bin/bash

echo "Downloading gocker from https://github.com/pommee/gocker/raw/main/gocker"
curl -L -o gocker https://github.com/pommee/gocker/raw/main/gocker

sudo mv gocker /usr/bin/
sudo chmod +x /usr/bin/gocker

echo ""
echo "Installation complete. You can now use the 'gocker' command from /usr/bin."
echo "For simplicity, add an alias."
echo "zsh Example: echo alias gocker=\"/usr/bin/gocker\" >> ~/.zshrc"
