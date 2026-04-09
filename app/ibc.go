package app

import (
	"cosmossdk.io/core/appmodule"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	evmaddress "github.com/cosmos/evm/encoding/address"
	"github.com/cosmos/evm/x/erc20"
	erc20v2 "github.com/cosmos/evm/x/erc20/v2"
	ibccallbackskeeper "github.com/cosmos/evm/x/ibc/callbacks/keeper"
	"github.com/cosmos/evm/x/ibc/transfer"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	transferv2 "github.com/cosmos/evm/x/ibc/transfer/v2"

	ibccallbacks "github.com/cosmos/ibc-go/v10/modules/apps/callbacks"
	icamodule "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts"
	icacontroller "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller/types"
	icahost "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host"
	icahostkeeper "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v10/modules/core"
	ibcclienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcapi "github.com/cosmos/ibc-go/v10/modules/core/api"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
)

// registerIBCModules initializes the IBC transfer keeper (cosmos/evm version), ICA host and
// controller keepers, and wires up the IBC routing stack with ERC-20 middleware and IBC callbacks.
//
// This must be called AFTER app.Erc20Keeper and app.EVMKeeper are initialized (in New()) because
// the cosmos/evm TransferKeeper and CallbackKeeper both depend on Erc20Keeper, and
// the callbacks keeper needs EVMKeeper.
func (app *App) registerIBCModules() error {
	authAddr := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	// ICA Host keeper — subspace param retained as nil (legacy; ibc-go v10.3 passes nil).
	app.ICAHostKeeper = icahostkeeper.NewKeeper(
		app.appCodec,
		runtime.NewKVStoreService(app.keys[icahosttypes.StoreKey]),
		nil, // legacySubspace
		app.IBCKeeper.ChannelKeeper, // ICS4Wrapper
		app.IBCKeeper.ChannelKeeper, // ChannelKeeper
		app.AccountKeeper,
		app.MsgServiceRouter(),
		app.GRPCQueryRouter(),
		authAddr,
	)

	// ICA Controller keeper — subspace param retained as nil (legacy).
	app.ICAControllerKeeper = icacontrollerkeeper.NewKeeper(
		app.appCodec,
		runtime.NewKVStoreService(app.keys[icacontrollertypes.StoreKey]),
		nil, // legacySubspace
		app.IBCKeeper.ChannelKeeper, // ICS4Wrapper
		app.IBCKeeper.ChannelKeeper, // ChannelKeeper
		app.MsgServiceRouter(),
		authAddr,
	)

	// IBC Transfer keeper (cosmos/evm version — includes Erc20Keeper for ERC-20 IBC transfers).
	// Must be instantiated AFTER Erc20Keeper, since Erc20Keeper holds a pointer to TransferKeeper
	// that is wired at Erc20Keeper construction time.
	app.TransferKeeper = transferkeeper.NewKeeper(
		app.appCodec,
		runtime.NewKVStoreService(app.keys[ibctransfertypes.StoreKey]),
		app.IBCKeeper.ChannelKeeper, // ICS4Wrapper
		app.IBCKeeper.ChannelKeeper, // ChannelKeeper
		app.MsgServiceRouter(),
		app.AccountKeeper,
		app.BankKeeper,
		app.Erc20Keeper,
		authAddr,
	)
	// Use EVM-aware address codec so hex and bech32 addresses are both accepted.
	app.TransferKeeper.SetAddressCodec(evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix()))

	/*
		IBC Transfer Stack (v1)

		The stack is assembled bottom-to-top. Packets flow top-down on receive and
		bottom-up on send:

		  Send:    transfer.SendPacket → erc20.SendPacket → callbacks.SendPacket → channel
		  Receive: channel → callbacks.OnRecvPacket → erc20.OnRecvPacket → transfer.OnRecvPacket
	*/

	// Bottom of stack: core ICS-20 transfer module (cosmos/evm version).
	var transferStack porttypes.IBCModule
	transferStack = transfer.NewIBCModule(app.TransferKeeper)

	// ERC-20 middleware — intercepts transfers and converts ICS-20 coins ↔ ERC-20 tokens.
	transferStack = erc20.NewIBCMiddleware(app.Erc20Keeper, transferStack)

	// IBC Callbacks middleware — calls EVM contracts on packet lifecycle events.
	callbackKeeper := ibccallbackskeeper.NewKeeper(
		app.AccountKeeper,
		app.EVMKeeper,
		app.Erc20Keeper,
	)
	transferStack = ibccallbacks.NewIBCMiddleware(
		transferStack,
		app.IBCKeeper.ChannelKeeper,
		callbackKeeper,
		1_000_000,
	)

	// Rebind TransferKeeper's ICS4Wrapper to the top of the assembled middleware stack.
	// Without this, keeper-originated sends (e.g. MsgTransfer) would bypass the ERC-20 and
	// callbacks middleware and go directly to ChannelKeeper, which was the initial ICS4Wrapper.
	ics4Wrapper, ok := transferStack.(porttypes.ICS4Wrapper)
	if !ok {
		panic("transfer middleware stack does not implement porttypes.ICS4Wrapper")
	}
	app.TransferKeeper.WithICS4Wrapper(ics4Wrapper)

	// IBC Transfer Stack (v2 — IBC Eureka protocol).
	var transferStackV2 ibcapi.IBCModule
	transferStackV2 = transferv2.NewIBCModule(app.TransferKeeper)
	transferStackV2 = erc20v2.NewIBCMiddleware(transferStackV2, app.Erc20Keeper)

	// ICA stacks.
	icaControllerStack := icacontroller.NewIBCMiddleware(app.ICAControllerKeeper)
	icaHostStack := icahost.NewIBCModule(app.ICAHostKeeper)

	// IBC v1 router.
	ibcRouter := porttypes.NewRouter().
		AddRoute(ibctransfertypes.ModuleName, transferStack).
		AddRoute(icacontrollertypes.SubModuleName, icaControllerStack).
		AddRoute(icahosttypes.SubModuleName, icaHostStack)
	app.IBCKeeper.SetRouter(ibcRouter)

	// IBC v2 router (Eureka).
	ibcv2Router := ibcapi.NewRouter().
		AddRoute(ibctransfertypes.PortID, transferStackV2)
	app.IBCKeeper.SetRouterV2(ibcv2Router)

	return nil
}

// RegisterIBC registers IBC modules for use on the client side (e.g. autocli, interface registry).
// Since IBC modules don't support dependency injection, they must be registered manually.
//
// Deprecated: this helper is used by the legacy depinject-based CLI bootstrapping (root.go).
// It will be removed when root.go is rewritten to use the temp-app approach.
func RegisterIBC(cdc codec.Codec) map[string]appmodule.AppModule {
	modules := map[string]appmodule.AppModule{
		ibcexported.ModuleName: ibc.NewAppModule(&ibckeeper.Keeper{}),
		// Use cosmos/evm's transfer module for full ERC-20 support.
		ibctransfertypes.ModuleName: transfer.NewAppModule(transferkeeper.Keeper{}),
		icatypes.ModuleName:         icamodule.NewAppModule(&icacontrollerkeeper.Keeper{}, &icahostkeeper.Keeper{}),
		ibctm.ModuleName:            ibctm.NewAppModule(ibctm.NewLightClientModule(cdc, ibcclienttypes.StoreProvider{})),
	}

	for _, m := range modules {
		if mr, ok := m.(module.AppModuleBasic); ok {
			mr.RegisterInterfaces(cdc.InterfaceRegistry())
		}
	}

	return modules
}
