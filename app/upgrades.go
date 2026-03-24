package app

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
)

const (
	// EVMUpgradeName is the on-chain upgrade name for the EVM integration upgrade.
	// This upgrade adds x/vm, x/feemarket, x/erc20, and x/precisebank modules.
	EVMUpgradeName = "evm-upgrade"
)

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

			// 3. RunMigrations handles existing module migrations and InitGenesis
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
