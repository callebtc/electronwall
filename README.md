# LND whitelist
A RPC daemon for LND that listens in the background and allows (whitelist) or denies (blacklist) incoming channels from a list of node public keys.

## Install

```bash
git clone https://github.com/callebtc/electronwall.git
cd electronwall
go build .
```

## Config
Edit `config.yaml.example` and rename to `config.yaml`.

## Run

```bash
./electronwall
```