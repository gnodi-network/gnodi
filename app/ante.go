package app

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/cosmos-sdk/client"

	evmante "github.com/cosmos/evm/ante"
	antetypes "github.com/cosmos/evm/ante/types"
)

// setAnteHandler configures the EVM-aware ante handler and registers it on the BaseApp.
// This must be called after all keepers are initialized.
func (app *App) setAnteHandler(txConfig client.TxConfig, maxGasWanted uint64) {
	options := evmante.HandlerOptions{
		Cdc:                    app.appCodec,
		AccountKeeper:          app.AccountKeeper,
		BankKeeper:             app.BankKeeper,
		ExtensionOptionChecker: antetypes.HasDynamicFeeExtensionOption,
		EvmKeeper:              app.EVMKeeper,
		FeegrantKeeper:         app.FeeGrantKeeper,
		IBCKeeper:              app.IBCKeeper,
		FeeMarketKeeper:        app.FeeMarketKeeper,
		SignModeHandler:        txConfig.SignModeHandler(),
		SigGasConsumer:         evmante.SigVerificationGasConsumer,
		MaxTxGasWanted:         maxGasWanted,
		DynamicFeeChecker:      true,
		PendingTxListener:      app.onPendingTx,
	}
	if err := options.Validate(); err != nil {
		panic(err)
	}
	app.SetAnteHandler(evmante.NewAnteHandler(options))
}

// onPendingTx forwards a pending Ethereum tx hash to all registered listeners.
// This is used as the PendingTxListener in the ante handler options so the
// EVM JSON-RPC server can stream pending transactions.
func (app *App) onPendingTx(hash common.Hash) {
	for _, listener := range app.pendingTxListeners {
		listener(hash)
	}
}
