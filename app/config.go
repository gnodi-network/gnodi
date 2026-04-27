package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// EVMChainID is the EIP-155 replay-protection chain ID for Gnodi EVM.
	EVMChainID = 46634
)

func init() {
	// Set prefixes
	accountPubKeyPrefix := AccountAddressPrefix + "pub"
	validatorAddressPrefix := AccountAddressPrefix + "valoper"
	validatorPubKeyPrefix := AccountAddressPrefix + "valoperpub"
	consNodeAddressPrefix := AccountAddressPrefix + "valcons"
	consNodePubKeyPrefix := AccountAddressPrefix + "valconspub"

	// Set and seal config
	config := sdk.GetConfig()
	config.SetCoinType(ChainCoinType)
	config.SetBech32PrefixForAccount(AccountAddressPrefix, accountPubKeyPrefix)
	config.SetBech32PrefixForValidator(validatorAddressPrefix, validatorPubKeyPrefix)
	config.SetBech32PrefixForConsensusNode(consNodeAddressPrefix, consNodePubKeyPrefix)
	config.Seal()

	// Set the SDK default bond denom so all module default genesis states
	// (staking, mint, gov) use uGNOD instead of "stake".
	// NOTE: Do NOT call EVMConfigurator.Configure() here. The EVM coin info is
	// set exactly once during InitGenesis via SetGlobalConfigVariables. On restarts,
	// the keeper's WithDefaultEvmCoinInfo provides the fallback.
	sdk.DefaultBondDenom = "uGNOD"
}
