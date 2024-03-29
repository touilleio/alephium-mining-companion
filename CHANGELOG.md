Alephium Mining Companion Changelog
====

# Version v7.1.2

- Bump alephium/go-sdk to v1.6.3

# Version v7.1.1

- Bump alephium/go-sdk to v1.6.2

# Version v7.1.0

- Bump alephium/go-sdk to v1.6

# Version v7.0.0

- Migrate to github.com/alephium/go-sdk
- Use sweepAll function

# Version v6.1.0

## Improvements

- Add support for API key via `ALEPHIUM_API_KEY` environment variable
- Remove no longer needed `START_MINING` environment variable and feature

# Version v6.0.0

## Breaking behaviour changes

- Use `SweepAll` function instead of Transfer, once the `TRANSFER_MIN_AMOUNT` threshold is met per address
- `TRANSFER_MAX_AMOUNT` is no longer used.
- `TRANSFER_MIN_AMOUNT` default changed to 20 ALPH

## Improvements

- Use `alephium.GetAddressesAsString`

# Version v5.1.1

## Improvement changes

- Add markers around the mnemonic to make it more visible

# Version v5.1.0

## Improvement changes

- Add metrics to monitor balance of miner and transfer addresses

# Version v5.0.0

## Breaking changes

- Follow up changes in v1.0.0 (mainnet)

# Version v4.0.0

## Breaking changes

- Follow up changes in v0.11.0

# Version v3.0.0

## Behavior changes

- START_MINING has be set to `false` by default, meaning the mining-companion is setting the mining addresses,
  but does not start the full node mining, since this feature is deprecated in favor of the 
  [CPU miner](https://github.com/alephium/cpu-miner)

# Version v2.1.0

## Improvements

- Expose metrics of number of amount transferred

# Version v2.0.0

- Rename mining-sidecar to mining-companion

# Version v1.5.1

## Improvements

- Follow upstream update in alephium-go-client to simplify code

# Version v1.5.0

## Improvements

- Adds `START_MINING` option, telling whether to start local mining. Useful when running external miner (brought by v0.8.5)

# Version v1.4.0

## Improvements

- Follow up API name change of v0.8.2

# Version v1.3.0

## Fix

- Followup change in coinbase lockup time

# Version v1.2.1

## Fix

- Followup change in miner addresses behaviour

# Version v1.2.0

## Improvements

- Add `TRANSFER_MIN_AMOUNT` configuration option
- Bump alephium-go-client dependency and refactor to use AFL manipulation module

# Version v1.1.0

## Improvements

- Add `IMMEDIATE_TRANSFER` configuration option
- Bump alephium-go-client dependency
- Enhance logging messages

# Version v1.0.0

- Initial version
