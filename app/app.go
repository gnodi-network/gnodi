package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	// Force-load the EVM tracer engines to trigger registration
	_ "github.com/ethereum/go-ethereum/eth/tracers/js"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cast"

	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	reflectionv1 "cosmossdk.io/api/cosmos/reflection/v1"
	"cosmossdk.io/client/v2/autocli"
	clienthelpers "cosmossdk.io/client/v2/helpers"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/evidence"
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	feegrantmodule "cosmossdk.io/x/feegrant/module"
	"cosmossdk.io/x/upgrade"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	circuittypes "cosmossdk.io/x/circuit/types"
	circuitmodule "cosmossdk.io/x/circuit"
	"cosmossdk.io/x/nft"
	nftkeeper "cosmossdk.io/x/nft/keeper"
	nftmodule "cosmossdk.io/x/nft/module"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	"github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	runtimeservices "github.com/cosmos/cosmos-sdk/runtime/services"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	"github.com/cosmos/cosmos-sdk/x/auth/posthandler"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/consensus"
	consensuskeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensustypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/epochs"
	epochskeeper "github.com/cosmos/cosmos-sdk/x/epochs/keeper"
	epochstypes "github.com/cosmos/cosmos-sdk/x/epochs/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/group"
	groupkeeper "github.com/cosmos/cosmos-sdk/x/group/keeper"
	groupmodule "github.com/cosmos/cosmos-sdk/x/group/module"
	"github.com/cosmos/cosmos-sdk/x/mint"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramsmodule "github.com/cosmos/cosmos-sdk/x/params"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	// Cosmos EVM
	evmante "github.com/cosmos/evm/ante"
	evmencoding "github.com/cosmos/evm/encoding"
	evmaddress "github.com/cosmos/evm/encoding/address"
	evmmempool "github.com/cosmos/evm/mempool"
	precompiletypes "github.com/cosmos/evm/precompiles/types"
	cosmosevmserver "github.com/cosmos/evm/server"
	srvflags "github.com/cosmos/evm/server/flags"
	"github.com/cosmos/evm/x/erc20"
	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/feemarket"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmibctransfer "github.com/cosmos/evm/x/ibc/transfer"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	"github.com/cosmos/evm/x/precisebank"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	"github.com/cosmos/evm/x/vm"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	// IBC
	ibctransfer "github.com/cosmos/ibc-go/v10/modules/apps/transfer"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	icamodule "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller/types"
	icahostkeeper "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
	ibc "github.com/cosmos/ibc-go/v10/modules/core"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"

	// Gnodi custom module
	distromodule "github.com/gnodi-network/gnodi/x/distro/module"
	distromodulekeeper "github.com/gnodi-network/gnodi/x/distro/keeper"
	distromoduletypes "github.com/gnodi-network/gnodi/x/distro/types"

	"github.com/gnodi-network/gnodi/docs"

	"github.com/cosmos/gogoproto/proto"
)

const (
	// Name is the name of the application.
	Name = "gnodi"
	// AccountAddressPrefix is the prefix for accounts addresses.
	AccountAddressPrefix = "gnodi"
	// ChainCoinType is the coin type of the chain.
	ChainCoinType = 118
)

// DefaultNodeHome default home directories for the application daemon.
var DefaultNodeHome string

var _ cosmosevmserver.Application = (*App)(nil)

// maccPerms defines module account permissions.
var maccPerms = map[string][]string{
	authtypes.FeeCollectorName:            nil,
	distrtypes.ModuleName:                nil,
	minttypes.ModuleName:                 {authtypes.Minter},
	stakingtypes.BondedPoolName:          {authtypes.Burner, authtypes.Staking},
	stakingtypes.NotBondedPoolName:       {authtypes.Burner, authtypes.Staking},
	govtypes.ModuleName:                  {authtypes.Burner},
	nft.ModuleName:                       nil,
	ibctransfertypes.ModuleName:          {authtypes.Minter, authtypes.Burner},
	icatypes.ModuleName:                  nil,
	distromoduletypes.ModuleName:         {authtypes.Minter, authtypes.Burner, authtypes.Staking},
	// Cosmos EVM modules
	evmtypes.ModuleName:         {authtypes.Minter, authtypes.Burner},
	feemarkettypes.ModuleName:   nil,
	erc20types.ModuleName:       {authtypes.Minter, authtypes.Burner},
	precisebanktypes.ModuleName: {authtypes.Minter, authtypes.Burner},
}

// blockedAddrs returns all app blocked module account addresses.
func blockedAddrs() map[string]bool {
	result := make(map[string]bool)
	for acc := range maccPerms {
		result[authtypes.NewModuleAddress(acc).String()] = true
	}
	// Allow gov to receive funds
	delete(result, authtypes.NewModuleAddress(govtypes.ModuleName).String())
	return result
}

