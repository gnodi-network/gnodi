package app

import (
	"encoding/json"

	sdkmath "cosmossdk.io/math"

	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// GenesisState of the blockchain is represented here as a map of raw json
// messages key'd by an identifier string.
// The identifier is used to determine which module genesis information belongs
// to so it may be appropriately routed during init chain.
// Within this application default genesis information is retrieved from
// the ModuleBasicManager which populates json from each BasicModule
// object provided to it during init.
type GenesisState map[string]json.RawMessage

// activePrecompiles lists the static precompiles enabled on Gnodi.
// The ICS20 precompile (0x...0802) is intentionally excluded: cosmos/evm v0.5.1
// contains GHSA-54gx-3cgr-7mfm, a critical reentrancy vulnerability in the ICS20
// precompile that was exploited on Saga chain (January 2026). The fix ships in
// cosmos/evm v0.6.0, which has breaking API changes requiring a separate migration.
// Until that upgrade is performed, the ICS20 precompile must remain disabled.
var activePrecompiles = []string{
	evmtypes.P256PrecompileAddress,
	evmtypes.Bech32PrecompileAddress,
	evmtypes.StakingPrecompileAddress,
	evmtypes.DistributionPrecompileAddress,
	evmtypes.VestingPrecompileAddress,
	evmtypes.BankPrecompileAddress,
	evmtypes.GovPrecompileAddress,
	evmtypes.SlashingPrecompileAddress,
}

// NewEVMGenesisState returns the default genesis state for the x/vm module.
func NewEVMGenesisState() *evmtypes.GenesisState {
	evmGenState := evmtypes.DefaultGenesisState()
	// For 6-decimal chains: EvmDenom is the native denom (uGNOD), not the extended 18-decimal
	// denom (aGNOD). InitEvmCoinInfo looks up bank metadata using EvmDenom, and bank metadata
	// is registered for the native denom. ExtendedDenomOptions carries aGNOD for PreciseBank.
	evmGenState.Params.EvmDenom = evmtypes.GetEVMCoinDenom()
	evmGenState.Params.ExtendedDenomOptions = &evmtypes.ExtendedDenomOptions{
		ExtendedDenom: evmtypes.GetEVMCoinExtendedDenom(),
	}
	evmGenState.Params.ActiveStaticPrecompiles = activePrecompiles
	evmGenState.Preinstalls = evmtypes.DefaultPreinstalls
	return evmGenState
}

// NewErc20GenesisState returns the default genesis state for the x/erc20 module.
func NewErc20GenesisState() *erc20types.GenesisState {
	return erc20types.DefaultGenesisState()
}

// NewFeeMarketGenesisState returns the genesis state for the x/feemarket module.
// NoBaseFee=true disables the EIP-1559 dynamic base fee mechanism so that gas
// prices remain stable and predictable. BaseFee is explicitly zeroed since it is
// not enforced when NoBaseFee=true.
//
// MinGasPrice is set to 1 Gwei (1_000_000_000 in 18-decimal Wei units) to establish
// a consensus-level minimum fee floor for all EVM transactions. This is the value
// enforced by CheckGlobalFee in the EVM ante handler and is the natural floor that
// MetaMask and other EVM wallets already target. Without this, CheckGlobalFee
// short-circuits on zero and EVM txs can be submitted fee-free.
func NewFeeMarketGenesisState() *feemarkettypes.GenesisState {
	feeMarketGenState := feemarkettypes.DefaultGenesisState()
	feeMarketGenState.Params.NoBaseFee = true
	feeMarketGenState.Params.BaseFee = sdkmath.LegacyZeroDec()
	feeMarketGenState.Params.MinGasPrice = sdkmath.LegacyNewDec(1_000_000_000)
	return feeMarketGenState
}
