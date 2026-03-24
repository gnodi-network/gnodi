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

			// Register uGNOD denom metadata in the bank module before RunMigrations.
			// Mainnet has no denom metadata (confirmed via REST API). The x/vm
			// InitGenesis calls InitEvmCoinInfo which looks up bank metadata for
			// params.EvmDenom ("uGNOD") — if missing it panics. We must register it
			// here before RunMigrations triggers InitGenesis for the new EVM modules.
			// NOTE: do NOT call EVMKeeper.InitEvmCoinInfo here — the x/vm params store
			// has not been initialized yet (InitGenesis hasn't run), so GetParams()
			// returns empty. Let x/vm.InitGenesis handle it via RunMigrations.
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

			// RunMigrations calls InitGenesis for new modules (x/vm, x/feemarket,
			// x/erc20, x/precisebank) and migrations for existing modules.
			// x/vm.InitGenesis will call InitEvmCoinInfo, which now finds the metadata.
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
