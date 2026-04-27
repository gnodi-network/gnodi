package keeper

import (
	"context"
	"time"

	errorsmod "cosmossdk.io/errors"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/gnodi-network/gnodi/x/distro/types"
)

func (k msgServer) Mint(goCtx context.Context, msg *types.MsgMint) (*types.MsgMintResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	signerBytes, err := k.addressCodec.StringToBytes(msg.Signer)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid signer address")
	}

	params, err := k.Params.Get(goCtx)
	if err != nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrNotFound, "module params not initialized")
	}

	if !k.IsAuthorized(params, signerBytes) {
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
	err = k.bankKeeper.MintCoins(ctx, types.ModuleName, coins)
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
	acctBytes, err := k.addressCodec.StringToBytes(toAddress)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address '%s'", toAddress)
	}

	coins := sdk.NewCoins(sdk.NewCoin(denom, math.NewIntFromUint64(amount)))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sdk.AccAddress(acctBytes), coins); err != nil {
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
		totalDistributable += halvingPeriodLimit(params.MaxSupply, period)
	}

	if currentHalvingPeriod > 0 {
		periodStart := addMonths(startDate, int((currentHalvingPeriod-1)*params.MonthsInHalvingPeriod))
		periodEnd := addMonths(startDate, int(currentHalvingPeriod*params.MonthsInHalvingPeriod)).AddDate(0, 0, -1)

		daysInPeriod := uint64(periodEnd.Sub(periodStart).Hours()/24) + 1
		periodYearlyLimit := halvingPeriodLimit(params.MaxSupply, currentHalvingPeriod)

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

// halvingPeriodLimit returns the total distributable tokens for the given halving period.
// Guards against a division-by-zero: uint64(1) << n is 0 for n >= 64 in Go, which
// would cause a runtime panic. Period 65+ would require shifting by >= 64 bits; in
// practice all supply is exhausted well before that point, so the limit is 0.
func halvingPeriodLimit(maxSupply, period uint64) uint64 {
	if period == 0 || period > 64 {
		return 0
	}
	return maxSupply / (uint64(1) << (period - 1)) / 2
}

// addMonths adds n calendar months to t, clamping the day to the last day of
// the resulting month rather than overflowing into the next month the way
// time.AddDate does. This ensures period boundary calculations are consistent
// with the monthsBetween period-index calculation for month-end start dates.
func addMonths(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	targetMonth := int(m) + months
	targetYear := y + (targetMonth-1)/12
	targetMonth = ((targetMonth - 1) % 12) + 1
	if targetMonth < 1 {
		targetMonth += 12
		targetYear--
	}
	// Clamp day to last day of target month.
	lastDay := time.Date(targetYear, time.Month(targetMonth)+1, 0, 0, 0, 0, 0, t.Location()).Day()
	if d > lastDay {
		d = lastDay
	}
	return time.Date(targetYear, time.Month(targetMonth), d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

func monthsBetween(start, end time.Time) int {
	if end.Before(start) {
		return -1
	}

	years := end.Year() - start.Year()
	months := years*12 + int(end.Month()) - int(start.Month())

	// Adjust for day of month. If end's day is less than start's day, a full
	// month hasn't elapsed yet — unless end is the last day of its month,
	// which means start's day simply doesn't exist in end's month (e.g.
	// Jan 31 → Feb 28: Feb has no 31st, so Feb 28 is the month boundary).
	// This is consistent with addMonths() clamping: addMonths(Jan31, 1) = Feb28.
	if end.Day() < start.Day() {
		endIsLastDay := end.Day() == time.Date(end.Year(), end.Month()+1, 0, 0, 0, 0, 0, end.Location()).Day()
		if !endIsLastDay {
			months--
		}
	}

	if months < 0 {
		return 0
	}
	return months
}