// App extends an ABCI application with most parameters exported for convenience.
type App struct {
	*baseapp.BaseApp

	legacyAmino       *codec.LegacyAmino
	appCodec          codec.Codec
	interfaceRegistry codectypes.InterfaceRegistry
	txConfig          client.TxConfig

	pendingTxListeners []evmante.PendingTxListener
	clientCtx          client.Context

	// KV and transient store keys
	keys  map[string]*storetypes.KVStoreKey
	tkeys map[string]*storetypes.TransientStoreKey

	// Keepers — standard SDK modules
	AccountKeeper         authkeeper.AccountKeeper
	BankKeeper            bankkeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        slashingkeeper.Keeper
	MintKeeper            mintkeeper.Keeper
	DistrKeeper           distrkeeper.Keeper
	GovKeeper             govkeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	AuthzKeeper           authzkeeper.Keeper
	EvidenceKeeper        evidencekeeper.Keeper
	FeeGrantKeeper        feegrantkeeper.Keeper
	ConsensusParamsKeeper consensuskeeper.Keeper
	CircuitBreakerKeeper  circuitkeeper.Keeper
	ParamsKeeper          paramskeeper.Keeper
	EpochsKeeper          epochskeeper.Keeper
	NftKeeper             nftkeeper.Keeper
	GroupKeeper           groupkeeper.Keeper

	// IBC keepers
	IBCKeeper           *ibckeeper.Keeper
	TransferKeeper      transferkeeper.Keeper
	ICAControllerKeeper icacontrollerkeeper.Keeper
	ICAHostKeeper       icahostkeeper.Keeper

	// Cosmos EVM keepers
	FeeMarketKeeper   feemarketkeeper.Keeper
	EVMKeeper         *evmkeeper.Keeper
	Erc20Keeper       erc20keeper.Keeper
	PreciseBankKeeper precisebankkeeper.Keeper
	EVMMempool        *evmmempool.ExperimentalEVMMempool

	// Gnodi custom module
	DistroKeeper distromodulekeeper.Keeper

	// Module management
	ModuleManager      *module.Manager
	BasicModuleManager module.BasicManager

	// Simulation manager
	sm *module.SimulationManager

	// Module configurator
	configurator module.Configurator
}

func init() {
	var err error
	clienthelpers.EnvPrefix = Name
	DefaultNodeHome, err = clienthelpers.GetNodeHomeDirectory("." + Name)
	if err != nil {
		panic(err)
	}
}

