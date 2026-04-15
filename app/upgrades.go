package app

import (
	"context"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/group"

	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
)

const (
	// EVMUpgradeName is the on-chain upgrade name for the EVM integration upgrade.
	// This upgrade adds x/vm, x/feemarket, x/erc20, and x/precisebank modules,
	// and disables x/group via the circuit breaker.
	EVMUpgradeName = "evm-upgrade"
)

// groupMsgTypeURLs lists all x/group message type URLs to be disabled via the
// circuit breaker. x/group is deprecated upstream and has zero usage on Gnodi.
var groupMsgTypeURLs = []string{
	sdk.MsgTypeURL(&group.MsgCreateGroup{}),
	sdk.MsgTypeURL(&group.MsgCreateGroupWithPolicy{}),
	sdk.MsgTypeURL(&group.MsgUpdateGroupMembers{}),
	sdk.MsgTypeURL(&group.MsgUpdateGroupAdmin{}),
	sdk.MsgTypeURL(&group.MsgUpdateGroupMetadata{}),
	sdk.MsgTypeURL(&group.MsgCreateGroupPolicy{}),
	sdk.MsgTypeURL(&group.MsgUpdateGroupPolicyAdmin{}),
	sdk.MsgTypeURL(&group.MsgUpdateGroupPolicyDecisionPolicy{}),
	sdk.MsgTypeURL(&group.MsgUpdateGroupPolicyMetadata{}),
	sdk.MsgTypeURL(&group.MsgSubmitProposal{}),
	sdk.MsgTypeURL(&group.MsgWithdrawProposal{}),
	sdk.MsgTypeURL(&group.MsgVote{}),
	sdk.MsgTypeURL(&group.MsgExec{}),
	sdk.MsgTypeURL(&group.MsgLeaveGroup{}),
}

// RegisterUpgradeHandlers registers the upgrade handlers for the app.
func (app *App) RegisterUpgradeHandlers() {
	app.UpgradeKeeper.SetUpgradeHandler(
		EVMUpgradeName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			cdc := app.appCodec

			// 1. Register uGNOD denom metadata in the bank module.
			// Mainnet has no denom metadata. x/vm InitGenesis calls InitEvmCoinInfo
			// which looks up bank metadata for params.EvmDenom — must exist first.
			app.BankKeeper.SetDenomMetaData(sdkCtx, banktypes.Metadata{
				Description: "Gnodi native token",
				Base:        "uGNOD",
				DenomUnits: []*banktypes.DenomUnit{
					{Denom: "uGNOD", Exponent: 0},
					{Denom: "GNOD", Exponent: 6},
				},
				Name:    "Gnodi",
				Symbol:  "GNOD",
				Display: "GNOD",
			})

			// 2. Manually initialize x/vm and x/feemarket with Gnodi-specific genesis.
			// RunMigrations calls module.DefaultGenesis() for new modules, but
			// x/vm.DefaultParams() hardcodes EvmDenom="aatom" regardless of our config.
			// We must init these modules ourselves and mark them in fromVM so
			// RunMigrations skips their InitGenesis.
			// x/erc20 and x/precisebank use correct defaults and are left to RunMigrations.
			evmMod := app.ModuleManager.Modules[evmtypes.ModuleName].(module.HasABCIGenesis)
			evmMod.InitGenesis(sdkCtx, cdc, cdc.MustMarshalJSON(NewEVMGenesisState()))
			fromVM[evmtypes.ModuleName] = 1

			feeMarketMod := app.ModuleManager.Modules[feemarkettypes.ModuleName].(module.HasABCIGenesis)
			feeMarketMod.InitGenesis(sdkCtx, cdc, cdc.MustMarshalJSON(NewFeeMarketGenesisState()))
			fromVM[feemarkettypes.ModuleName] = 1

			// 2b. Set MinGasPrice = 1 Gwei on the feemarket params.
			// NewFeeMarketGenesisState() already includes MinGasPrice=1Gwei, but we
			// explicitly set it here via the keeper so the value is persisted in state
			// independently of InitGenesis ordering. This also makes the intent clear
			// for future upgrades.
			feeMarketParams := app.FeeMarketKeeper.GetParams(sdkCtx)
			feeMarketParams.MinGasPrice = sdkmath.LegacyNewDec(1_000_000_000)
			if err := app.FeeMarketKeeper.SetParams(sdkCtx, feeMarketParams); err != nil {
				return nil, err
			}

			// 3. Disable x/group via the circuit breaker.
			// x/group is deprecated and unused on Gnodi. Its MsgExec execution path
			// calls message handlers directly, bypassing all ante middleware. Disabling
			// at the circuit level provides defense in depth alongside the
			// groupLimiterDecorator in the ante handler.
			for _, typeURL := range groupMsgTypeURLs {
				if err := app.CircuitBreakerKeeper.DisableList.Set(ctx, typeURL); err != nil {
					return nil, err
				}
			}

			// 4. RunMigrations handles existing module migrations and InitGenesis
			// for x/erc20 and x/precisebank (their defaults are correct).
			return app.ModuleManager.RunMigrations(ctx, app.Configurator(), fromVM)
		},
	)

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(err)
	}

	if upgradeInfo.Name == EVMUpgradeName && !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		storeUpgrades := storetypes.StoreUpgrades{
			Added: []string{
				evmtypes.StoreKey,
				feemarkettypes.StoreKey,
				erc20types.StoreKey,
				precisebanktypes.StoreKey,
			},
		}
		// configure the store loader to add new stores at upgrade height
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
}
