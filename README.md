# LND whitelist
A RPC daemon for LND that listens in the background and allows (whitelist) or denies (blacklist) incoming channels from a list of node public keys.

## Install

### From source
Build from source (you may need to install go for this):

```bash
git clone https://github.com/callebtc/electronwall.git
cd electronwall
go build .
```

### Binaries

You can download a binary for your system [here](https://github.com/callebtc/electronwall/releases).

## Config
Edit `config.yaml.example` and rename to `config.yaml`.

## Run

```bash
./electronwall
```