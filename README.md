Alephium Mining Sidecar
====

This project is a sidecar for Alephium mining nodes:

1. Creates a mining wallet if none exists. It can restore the wallet if mnemonics are provided.
2. Configures the mining to use the created wallet, if one created
3. Starts the mining automatically when the node is in sync
4. Transfers the mining reward on a fixed frequency (hourly, daily, ...) to another provided wallet

Sidecar means it needs to run alongside an Alephium full node, 
and have REST connectivity to it.

# Configuration

Docker-compose is a good way of running the side, assuming the
[Alephium node is also running in docker](https://touille.io/posts/how-to-run-alephium-full-node/).


```
version: "3"
services:
  mining-sidecar:
    image: touilleio/alephium-mining-sidecar:v1
    restart: unless-stopped
    security_opt:
      - no-new-privileges:true
    environment:
      - LOG_LEVEL=debug
      # provided via .env file, miningWalletName=...
      - WALLET_NAME=$miningWalletName 
      # miningWalletPassword=... in .env file
      - WALLET_PASSWORD=$miningWalletPassword
      # miningTransferAddress=... in .env file
      - TRANSFER_ADDRESS=$miningTransferAddress
    labels:
      - org.label-schema.group=alephium
```
