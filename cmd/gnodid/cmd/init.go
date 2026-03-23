package cmd

// InitCmd creates a custom genesis init command that calls app.DefaultGenesis()
// instead of the SDK's BasicModuleManager.DefaultGenesis(). This ensures our
// chain-specific overrides (EVM denom, feemarket params, active precompiles, etc.)
// are written to the genesis file when running `gnodid init`.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	bip39 "github.com/cosmos/go-bip39"

	"cosmossdk.io/math/unsafe"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/input"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
	cfg "github.com/cometbft/cometbft/config"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/version"

	"github.com/gnodi-network/gnodi/app"
)

const (
	flagOverwrite        = "overwrite"
	flagRecover          = "recover"
	flagDefaultBondDenom = "default-denom"
	flagConsensusKeyAlgo = "consensus-key-algo"
)

// InitCmd returns the gnodi init command. It calls app.DefaultGenesis() to
// generate the genesis state, ensuring all Gnodi-specific overrides are applied.
func InitCmd(gnodiApp *app.App, defaultNodeHome string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [moniker]",
		Short: "Initialize private validator, p2p, genesis, and application configuration files",
		Long:  `Initialize validators's and node's configuration files.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)

			serverCtx := sdkserver.GetServerContextFromCmd(cmd)
			config := serverCtx.Config
			config.SetRoot(clientCtx.HomeDir)

			chainID, _ := cmd.Flags().GetString(flags.FlagChainID)
			switch {
			case chainID != "":
			case clientCtx.ChainID != "":
				chainID = clientCtx.ChainID
			default:
				chainID = fmt.Sprintf("test-chain-%v", unsafe.Str(6))
			}

			// Get bip39 mnemonic
			var mnemonic string
			recover, _ := cmd.Flags().GetBool(flagRecover)
			if recover {
				inBuf := bufio.NewReader(cmd.InOrStdin())
				value, err := input.GetString("Enter your bip39 mnemonic", inBuf)
				if err != nil {
					return err
				}
				mnemonic = value
				if !bip39.IsMnemonicValid(mnemonic) {
					return errors.New("invalid mnemonic")
				}
			}

			initHeight, _ := cmd.Flags().GetInt64(flags.FlagInitHeight)
			if initHeight < 1 {
				initHeight = 1
			}

			nodeID, _, err := genutil.InitializeNodeValidatorFilesFromMnemonic(config, mnemonic)
			if err != nil {
				return err
			}

			config.Moniker = args[0]

			genFile := config.GenesisFile()
			overwrite, _ := cmd.Flags().GetBool(flagOverwrite)

			_, err = os.Stat(genFile)
			if !overwrite && !os.IsNotExist(err) {
				return fmt.Errorf("genesis.json file already exists: %v", genFile)
			}

			// Use our app's DefaultGenesis() to apply all Gnodi-specific overrides.
			appGenState := gnodiApp.DefaultGenesis()

			appState, err := json.MarshalIndent(appGenState, "", " ")
			if err != nil {
				return fmt.Errorf("failed to marshal default genesis state: %w", err)
			}

			appGenesis := &genutiltypes.AppGenesis{}
			if _, err := os.Stat(genFile); err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			} else {
				appGenesis, err = genutiltypes.AppGenesisFromFile(genFile)
				if err != nil {
					return fmt.Errorf("failed to read genesis doc from file: %w", err)
				}
			}

			appGenesis.AppName = version.AppName
			appGenesis.AppVersion = version.Version
			appGenesis.ChainID = chainID
			appGenesis.AppState = appState
			appGenesis.InitialHeight = initHeight
			appGenesis.Consensus = &genutiltypes.ConsensusGenesis{
				Validators: nil,
				Params:     cmttypes.DefaultConsensusParams(),
			}

			consensusKey, err := cmd.Flags().GetString(flagConsensusKeyAlgo)
			if err != nil {
				return fmt.Errorf("failed to get consensus key algo: %w", err)
			}
			appGenesis.Consensus.Params.Validator.PubKeyTypes = []string{consensusKey}

			if err = genutil.ExportGenesisFile(appGenesis, genFile); err != nil {
				return fmt.Errorf("failed to export genesis file: %w", err)
			}

			toPrint := struct {
				ChainID    string          `json:"chain_id"`
				NodeID     string          `json:"node_id"`
				AppMessage json.RawMessage `json:"app_message"`
			}{
				ChainID:    chainID,
				NodeID:     nodeID,
				AppMessage: appState,
			}
			out, err := json.MarshalIndent(toPrint, "", " ")
			if err != nil {
				return err
			}
			cfg.WriteConfigFile(filepath.Join(config.RootDir, "config", "config.toml"), config)
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return err
		},
	}

	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "node's home directory")
	cmd.Flags().BoolP(flagOverwrite, "o", false, "overwrite the genesis.json file")
	cmd.Flags().Bool(flagRecover, false, "provide seed phrase to recover existing key instead of creating")
	cmd.Flags().String(flagDefaultBondDenom, "", "genesis file default denomination (ignored — Gnodi always uses uGNOD)")
	cmd.Flags().String(flagConsensusKeyAlgo, "ed25519", "algorithm to use for the consensus key")
	cmd.Flags().String(flags.FlagChainID, "", "genesis file chain-id, if left blank will be randomly created")
	cmd.Flags().Int64(flags.FlagInitHeight, 1, "specify the initial block height at genesis")

	return cmd
}