// New returns a reference to an initialized App.
func New(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) *App {
	// evmChainID is 0 when app.New is called for CLI bootstrapping (tempApp with empty opts).
	// DefaultChainConfig(0) sets the sentinel ChainId=262144, which allows the real node
	// start (--evm.evm-chain-id 46634) to override it without panicking.
	evmChainID := cast.ToUint64(appOpts.Get(srvflags.EVMChainID))
	encodingConfig := evmencoding.MakeConfig(evmChainID)

	appCodec := encodingConfig.Codec
	legacyAmino := encodingConfig.Amino
	interfaceRegistry := encodingConfig.InterfaceRegistry
	txConfig := encodingConfig.TxConfig

	bApp := baseapp.NewBaseApp(
		Name,
		logger,
		db,
		encodingConfig.TxConfig.TxDecoder(),
		baseAppOptions...,
	)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetTxEncoder(txConfig.TxEncoder())

	keys := storetypes.NewKVStoreKeys(
		// Standard SDK
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, consensustypes.StoreKey,
		upgradetypes.StoreKey, feegrant.StoreKey, evidencetypes.StoreKey, authzkeeper.StoreKey,
		// Extra SDK modules
		paramstypes.StoreKey, nft.StoreKey, group.StoreKey, circuittypes.StoreKey,
		epochstypes.StoreKey,
		// IBC
		ibcexported.StoreKey, ibctransfertypes.StoreKey,
		icahosttypes.StoreKey, icacontrollertypes.StoreKey,
		// Cosmos EVM
		evmtypes.StoreKey, feemarkettypes.StoreKey, erc20types.StoreKey, precisebanktypes.StoreKey,
		// Gnodi custom
		distromoduletypes.StoreKey,
	)

	tkeys := storetypes.NewTransientStoreKeys(
		paramstypes.TStoreKey,
		evmtypes.TransientKey,
		feemarkettypes.TransientKey,
	)

	if err := bApp.RegisterStreamingServices(appOpts, keys); err != nil {
		fmt.Printf("failed to load state streaming: %s", err)
		os.Exit(1)
	}

	app := &App{
		BaseApp:           bApp,
		legacyAmino:       legacyAmino,
		appCodec:          appCodec,
		txConfig:          txConfig,
		interfaceRegistry: interfaceRegistry,
		keys:              keys,
		tkeys:             tkeys,
	}

	authAddr := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	// Consensus params keeper
	app.ConsensusParamsKeeper = consensuskeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[consensustypes.StoreKey]),
		authAddr,
		runtime.EventService{},
	)
	bApp.SetParamStore(app.ConsensusParamsKeeper.ParamsStore)

	// Params keeper (vestigial — retained for legacy IBC subspace migration)
	app.ParamsKeeper = paramskeeper.NewKeeper(
		appCodec, legacyAmino,
		keys[paramstypes.StoreKey],
		tkeys[paramstypes.TStoreKey],
	)

	// Account keeper with EVM address codec
	app.AccountKeeper = authkeeper.NewAccountKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[authtypes.StoreKey]),
		authtypes.ProtoBaseAccount,
		maccPerms,
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
		sdk.GetConfig().GetBech32AccountAddrPrefix(),
		authAddr,
	)

	// Bank keeper
	app.BankKeeper = bankkeeper.NewBaseKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[banktypes.StoreKey]),
		app.AccountKeeper,
		blockedAddrs(),
		authAddr,
		logger,
	)

	// Re-build tx config with sign mode textual after bank keeper is set
	enabledSignModes := append(authtx.DefaultSignModes, signingtypes.SignMode_SIGN_MODE_TEXTUAL) //nolint:gocritic
	txConfigOpts := authtx.ConfigOptions{
		EnabledSignModes:           enabledSignModes,
		TextualCoinMetadataQueryFn: txmodule.NewBankKeeperCoinMetadataQueryFn(app.BankKeeper),
	}
	var err error
	txConfig, err = authtx.NewTxConfigWithOptions(appCodec, txConfigOpts)
	if err != nil {
		panic(err)
	}
	app.txConfig = txConfig

	// Staking keeper with EVM address codecs for validator/consensus addresses
	app.StakingKeeper = stakingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[stakingtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		authAddr,
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()),
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ConsensusAddrPrefix()),
	)

	app.MintKeeper = mintkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[minttypes.StoreKey]),
		app.StakingKeeper,
		app.AccountKeeper,
		app.BankKeeper,
		authtypes.FeeCollectorName,
		authAddr,
	)

	app.DistrKeeper = distrkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[distrtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		authtypes.FeeCollectorName,
		authAddr,
	)

	app.SlashingKeeper = slashingkeeper.NewKeeper(
		appCodec,
		app.LegacyAmino(),
		runtime.NewKVStoreService(keys[slashingtypes.StoreKey]),
		app.StakingKeeper,
		authAddr,
	)

	app.FeeGrantKeeper = feegrantkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[feegrant.StoreKey]),
		app.AccountKeeper,
	)

	// Set staking hooks (must be after DistrKeeper and SlashingKeeper are set)
	app.StakingKeeper.SetHooks(
		stakingtypes.NewMultiStakingHooks(app.DistrKeeper.Hooks(), app.SlashingKeeper.Hooks()),
	)

	app.AuthzKeeper = authzkeeper.NewKeeper(
		runtime.NewKVStoreService(keys[authzkeeper.StoreKey]),
		appCodec,
		app.MsgServiceRouter(),
		app.AccountKeeper,
	)

	skipUpgradeHeights := map[int64]bool{}
	for _, h := range cast.ToIntSlice(appOpts.Get(sdkserver.FlagUnsafeSkipUpgrades)) {
		skipUpgradeHeights[int64(h)] = true
	}
	homePath := cast.ToString(appOpts.Get(flags.FlagHome))
	app.UpgradeKeeper = upgradekeeper.NewKeeper(
		skipUpgradeHeights,
		runtime.NewKVStoreService(keys[upgradetypes.StoreKey]),
		appCodec,
		homePath,
		app.BaseApp,
		authAddr,
	)

	govConfig := govtypes.DefaultConfig()
	govKeeper := govkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[govtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		app.DistrKeeper,
		app.MsgServiceRouter(),
		govConfig,
		authAddr,
	)
	app.GovKeeper = *govKeeper.SetHooks(govtypes.NewMultiGovHooks())

	evidenceKeeper := evidencekeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[evidencetypes.StoreKey]),
		app.StakingKeeper,
		app.SlashingKeeper,
		app.AccountKeeper.AddressCodec(),
		runtime.ProvideCometInfoService(),
	)
	app.EvidenceKeeper = *evidenceKeeper

	app.CircuitBreakerKeeper = circuitkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[circuittypes.StoreKey]),
		authAddr,
		app.AccountKeeper.AddressCodec(),
	)
	// Wire the circuit breaker so that entries written to the DisableList are
	// enforced at the BaseApp message router level (GCA-01).
	app.SetCircuitBreaker(&app.CircuitBreakerKeeper)

	app.NftKeeper = nftkeeper.NewKeeper(
		runtime.NewKVStoreService(keys[nft.StoreKey]),
		appCodec,
		app.AccountKeeper,
		app.BankKeeper,
	)

	app.GroupKeeper = groupkeeper.NewKeeper(
		keys[group.StoreKey],
		appCodec,
		app.MsgServiceRouter(),
		app.AccountKeeper,
		group.DefaultConfig(),
	)

	app.EpochsKeeper = epochskeeper.NewKeeper(
		runtime.NewKVStoreService(keys[epochstypes.StoreKey]),
		appCodec,
	)

	// IBC core keeper (subspace param removed in ibc-go v10.3)
	app.IBCKeeper = ibckeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[ibcexported.StoreKey]),
		nil,
		app.UpgradeKeeper,
		authAddr,
	)

	// ── Cosmos EVM keepers ──────────────────────────────────────────────────────

	app.FeeMarketKeeper = feemarketkeeper.NewKeeper(
		appCodec,
		authtypes.NewModuleAddress(govtypes.ModuleName),
		keys[feemarkettypes.StoreKey],
		tkeys[feemarkettypes.TransientKey],
	)

	// PreciseBank wraps BankKeeper for 18-decimal EVM precision.
	// 1 uGNOD = 1,000,000,000,000 aGNOD (1e12)
	app.PreciseBankKeeper = precisebankkeeper.NewKeeper(
		appCodec,
		keys[precisebanktypes.StoreKey],
		app.BankKeeper,
		app.AccountKeeper,
	)

	tracer := cast.ToString(appOpts.Get(srvflags.EVMTracer))
	app.EVMKeeper = evmkeeper.NewKeeper(
		appCodec,
		keys[evmtypes.StoreKey],
		tkeys[evmtypes.TransientKey],
		keys,
		authtypes.NewModuleAddress(govtypes.ModuleName),
		app.AccountKeeper,
		app.PreciseBankKeeper,
		app.StakingKeeper,
		app.FeeMarketKeeper,
		&app.ConsensusParamsKeeper,
		&app.Erc20Keeper,
		evmChainID,
		tracer,
	).WithDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         "uGNOD",
		ExtendedDenom: "aGNOD",
		DisplayDenom:  "GNOD",
		Decimals:      evmtypes.SixDecimals.Uint32(),
	}).WithStaticPrecompiles(
		precompiletypes.DefaultStaticPrecompiles(
			*app.StakingKeeper,
			app.DistrKeeper,
			app.PreciseBankKeeper,
			&app.Erc20Keeper,
			&app.TransferKeeper,
			app.IBCKeeper.ChannelKeeper,
			app.GovKeeper,
			app.SlashingKeeper,
			appCodec,
		),
	)

	app.Erc20Keeper = erc20keeper.NewKeeper(
		keys[erc20types.StoreKey],
		appCodec,
		authtypes.NewModuleAddress(govtypes.ModuleName),
		app.AccountKeeper,
		app.PreciseBankKeeper,
		app.EVMKeeper,
		app.StakingKeeper,
		&app.TransferKeeper,
	)

	// ── IBC modules ─────────────────────────────────────────────────────────────
	// registerIBCModules initializes TransferKeeper, ICA keepers, and sets up
	// IBC routing stacks with ERC-20 middleware.
	if err := app.registerIBCModules(); err != nil {
		panic(err)
	}

	// ── Gnodi custom distro module ───────────────────────────────────────────────
	app.DistroKeeper = distromodulekeeper.NewKeeper(
		runtime.NewKVStoreService(keys[distromoduletypes.StoreKey]),
		appCodec,
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
		authtypes.NewModuleAddress(govtypes.ModuleName),
		app.BankKeeper,
		app.AccountKeeper,
	)

	// ── Module manager ──────────────────────────────────────────────────────────

	storeProvider := app.IBCKeeper.ClientKeeper.GetStoreProvider()
	tmLightClientModule := ibctm.NewLightClientModule(appCodec, storeProvider)
	app.IBCKeeper.ClientKeeper.AddRoute(ibctm.ModuleName, &tmLightClientModule)

	transferModule := evmibctransfer.NewAppModule(app.TransferKeeper)

	app.ModuleManager = module.NewManager(
		genutil.NewAppModule(app.AccountKeeper, app.StakingKeeper, app, app.txConfig),
		auth.NewAppModule(appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, nil),
		vesting.NewAppModule(app.AccountKeeper, app.BankKeeper),
		bank.NewAppModule(appCodec, app.BankKeeper, app.AccountKeeper, nil),
		feegrantmodule.NewAppModule(appCodec, app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper, app.interfaceRegistry),
		gov.NewAppModule(appCodec, &app.GovKeeper, app.AccountKeeper, app.BankKeeper, nil),
		mint.NewAppModule(appCodec, app.MintKeeper, app.AccountKeeper, nil, nil),
		slashing.NewAppModule(appCodec, app.SlashingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, nil, app.interfaceRegistry),
		distr.NewAppModule(appCodec, app.DistrKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, nil),
		staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, nil),
		upgrade.NewAppModule(app.UpgradeKeeper, app.AccountKeeper.AddressCodec()),
		evidence.NewAppModule(app.EvidenceKeeper),
		authzmodule.NewAppModule(appCodec, app.AuthzKeeper, app.AccountKeeper, app.BankKeeper, app.interfaceRegistry),
		consensus.NewAppModule(appCodec, app.ConsensusParamsKeeper),
		circuitmodule.NewAppModule(appCodec, app.CircuitBreakerKeeper),
		nftmodule.NewAppModule(appCodec, app.NftKeeper, app.AccountKeeper, app.BankKeeper, app.interfaceRegistry),
		groupmodule.NewAppModule(appCodec, app.GroupKeeper, app.AccountKeeper, app.BankKeeper, app.interfaceRegistry),
		paramsmodule.NewAppModule(app.ParamsKeeper),
		epochs.NewAppModule(app.EpochsKeeper),
		// IBC modules
		ibc.NewAppModule(app.IBCKeeper),
		ibctm.NewAppModule(tmLightClientModule),
		transferModule,
		icamodule.NewAppModule(&app.ICAControllerKeeper, &app.ICAHostKeeper),
		// Cosmos EVM modules
		vm.NewAppModule(app.EVMKeeper, app.AccountKeeper, app.BankKeeper, app.AccountKeeper.AddressCodec()),
		feemarket.NewAppModule(app.FeeMarketKeeper),
		erc20.NewAppModule(app.Erc20Keeper, app.AccountKeeper),
		precisebank.NewAppModule(app.PreciseBankKeeper, app.BankKeeper, app.AccountKeeper),
		// Gnodi custom module
		distromodule.NewAppModule(appCodec, app.DistroKeeper, app.AccountKeeper, app.BankKeeper),
	)

	app.BasicModuleManager = module.NewBasicManagerFromManager(
		app.ModuleManager,
		map[string]module.AppModuleBasic{
			genutiltypes.ModuleName:     genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
			govtypes.ModuleName:         gov.NewAppModuleBasic(nil),
			ibctransfertypes.ModuleName: evmibctransfer.AppModuleBasic{AppModuleBasic: &ibctransfer.AppModuleBasic{}},
		},
	)
	app.BasicModuleManager.RegisterLegacyAminoCodec(legacyAmino)
	app.BasicModuleManager.RegisterInterfaces(interfaceRegistry)

	// ── Module ordering ─────────────────────────────────────────────────────────

	app.ModuleManager.SetOrderPreBlockers(
		upgradetypes.ModuleName,
		authtypes.ModuleName,
		evmtypes.ModuleName,
	)

	app.ModuleManager.SetOrderBeginBlockers(
		minttypes.ModuleName,
		// IBC
		ibcexported.ModuleName, ibctransfertypes.ModuleName,
		// Cosmos EVM (erc20 before feemarket, feemarket before evm)
		erc20types.ModuleName, feemarkettypes.ModuleName, evmtypes.ModuleName,
		// SDK standard
		distrtypes.ModuleName, slashingtypes.ModuleName,
		evidencetypes.ModuleName, stakingtypes.ModuleName,
		authtypes.ModuleName, banktypes.ModuleName, govtypes.ModuleName, genutiltypes.ModuleName,
		authz.ModuleName, feegrant.ModuleName, consensustypes.ModuleName,
		epochstypes.ModuleName, precisebanktypes.ModuleName, vestingtypes.ModuleName,
		icatypes.ModuleName,
		nft.ModuleName, group.ModuleName, circuittypes.ModuleName, paramstypes.ModuleName,
		// Gnodi
		distromoduletypes.ModuleName,
	)

	app.ModuleManager.SetOrderEndBlockers(
		govtypes.ModuleName, stakingtypes.ModuleName,
		authtypes.ModuleName, banktypes.ModuleName,
		// Cosmos EVM
		evmtypes.ModuleName, erc20types.ModuleName, feemarkettypes.ModuleName,
		// no-ops
		ibcexported.ModuleName, ibctransfertypes.ModuleName,
		distrtypes.ModuleName, slashingtypes.ModuleName, minttypes.ModuleName,
		genutiltypes.ModuleName, evidencetypes.ModuleName, authz.ModuleName,
		feegrant.ModuleName, upgradetypes.ModuleName, consensustypes.ModuleName,
		precisebanktypes.ModuleName, vestingtypes.ModuleName, epochstypes.ModuleName,
		icatypes.ModuleName,
		nft.ModuleName, group.ModuleName, circuittypes.ModuleName, paramstypes.ModuleName,
		// Gnodi
		distromoduletypes.ModuleName,
	)

	genesisModuleOrder := []string{
		consensustypes.ModuleName,
		authtypes.ModuleName, banktypes.ModuleName,
		distrtypes.ModuleName, stakingtypes.ModuleName, slashingtypes.ModuleName,
		govtypes.ModuleName, minttypes.ModuleName,
		evidencetypes.ModuleName, authz.ModuleName, feegrant.ModuleName,
		vestingtypes.ModuleName, nft.ModuleName, group.ModuleName,
		upgradetypes.ModuleName, circuittypes.ModuleName, epochstypes.ModuleName,
		paramstypes.ModuleName,
		// IBC core
		ibcexported.ModuleName,
		// Cosmos EVM (feemarket before genutil — gentx uses MinGasPriceDecorator)
		evmtypes.ModuleName, feemarkettypes.ModuleName, erc20types.ModuleName, precisebanktypes.ModuleName,
		// IBC transfer after EVM
		ibctransfertypes.ModuleName, icatypes.ModuleName,
		// genutil
		genutiltypes.ModuleName,
		// Gnodi
		distromoduletypes.ModuleName,
	}
	app.ModuleManager.SetOrderInitGenesis(genesisModuleOrder...)
	app.ModuleManager.SetOrderExportGenesis(genesisModuleOrder...)

	// ── Register services ───────────────────────────────────────────────────────

	app.configurator = module.NewConfigurator(appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	if err = app.ModuleManager.RegisterServices(app.configurator); err != nil {
		panic(fmt.Sprintf("failed to register module services: %s", err))
	}

	app.RegisterUpgradeHandlers()

	autocliv1.RegisterQueryServer(app.GRPCQueryRouter(), runtimeservices.NewAutoCLIQueryService(app.ModuleManager.Modules))

	reflectionSvc, err := runtimeservices.NewReflectionService()
	if err != nil {
		panic(err)
	}
	reflectionv1.RegisterReflectionServiceServer(app.GRPCQueryRouter(), reflectionSvc)

	// Simulation manager
	overrideModules := map[string]module.AppModuleSimulation{
		authtypes.ModuleName: auth.NewAppModule(appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, nil),
	}
	app.sm = module.NewSimulationManagerFromAppModules(app.ModuleManager.Modules, overrideModules)
	app.sm.RegisterStoreDecoders()

	// Mount stores
	app.MountKVStores(keys)
	app.MountTransientStores(tkeys)

	// ── Initialize BaseApp hooks ─────────────────────────────────────────────────

	maxGasWanted := cast.ToUint64(appOpts.Get(srvflags.EVMMaxTxGasWanted))

	app.SetInitChainer(app.InitChainer)
	app.SetPreBlocker(app.PreBlocker)
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetEndBlocker(app.EndBlocker)

	app.setAnteHandler(app.txConfig, maxGasWanted)
	app.setPostHandler()

	if err := app.configureEVMMempool(appOpts, logger); err != nil {
		panic(fmt.Sprintf("failed to configure EVM mempool: %s", err))
	}

	// Validate proto annotations at startup
	protoFiles, err := proto.MergedRegistry()
	if err != nil {
		panic(err)
	}
	if err := msgservice.ValidateProtoAnnotations(protoFiles); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			logger.Error("error on loading last version", "err", err)
			os.Exit(1)
		}
	}

	return app
}

