Alephium Mining Companion
====

This project is a companion for Alephium full nodes, providing some convenience features
when enabling mining:

1. Creates a mining wallet if none exists. 
   It can also restore the wallet if mnemonics are provided.
2. Configures the mining process to use the created wallet, 
   if one created or different from the one configured.
3. Starts the mining automatically when the node is in sync
4. Transfers the mining reward to another provided wallet/address
   on a fixed frequency (hourly, daily, ...). Note that the wallet
   can be created anywhere as long as it's a valid Alephium address
5. Expose metrics for both the mining and the transfer wallets

Companion means it needs to run alongside an Alephium full node,
and particularly have REST connectivity to it (default port 12973)

![mining-sidecar](alephium-mining.png)

# Configuration

## Docker compose

Docker-compose is a good way of running the side, assuming the
[Alephium node is also running in docker](https://touille.io/posts/how-to-run-alephium-full-node/).

```
version: "3"
services:
  mining-companion:
    image: touilleio/alephium-mining-companion:v6
    restart: unless-stopped
    security_opt:
      - no-new-privileges:true
    environment:
      - ALEPHIUM_ENDPOINT=http://broker:12973
      - LOG_LEVEL=debug
      - WALLET_NAME=$miningWalletName
      - WALLET_PASSWORD=$miningWalletPassword
      - WALLET_MNEMONIC=$miningWalletMnemonic
      - TRANSFER_ADDRESS=$miningTransferAddress
      - TRANSFER_FREQUENCY=5m
      - TRANSFER_MIN_AMOUNT=50000000000000000000
      - IMMEDIATE_TRANSFER=true
      - START_MINING=false
```
with variables `miningWalletName`, `miningWalletPassword`, `miningWalletMnemonic` and `miningTransferAddress`
in a `.env` file in the same folder as the `docker-compose.yml`.

| Variable | Default | Description |
|----------|---------|-------------|
| `ALEPHIUM_ENDPOINT` | `http://alephium:12973` | REST URI of your Alephium node. Mind localhost in a docker container point to the docker container, not the host itself. |
| `ALEPHIUM_API_KEY` | _optional_ | API key to use to connect to `ALEPHIUM_ENDPOINT`. |
| `WALLET_NAME` | `mining-companion-wallet-1` | Name of the miner wallet |
| `WALLET_PASSWORD` | `Default-Password-1234` | Password to unlock the miner wallet |
| `WALLET_MNEMONIC` | _optional_ | Mnemonic to restore (create) the wallet if it does not exist. Random mnemonic will be generated if not set |
| `WALLET_MNEMONIC_PASSPHRASE` | _optional_ | A passphrase associated with the mnemonic, if any |
| `TRANSFER_MIN_AMOUNT` | 20000000000000000000 (20 ALF) | Min amount to transfer at once. It uses the `sweepAll` function to optimize the transaction. |
| `TRANSFER_ADDRESS` | _optional_ | Address to transfer the mining rewards to. If none provided, no transfer is performed. Double check you're sending the funds to the right address !! |
| `TRANSFER_FREQUENCY` | `15m` | Frequency at which funds are transferred |
| `PRINT_MNEMONIC` | `true` | If true and a wallet is created without pre-set mnemonic (`WALLET_MNEMONIC` option above), the randomly generate mnemonic is printed out. This is a sensitive information, use it with caution! |
| `IMMEDIATE_TRANSFER` | `false` | If set to true, a transfer is sent at the start of the container, without waiting for `TRANSFER_FREQUENCY` initial time |
| `START_MINING` | `false` | If set to true, the mining machinery built-in the broker will start mining. This is disabled by default and the dedicated, more efficient [CPU miner](https://github.com/alephium/cpu-miner) is recommended for mining as the time of writing |

## Docker

Replace `123456789012345678901234567890123456789012345` below with your own wallet address!

```
docker run -it --rm --link alephium:alephium -e TRANSFER_ADDRESS=123456789012345678901234567890123456789012345 touilleio/alephium-mining-companion:v6
```

As a reminder, running a Alephium full node looks like the following:

```
docker run -it --rm --name alephium -p 12973:12973 alephium/alephium:v1.1.7
```
