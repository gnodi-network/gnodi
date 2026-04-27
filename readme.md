# Gnodi

**Gnodi** is a Cosmos SDK blockchain with full EVM support via [cosmos/evm](https://github.com/cosmos/evm). It is compatible with MetaMask, Hardhat, Foundry, and any standard Ethereum tooling.

| Parameter | Value |
|---|---|
| Chain ID (Cosmos) | `gnodi-1` (mainnet) / `gnodi-test-1` (testnet) |
| EVM Chain ID | `46634` |
| Native denom | `uGNOD` (6 decimals) |
| EVM denom | `aGNOD` (18 decimals, via x/precisebank) |
| Address prefix | `gnodi` |
| BIP-44 coin type | `118` |
| RPC (CometBFT) | `http://localhost:26657` |
| gRPC | `localhost:9090` |
| EVM JSON-RPC | `http://localhost:8545` |
| EVM WebSocket | `ws://localhost:8546` |

---

## Requirements

- Go 1.24+
- Git
- C build toolchain (CGO is required — `cosmos/evm` links against libsecp256k1)

**macOS**
```bash
xcode-select --install
```

**Ubuntu / Debian**
```bash
sudo apt-get update && sudo apt-get install -y build-essential
```

**Fedora / RHEL / Amazon Linux**
```bash
sudo dnf groupinstall -y "Development Tools"
```

> **Note for Docker users**: If you are building inside a minimal container image (e.g. `golang:alpine`), you must install `gcc` and `musl-dev` (Alpine) or `build-essential` (Debian-based) before running `go build`. Setting `CGO_ENABLED=0` will not work — the build will fail with `undefined: secp256k1.RecoverPubkey`.

---

## Build

```bash
git clone https://github.com/gnodi-network/gnodi
cd gnodi
go build -o gnodid ./cmd/gnodid
```

Or use the Makefile to install the binary to `$GOPATH/bin`:

```bash
make install
```

---

## Local Testnet Setup

### 1. Initialize the node

```bash
./gnodid init <moniker> --chain-id gnodi-test-1
```

This creates `~/.gnodi/` with config files and a genesis that already includes correct denoms (`uGNOD`, `aGNOD`) and EVM module state.

### 2. Create a validator key

```bash
./gnodid keys add validator --keyring-backend test
```

Save the mnemonic shown. To recover the address later:

```bash
./gnodid keys show validator --keyring-backend test
```

### 3. Fund the validator in genesis

```bash
./gnodid genesis add-genesis-account validator 10000000000uGNOD --keyring-backend test
```

### 4. Create the genesis transaction

```bash
./gnodid genesis gentx validator 1000000000uGNOD \
  --chain-id gnodi-test-1 \
  --keyring-backend test \
  --moniker <moniker>
```

### 5. Collect gentxs and validate

```bash
./gnodid genesis collect-gentxs
./gnodid genesis validate
```

### 6. Configure the client

```bash
./gnodid config set client chain-id gnodi-test-1
```

### 7. Start the node

```bash
./gnodid start \
  --evm.evm-chain-id 46634 \
  --keyring-backend test \
  --minimum-gas-prices 0uGNOD \
  --json-rpc.enable
```

The node will begin producing blocks. You should see lines like:

```
INF committed state height=1
INF committed state height=2
...
```

---

## Verify EVM JSON-RPC

Once the node is running, test the EVM endpoints:

```bash
# Chain ID (should return 0xb62a = 46634)
curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}'

# Current block number
curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":2}'

# Network version (should return "46634")
curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"net_version","params":[],"id":3}'
```

---

## MetaMask Configuration

Add Gnodi as a custom network in MetaMask:

| Field | Value |
|---|---|
| Network name | Gnodi Testnet |
| RPC URL | `http://localhost:8545` |
| Chain ID | `46634` |
| Currency symbol | `GNOD` |

---

## Reset the Node

To wipe all chain data and start fresh:

```bash
rm -rf ~/.gnodi
```

Then repeat the setup steps above.

---

## Ports Reference

| Service | Port |
|---|---|
| CometBFT RPC | `26657` |
| CometBFT P2P | `26656` |
| gRPC | `9090` |
| gRPC-Web | `9900` |
| EVM JSON-RPC | `8545` |
| EVM WebSocket | `8546` |
| Cosmos REST API | `1317` (disabled by default) |
