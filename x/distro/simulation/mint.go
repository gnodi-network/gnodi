package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"gnodi/x/distro/keeper"
	"gnodi/x/distro/types"
)

func SimulateMsgMint(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
	txGen client.TxConfig,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgMint{
			Signer: simAccount.Address.String(),
			Amount: uint64(r.Intn(100000000)),
		}

		// TODO: Handle the Mint simulation

		return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "Mint simulation not implemented"), nil, nil
	}
}
