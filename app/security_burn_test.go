package app

import (
	"encoding/json"
	"os"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/baseapp"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// genesisPath points at the real exported halt-state genesis (block 670552).
// Skipped when absent so CI does not depend on a 6.8MB fixture.
const genesisPath = "../incident/genesis-halt.json"

const (
	testChainID = "gnodi"
	uGNOD       = "uGNOD"

	// expected values measured on mainnet at the halt height
	haltSupply     = "115792089237316195423570985008687907853269984665652947366971569930"
	minterBalance  = "115792089237316195423570985008687907853269984665640464039457134006"
	hop1Balance    = "91000022388053"
	hop2Balance    = "1000069400763"
	escrowBalance  = "22872492772359"
	escrowAddr     = "gnodi1a53udazy8ayufvy0s434pfwjcedzqv343vf68v"
	legitSupplyAft = "12391327422647108"
)

func loadHaltGenesis(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(genesisPath)
	if err != nil {
		t.Skipf("halt genesis fixture not present (%v) - skipping", err)
	}
	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &doc))

	// Verify the fixture really is the halt state before we rely on it.
	var st struct {
		Bank struct {
			Supply   []struct{ Denom, Amount string } `json:"supply"`
			Balances []struct {
				Address string
				Coins   []struct{ Denom, Amount string }
			} `json:"balances"`
		} `json:"bank"`
	}
	require.NoError(t, json.Unmarshal(doc["app_state"], &st))

	var supply string
	for _, c := range st.Bank.Supply {
		if c.Denom == uGNOD {
			supply = c.Amount
		}
	}
	require.Equal(t, haltSupply, supply, "fixture must carry the inflated halt-state supply")

	find := func(addr string) string {
		for _, b := range st.Bank.Balances {
			if b.Address == addr {
				for _, c := range b.Coins {
					if c.Denom == uGNOD {
						return c.Amount
					}
				}
			}
		}
		return "0"
	}
	require.Equal(t, minterBalance, find(securityBurnAccounts[0]), "fixture minter balance")
	require.Equal(t, hop1Balance, find(securityBurnAccounts[1]), "fixture hop1 balance")
	require.Equal(t, hop2Balance, find(securityBurnAccounts[2]), "fixture hop2 balance")
	require.Equal(t, escrowBalance, find(escrowAddr), "fixture escrow balance")

	return doc["app_state"]
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	return New(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.EmptyAppOptions{},
		baseapp.SetChainID(testChainID),
	)
}

func defaultConsensusParams() *cmtproto.ConsensusParams {
	return &cmtproto.ConsensusParams{
		Block:     &cmtproto.BlockParams{MaxBytes: 22020096, MaxGas: -1},
		Evidence:  &cmtproto.EvidenceParams{MaxAgeNumBlocks: 100000, MaxAgeDuration: 172800000000000, MaxBytes: 1048576},
		Validator: &cmtproto.ValidatorParams{PubKeyTypes: []string{"ed25519"}},
	}
}

func bal(t *testing.T, a *App, ctx sdk.Context, bech32 string) string {
	t.Helper()
	addr, err := sdk.AccAddressFromBech32(bech32)
	require.NoError(t, err)
	return a.BankKeeper.GetBalance(ctx, addr, uGNOD).Amount.String()
}

func commitBlock(t *testing.T, a *App, height int64) {
	t.Helper()
	_, err := a.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height: height,
		Hash:   []byte("test-block-hash-0000000000000000"),
	})
	require.NoError(t, err, "block %d must execute without error", height)
	_, err = a.Commit()
	require.NoError(t, err)
}

// TestSecurityBurn runs the real halt-state genesis through InitChain, executes
// block 670553, and asserts the exploit-minted funds are destroyed while
// legitimate state is untouched. It then runs 670554 to confirm the burn is a
// one-shot and does not fire again.
func TestSecurityBurn(t *testing.T) {
	appState := loadHaltGenesis(t) // asserts the fixture is genuinely the halt state
	a := newTestApp(t)

	_, err := a.InitChain(&abci.RequestInitChain{
		ChainId:         testChainID,
		InitialHeight:   SecurityBurnHeight,
		AppStateBytes:   appState,
		ConsensusParams: defaultConsensusParams(),
	})
	require.NoError(t, err, "InitChain from halt genesis must succeed")

	// --- execute the burn height -------------------------------------------
	commitBlock(t, a, SecurityBurnHeight)

	post := a.NewContextLegacy(true, cmtproto.Header{Height: SecurityBurnHeight, ChainID: testChainID})

	for _, acct := range securityBurnAccounts {
		require.Equal(t, "0", bal(t, a, post, acct), "attacker account %s must be empty", acct)
	}
	require.Equal(t, escrowBalance, bal(t, a, post, escrowAddr),
		"channel-0 escrow must be untouched - it backs the Osmosis vouchers")
	require.Equal(t, legitSupplyAft, a.BankKeeper.GetSupply(post, uGNOD).Amount.String(),
		"supply must match the independently-computed genesis-remediation figure")

	// --- next block must change nothing ------------------------------------
	commitBlock(t, a, SecurityBurnHeight+1)

	next := a.NewContextLegacy(true, cmtproto.Header{Height: SecurityBurnHeight + 1, ChainID: testChainID})
	for _, acct := range securityBurnAccounts {
		require.Equal(t, "0", bal(t, a, next, acct), "account %s must stay empty", acct)
	}
	require.Equal(t, escrowBalance, bal(t, a, next, escrowAddr), "escrow must stay untouched")
	require.Equal(t, legitSupplyAft, a.BankKeeper.GetSupply(next, uGNOD).Amount.String(),
		"supply must not change after the burn height")
}
