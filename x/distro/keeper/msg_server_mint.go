package keeper

import (
	"context"
	"time"

	"gnodi/x/distro/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (k msgServer) Mint(goCtx context.Context, msg *types.MsgMint) (*types.MsgMintResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if _, err := k.addressCodec.StringToBytes(msg.Signer); err != nil {
		return nil, errorsmod.Wrap(err, "invalid authority address")
	}

	signer := msg.GetSigner()

	if len(signer) == 0 {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "signer is required")
	}

	params := k.GetParams(ctx)

	if !k.IsAuthorized(params, signer) {
		return nil, errorsmod.Wrap(sdkerrors.ErrUnauthorized, "unauthorized sender")
	}

	currentSupply := math.NewUint(k.bankKeeper.GetSupply(ctx, params.Denom).Amount.Uint64())
	msgAmount := math.NewUint(msg.Amount)
	if currentSupply.Add(msgAmount).GT(math.NewUint(params.MaxSupply)) {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "max supply exceeded")
	}

	if err := validateMintingLimits(ctx, currentSupply, msgAmount, params); err != nil {
		return nil, err
	}

	coins := sdk.NewCoins(sdk.NewCoin(params.Denom, math.NewIntFromUint64(msgAmount.Uint64())))
	err := k.bankKeeper.MintCoins(ctx, types.ModuleName, coins)
	if err != nil {
		return nil, err
	}

	if err := k.depositCoins(ctx, params.ReceivingAddress, msg.Amount, params.Denom); err != nil {
		return nil, err
	}

	return &types.MsgMintResponse{}, nil
}

func parseDate(dateStr string) (time.Time, error) {
	return time.Parse("2006-01-02", dateStr)
}

func (k msgServer) depositCoins(ctx context.Context, toAddress string, amount uint64, denom string) error {
	acct, err := sdk.AccAddressFromBech32(toAddress)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address '%s'", toAddress)
	}

	coins := sdk.NewCoins(sdk.NewCoin(denom, math.NewIntFromUint64(amount)))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, acct, coins); err != nil {
		return err
	}
	return nil
}

func validateMintingLimits(ctx sdk.Context, currentSupply math.Uint, amount math.Uint, params types.Params) error {
	startDate, err := parseDate(params.DistributionStartDate)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "invalid distribution start date: %v", err)
	}
	targetDate, err := parseDate(ctx.BlockTime().Format("2006-01-02"))
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "invalid target date: %v", err)
	}

	months := monthsBetween(startDate, targetDate)
	if months < 0 {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "target date is before start date")
	}

	currentHalvingPeriod := 1 + uint64(months)/params.MonthsInHalvingPeriod
	var totalDistributable uint64

	for period := uint64(1); period < currentHalvingPeriod; period++ {
		periodYearlyLimit := params.MaxSupply / (1 << (period - 1)) / 2
		totalDistributable += periodYearlyLimit
	}

	if currentHalvingPeriod > 0 {
		periodStart := startDate.AddDate(0, int((currentHalvingPeriod-1)*params.MonthsInHalvingPeriod), 0)
		periodEnd := startDate.AddDate(0, int(currentHalvingPeriod*params.MonthsInHalvingPeriod), -1)

		daysInPeriod := uint64(periodEnd.Sub(periodStart).Hours()/24) + 1
		periodYearlyLimit := params.MaxSupply / (1 << (currentHalvingPeriod - 1)) / 2

		daysElapsed := uint64(targetDate.Sub(periodStart).Hours() / 24)
		if daysElapsed > daysInPeriod {
			daysElapsed = daysInPeriod
		}

		if daysInPeriod != 0 {
			currentPeriodAmount := (periodYearlyLimit * daysElapsed) / daysInPeriod
			totalDistributable += currentPeriodAmount
		}
	}

	if amount.Add(currentSupply).GT(math.NewUint(totalDistributable)) {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "amount exceeds total distributable limit of %d", totalDistributable)
	}

	return nil
}

func monthsBetween(start, end time.Time) int {
	if end.Before(start) {
		return -1
	}

	years := end.Year() - start.Year()
	months := years*12 + int(end.Month()) - int(start.Month())

	// Adjust for day of month
	if end.Day() < start.Day() {
		months--
	}

	// Handle edge case where end is exactly on start's day but in a prior month
	if months < 0 {
		return 0
	}
	return months
}
