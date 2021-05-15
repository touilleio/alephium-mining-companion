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

| Variable | Default | Description |
|----------|---------|-------------|
| `ALEPHIUM_ENDPOINT` | `http://alephium:12973` | REST URI of your Alephium node. Mind localhost in a docker container point to the docker container, not the host itself. |
| `WALLET_NAME` | | Name of the miner wallet |
| `WALLET_PASSWORD` | | Password to unlock the miner wallet |
| `WALLET_MNEMONIC` | | (optional) Mnemonic to restore (create) the wallet if it does not exist. Random mnemonic will be generated if not set |
| `WALLET_MNEMONIC_PASSPHRASE` | | (optional) A passphrase associated with the mnemonic, if any |
| `TRANSFER_MAX_AMOUNT` | 50000000000000000000 (50 ALP) | Max amount to transfer at once.
| `TRANSFER_ADDRESS` | | Address to transfer the mining rewards to. Double check you're sending the funds to the right address !! |
| `TRANSFER_FREQUENCY` | `15m` | Frequency at which funds are transferred |
| `PRINT_MNEMONIC` | `true` | If true and a wallet is created without pre-set mnemonic (`WALLET_MNEMONIC` option above), the randomly generate mnemonic is printed out. This is a sensitive information, use it with caution! |
