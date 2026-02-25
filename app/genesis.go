package app

import (
	"encoding/json"

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

// NewEVMGenesisState returns the default genesis state for the x/vm module.
// All available static precompiles are enabled by default so Gnodi EVM contracts
// can call staking, distribution, governance, etc. via precompile addresses.
func NewEVMGenesisState() *evmtypes.GenesisState {
	evmGenState := evmtypes.DefaultGenesisState()
	// For 6-decimal chains: EvmDenom is the native denom (uGNOD), not the extended 18-decimal
	// denom (aGNOD). InitEvmCoinInfo looks up bank metadata using EvmDenom, and bank metadata
	// is registered for the native denom. ExtendedDenomOptions carries aGNOD for PreciseBank.
	evmGenState.Params.EvmDenom = evmtypes.GetEVMCoinDenom()
	evmGenState.Params.ExtendedDenomOptions = &evmtypes.ExtendedDenomOptions{
		ExtendedDenom: evmtypes.GetEVMCoinExtendedDenom(),
	}
	evmGenState.Params.ActiveStaticPrecompiles = evmtypes.AvailableStaticPrecompiles
	evmGenState.Preinstalls = evmtypes.DefaultPreinstalls
	return evmGenState
}

// NewErc20GenesisState returns the default genesis state for the x/erc20 module.
func NewErc20GenesisState() *erc20types.GenesisState {
	return erc20types.DefaultGenesisState()
}

// NewFeeMarketGenesisState returns the default genesis state for the x/feemarket module.
// NoBaseFee is set to true so the chain starts without a dynamic base fee, matching
// the original Gnodi fee model.
func NewFeeMarketGenesisState() *feemarkettypes.GenesisState {
	feeMarketGenState := feemarkettypes.DefaultGenesisState()
	feeMarketGenState.Params.NoBaseFee = true
	return feeMarketGenState
}