// GetSubspace returns a param subspace for a given module name.
// Retained for legacy IBC subspace migration compatibility.
func (app *App) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

func (app *App) setPostHandler() {
	postHandler, err := posthandler.NewPostHandler(posthandler.HandlerOptions{})
	if err != nil {
		panic(err)
	}
	app.SetPostHandler(postHandler)
}

// RegisterPendingTxListener registers a listener for pending transactions.
// Required by cosmosevmserver.AppWithPendingTxStream.
func (app *App) RegisterPendingTxListener(listener func(common.Hash)) {
	app.pendingTxListeners = append(app.pendingTxListeners, listener)
}

// Name returns the name of the App.
func (app *App) Name() string { return app.BaseApp.Name() }

// BeginBlocker runs the begin-block logic for every block.
func (app *App) BeginBlocker(ctx sdk.Context) (sdk.BeginBlock, error) {
	// Emergency hotfix: MinGasPrice was incorrectly set to 1_000_000_000 (1 Gwei
	// in 18-decimal Wei units) during the evm-upgrade handler. The
	// MinGasPriceDecorator applies this value directly in uGNOD to Cosmos txs
	// without 18-decimal conversion, making all Cosmos transactions require
	// ~1 billion uGNOD per gas unit. This one-time patch resets it to zero.
	// Zero is correct for Cosmos txs; EVM tx fees are enforced separately via
	// CheckGlobalFee when a non-zero BaseFee is set.
	feeMarketParams := app.FeeMarketKeeper.GetParams(ctx)
	if feeMarketParams.MinGasPrice.Equal(sdkmath.LegacyNewDec(1_000_000_000)) {
		feeMarketParams.MinGasPrice = sdkmath.LegacyZeroDec()
		if err := app.FeeMarketKeeper.SetParams(ctx, feeMarketParams); err != nil {
			panic(fmt.Sprintf("failed to apply MinGasPrice hotfix: %v", err))
		}
	}
	return app.ModuleManager.BeginBlock(ctx)
}

