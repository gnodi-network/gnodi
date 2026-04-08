package app

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/group"
	"github.com/ethereum/go-ethereum/common"

	evmante "github.com/cosmos/evm/ante"
	antetypes "github.com/cosmos/evm/ante/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// maxGroupNestedMsgs caps recursive inspection depth inside group.MsgSubmitProposal.
const maxGroupNestedMsgs = 7

// groupLimiterDecorator blocks disallowed message types from being embedded
// inside x/group proposals. x/group's MsgExec replays stored proposal messages
// by calling handlers directly, bypassing the ante handler chain entirely.
// Blocking at MsgSubmitProposal time prevents disallowed msgs from reaching state.
type groupLimiterDecorator struct {
	disabledMsgTypes []string
}

func newGroupLimiterDecorator(disabledMsgTypes ...string) groupLimiterDecorator {
	return groupLimiterDecorator{disabledMsgTypes: disabledMsgTypes}
}

func (g groupLimiterDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	for _, msg := range tx.GetMsgs() {
		if err := g.checkMsg(msg, 1); err != nil {
			return ctx, errorsmod.Wrapf(errortypes.ErrUnauthorized, "%s", err.Error())
		}
	}
	return next(ctx, tx, simulate)
}

func (g groupLimiterDecorator) checkMsg(msg sdk.Msg, depth int) error {
	if depth >= maxGroupNestedMsgs {
		return fmt.Errorf("exceeded max nested message depth in group proposal")
	}
	// Check the message itself.
	if g.isDisabled(sdk.MsgTypeURL(msg)) {
		return fmt.Errorf("found disabled msg type: %s", sdk.MsgTypeURL(msg))
	}
	// Recursively inspect messages embedded in a group proposal.
	if proposal, ok := msg.(*group.MsgSubmitProposal); ok {
		innerMsgs, err := proposal.GetMsgs()
		if err != nil {
			return err
		}
		for _, inner := range innerMsgs {
			if err := g.checkMsg(inner, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g groupLimiterDecorator) isDisabled(typeURL string) bool {
	for _, disabled := range g.disabledMsgTypes {
		if typeURL == disabled {
			return true
		}
	}
	return false
}

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
		MaxTxGasWanted: maxGasWanted,
		// DynamicFeeChecker is disabled because NoBaseFee=true in feemarket params.
		// The DynamicFeeChecker reads feemarketParams.BaseFee but never branches on
		// NoBaseFee, so enabling it with a non-zero BaseFee would enforce a fee floor
		// that contradicts the declared intent. Cosmos txs instead use the standard
		// DeductFeeDecorator against the node's --minimum-gas-prices config.
		// EVM txs are routed through newEVMAnteHandler and are unaffected by this flag.
		DynamicFeeChecker: false,
		PendingTxListener: app.onPendingTx,
	}
	if err := options.Validate(); err != nil {
		panic(err)
	}

	// Prepend the group limiter to block MsgEthereumTx from being embedded inside
	// x/group proposals. x/group's execution path bypasses ante middleware, so we
	// must reject disallowed msgs at MsgSubmitProposal time.
	evmAnteHandler := evmante.NewAnteHandler(options)
	groupLimiter := newGroupLimiterDecorator(sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}))
	app.SetAnteHandler(func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		return groupLimiter.AnteHandle(ctx, tx, simulate, evmAnteHandler)
	})
}

// onPendingTx forwards a pending Ethereum tx hash to all registered listeners.
// This is used as the PendingTxListener in the ante handler options so the
// EVM JSON-RPC server can stream pending transactions.
func (app *App) onPendingTx(hash common.Hash) {
	for _, listener := range app.pendingTxListeners {
		listener(hash)
	}
}
