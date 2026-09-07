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
//
// The ICS20 precompile (0x...0802) remains excluded. It was originally disabled as a
// stopgap against GHSA-54gx-3cgr-7mfm / ASA-2026-002, the critical ICS20 reentrancy
// bug in cosmos/evm <v0.6.0 that was exploited on Saga chain (January 2026). That fix
// is now in place: this chain runs cosmos/evm v0.6.3, so the vulnerability is patched
// and enabling ICS20 would no longer be unsafe on that account.
//
// It is left disabled deliberately rather than by necessity. Note that as of v0.6.0
// the ICS20 precompile is the ONLY path for ERC-20 IBC transfers — the x/ibc/transfer
// wrapper that used to convert ERC-20 on the ICS-20 message path was removed upstream.
// Enabling it is therefore a product decision (it turns on ERC-20 IBC transfers), and
// on a live chain it is a governance param change, not a genesis edit.
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
// MinGasPrice is set to zero. On a 6-decimal chain (uGNOD), the MinGasPrice param
// is applied directly in uGNOD/gas by MinGasPriceDecorator with no 18-decimal
// conversion, so any non-zero value causes Cosmos txs to require enormous fees.
// EVM-specific fee enforcement is handled by a custom ante decorator.
func NewFeeMarketGenesisState() *feemarkettypes.GenesisState {
	feeMarketGenState := feemarkettypes.DefaultGenesisState()
	feeMarketGenState.Params.NoBaseFee = true
	feeMarketGenState.Params.BaseFee = sdkmath.LegacyZeroDec()
	feeMarketGenState.Params.MinGasPrice = sdkmath.LegacyZeroDec()
	return feeMarketGenState
}