// EndBlocker runs the end-block logic for every block.
func (app *App) EndBlocker(ctx sdk.Context) (sdk.EndBlock, error) {
	return app.ModuleManager.EndBlock(ctx)
}

// PreBlocker runs the pre-block logic for every block.
func (app *App) PreBlocker(ctx sdk.Context, _ *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
	return app.ModuleManager.PreBlock(ctx)
}

// InitChainer runs at chain initialization.
func (app *App) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}
	if err := app.UpgradeKeeper.SetModuleVersionMap(ctx, app.ModuleManager.GetVersionMap()); err != nil {
		panic(err)
	}
	return app.ModuleManager.InitGenesis(ctx, app.appCodec, genesisState)
}

// LoadHeight loads a particular height.
func (app *App) LoadHeight(height int64) error {
	return app.LoadVersion(height)
}

// LegacyAmino returns App's amino codec.
func (app *App) LegacyAmino() *codec.LegacyAmino { return app.legacyAmino }

// AppCodec returns App's app codec.
func (app *App) AppCodec() codec.Codec { return app.appCodec }

// InterfaceRegistry returns App's InterfaceRegistry.
func (app *App) InterfaceRegistry() codectypes.InterfaceRegistry { return app.interfaceRegistry }

// TxConfig returns App's TxConfig.
func (app *App) TxConfig() client.TxConfig { return app.txConfig }

