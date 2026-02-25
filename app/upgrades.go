package app

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

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

			// Initialize EvmCoinInfo in the EVM module store.
			// This is required for NON-18 denom chains (Gnodi uses uGNOD at 6 decimals).
			if err := app.EVMKeeper.InitEvmCoinInfo(sdkCtx); err != nil {
				return nil, err
			}

			// RunMigrations calls InitGenesis for any newly added modules
			// and runs existing module migrations. New store keys must already
			// be registered via SetStoreLoader (see below).
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
