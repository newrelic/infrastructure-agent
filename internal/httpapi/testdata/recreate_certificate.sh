#!/usr/bin/env bash

# The generated certificate expires in 2 years and it seems that is not configurable
# execute the following commands to regenerate it
brew install mkcert nss
sudo mkcert -install
mkcert -key-file localhost-key.pem -cert-file localhost.pem "localhost"
mkcert -key-file client-client-key.pem -cert-file client-client.pem -client "client"
cp /Users/$USER/Library/Application\ Support/mkcert/rootCA.pem .
sudo mkcert -uninstall