// GetTxConfig returns App's TxConfig (required by TestingApp interface).
func (app *App) GetTxConfig() client.TxConfig { return app.txConfig }

// GetKey returns the KVStoreKey for the provided store key.
func (app *App) GetKey(storeKey string) *storetypes.KVStoreKey { return app.keys[storeKey] }

// SimulationManager implements the SimulationApp interface.
func (app *App) SimulationManager() *module.SimulationManager { return app.sm }

// GetStoreKeys returns all KV and transient store keys registered in the app.
// Used by simulation tests that need to iterate all stores.
func (app *App) GetStoreKeys() []storetypes.StoreKey {
	keys := make([]storetypes.StoreKey, 0, len(app.keys)+len(app.tkeys))
	for _, k := range app.keys {
		keys = append(keys, k)
	}
	for _, k := range app.tkeys {
		keys = append(keys, k)
	}
	return keys
}

// Configurator returns the module configurator.
func (app *App) Configurator() module.Configurator { return app.configurator }

// DefaultGenesis returns a default genesis from the registered AppModuleBasic's.
func (app *App) DefaultGenesis() map[string]json.RawMessage {
	genesis := app.BasicModuleManager.DefaultGenesis(app.appCodec)
	genesis[evmtypes.ModuleName] = app.appCodec.MustMarshalJSON(NewEVMGenesisState())
	genesis[erc20types.ModuleName] = app.appCodec.MustMarshalJSON(NewErc20GenesisState())
	genesis[feemarkettypes.ModuleName] = app.appCodec.MustMarshalJSON(NewFeeMarketGenesisState())

	// Register bank denom metadata for the native EVM denom (uGNOD).
	// InitEvmCoinInfo (called during x/vm InitGenesis) looks up bank metadata using
	// params.EvmDenom to determine the coin's display name and decimal precision.
	var bankGenState banktypes.GenesisState
	app.appCodec.MustUnmarshalJSON(genesis[banktypes.ModuleName], &bankGenState)
	bankGenState.DenomMetadata = append(bankGenState.DenomMetadata, banktypes.Metadata{
		Description: "Gnodi native token",
		Base:        evmtypes.GetEVMCoinDenom(),
		DenomUnits: []*banktypes.DenomUnit{
			{Denom: evmtypes.GetEVMCoinDenom(), Exponent: 0},
			{Denom: evmtypes.GetEVMCoinDisplayDenom(), Exponent: uint32(evmtypes.GetEVMCoinDecimals())},
		},
		Name:    "Gnodi",
		Symbol:  evmtypes.GetEVMCoinDisplayDenom(),
		Display: evmtypes.GetEVMCoinDisplayDenom(),
	})
	genesis[banktypes.ModuleName] = app.appCodec.MustMarshalJSON(&bankGenState)

	return genesis
}

