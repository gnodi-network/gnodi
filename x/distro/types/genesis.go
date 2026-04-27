package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultGenesis returns the default genesis state.
// MintingAddress and ReceivingAddress are empty by default because they are
// deployment-specific values that must be set by the chain operator before
// minting operations can proceed. Use Params.Validate() to enforce their
// presence at runtime (e.g. in MsgUpdateParams).
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// Validate performs genesis state validation.
// Addresses are allowed to be empty in genesis — they are deployment-specific
// and must be configured before minting. If provided, they must be valid bech32.
// Use Params.Validate() for strict runtime enforcement.
func (gs GenesisState) Validate() error {
	p := gs.Params
	if p.MintingAddress != "" {
		if _, err := sdk.AccAddressFromBech32(p.MintingAddress); err != nil {
			return fmt.Errorf("invalid minting address: %w", err)
		}
	}
	if p.ReceivingAddress != "" {
		if _, err := sdk.AccAddressFromBech32(p.ReceivingAddress); err != nil {
			return fmt.Errorf("invalid receiving address: %w", err)
		}
	}
	if err := validateDenom(p.Denom); err != nil {
		return err
	}
	if err := validateDistributionStartDate(p.DistributionStartDate); err != nil {
		return err
	}
	if err := validateMonthsInHalvingPeriod(p.MonthsInHalvingPeriod); err != nil {
		return err
	}
	return nil
}
