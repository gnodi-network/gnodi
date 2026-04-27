package types_test

import (
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gnodi-network/gnodi/x/distro/types"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("gnodi", "gnodipub")
	os.Exit(m.Run())
}

func TestGenesisState_Validate(t *testing.T) {
	tests := []struct {
		desc     string
		genState *types.GenesisState
		valid    bool
	}{
		{
			desc:     "default genesis is valid",
			genState: types.DefaultGenesis(),
			valid:    true,
		},
		{
			desc: "fully configured genesis is valid",
			genState: &types.GenesisState{
				Params: types.NewParams(
					"gnodi1znqekah4r9q8g69v9jt062xtl4ygjwy0n68uuu",
					"gnodi1znqekah4r9q8g69v9jt062xtl4ygjwy0n68uuu",
					"uGNOD",
					35_000_000_000_000_000,
					"2025-07-22",
					12,
				),
			},
			valid: true,
		},
		{
			desc: "empty addresses are allowed in genesis",
			genState: &types.GenesisState{
				Params: types.NewParams("", "", "uGNOD", 35_000_000_000_000_000, "2025-07-22", 12),
			},
			valid: true,
		},
		{
			desc: "invalid minting address is rejected",
			genState: &types.GenesisState{
				Params: types.NewParams("notanaddress", "", "uGNOD", 35_000_000_000_000_000, "2025-07-22", 12),
			},
			valid: false,
		},
		{
			desc: "empty denom is rejected",
			genState: &types.GenesisState{
				Params: types.NewParams("", "", "", 35_000_000_000_000_000, "2025-07-22", 12),
			},
			valid: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