// RegisterAPIRoutes registers all application module routes with the provided API server.
func (app *App) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	clientCtx := apiSvr.ClientCtx
	authtx.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	cmtservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	node.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	app.BasicModuleManager.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	if err := sdkserver.RegisterSwaggerAPI(apiSvr.ClientCtx, apiSvr.Router, apiConfig.Swagger); err != nil {
		panic(err)
	}
	docs.RegisterOpenAPIService(Name, apiSvr.Router)
}

// RegisterTxService implements the Application.RegisterTxService method.
func (app *App) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.GRPCQueryRouter(), clientCtx, app.Simulate, app.interfaceRegistry)
}

// RegisterTendermintService implements the Application.RegisterTendermintService method.
func (app *App) RegisterTendermintService(clientCtx client.Context) {
	cmtservice.RegisterTendermintService(clientCtx, app.GRPCQueryRouter(), app.interfaceRegistry, app.Query)
}

// RegisterNodeService implements the Application.RegisterNodeService method.
func (app *App) RegisterNodeService(clientCtx client.Context, cfg config.Config) {
	node.RegisterNodeService(clientCtx, app.GRPCQueryRouter(), cfg)
}

// AutoCliOpts returns the autocli options for the app.
func (app *App) AutoCliOpts() autocli.AppOptions {
	modules := make(map[string]appmodule.AppModule)
	for _, m := range app.ModuleManager.Modules {
		if moduleWithName, ok := m.(module.HasName); ok {
			if appModule, ok := moduleWithName.(appmodule.AppModule); ok {
				modules[moduleWithName.Name()] = appModule
			}
		}
	}
	return autocli.AppOptions{
		Modules:               modules,
		ModuleOptions:         runtimeservices.ExtractAutoCLIOptions(app.ModuleManager.Modules),
		AddressCodec:          evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
		ValidatorAddressCodec: evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()),
		ConsensusAddressCodec: evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ConsensusAddrPrefix()),
	}
}

// GetAnteHandler returns the ante handler.
func (app *App) GetAnteHandler() sdk.AnteHandler {
	return app.BaseApp.AnteHandler()
}

// GetMempool returns the EVM mempool.
func (app *App) GetMempool() sdkmempool.ExtMempool {
	return app.EVMMempool
}

// SetClientCtx is required by cosmosevmserver.Application.
func (app *App) SetClientCtx(clientCtx client.Context) {
	app.clientCtx = clientCtx
}

// ── cosmosevmserver.Application getter methods ──────────────────────────────

func (app *App) GetEVMKeeper() *evmkeeper.Keeper                      { return app.EVMKeeper }
func (app *App) GetErc20Keeper() *erc20keeper.Keeper                   { return &app.Erc20Keeper }
func (app *App) GetFeeMarketKeeper() *feemarketkeeper.Keeper           { return &app.FeeMarketKeeper }
func (app *App) GetPreciseBankKeeper() *precisebankkeeper.Keeper       { return &app.PreciseBankKeeper }
func (app *App) GetAccountKeeper() authkeeper.AccountKeeper            { return app.AccountKeeper }
func (app *App) GetBankKeeper() bankkeeper.Keeper                      { return app.BankKeeper }
func (app *App) GetStakingKeeper() *stakingkeeper.Keeper               { return app.StakingKeeper }
func (app *App) GetDistrKeeper() distrkeeper.Keeper                    { return app.DistrKeeper }
func (app *App) GetGovKeeper() govkeeper.Keeper                        { return app.GovKeeper }
func (app *App) GetSlashingKeeper() slashingkeeper.Keeper              { return app.SlashingKeeper }
func (app *App) GetMintKeeper() mintkeeper.Keeper                      { return app.MintKeeper }
func (app *App) GetFeeGrantKeeper() feegrantkeeper.Keeper              { return app.FeeGrantKeeper }
func (app *App) GetAuthzKeeper() authzkeeper.Keeper                    { return app.AuthzKeeper }
func (app *App) GetEvidenceKeeper() *evidencekeeper.Keeper             { return &app.EvidenceKeeper }
func (app *App) GetConsensusParamsKeeper() consensuskeeper.Keeper      { return app.ConsensusParamsKeeper }
func (app *App) GetIBCKeeper() *ibckeeper.Keeper                      { return app.IBCKeeper }
func (app *App) GetTransferKeeper() transferkeeper.Keeper              { return app.TransferKeeper }

func (app *App) SetErc20Keeper(k erc20keeper.Keeper)         { app.Erc20Keeper = k }
func (app *App) SetTransferKeeper(k transferkeeper.Keeper)   { app.TransferKeeper = k }

// Close shuts down the EVM mempool and underlying BaseApp.
func (app *App) Close() error {
	var mErr error
	if m, ok := app.GetMempool().(*evmmempool.ExperimentalEVMMempool); ok {
		app.Logger().Info("Shutting down mempool")
		mErr = m.Close()
	}
	return errors.Join(mErr, app.BaseApp.Close())
}

// GetMaccPerms returns a copy of the module account permissions.
func GetMaccPerms() map[string][]string {
	dup := make(map[string][]string, len(maccPerms))
	for k, v := range maccPerms {
		dup[k] = v
	}
	return dup
}

// BlockedAddresses returns all the app's blocked account addresses.
func BlockedAddresses() map[string]bool {
	return blockedAddrs()
}
