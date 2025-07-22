package types

const DefaultMinitingAddress string = ""
const DefaultReceivingAddress string = ""
const DefaultDenom string = "stake"
const DefaultMaxSupply uint64 = 30000000000000000
const DefaultDistributionStartDate string = "2025-07-22"
const DefaultMonthsInHalvingPeriod uint64 = 12

// NewParams creates a new Params instance.
func NewParams(
	minting_address string,
	receiving_address string,
	denom string,
	max_supply uint64,
	distribution_start_date string,
	months_in_halving_period uint64) Params {
	return Params{
		MintingAddress:        minting_address,
		ReceivingAddress:      receiving_address,
		Denom:                 denom,
		MaxSupply:             max_supply,
		DistributionStartDate: distribution_start_date,
		MonthsInHalvingPeriod: months_in_halving_period,
	}
}

func DefaultParams() Params {
	return NewParams(DefaultMinitingAddress, DefaultReceivingAddress, DefaultDenom, DefaultMaxSupply, DefaultDistributionStartDate, DefaultMonthsInHalvingPeriod)
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if err := validateMinitingAddress(p.MintingAddress); err != nil {
		return err
	}
	if err := validateReceivingAddress(p.ReceivingAddress); err != nil {
		return err
	}
	if err := validateDenom(p.Denom); err != nil {
		return err
	}
	if err := validateMaxSupply(p.MaxSupply); err != nil {
		return err
	}
	if err := validateDistributionStartDate(p.DistributionStartDate); err != nil {
		return err
	}
	if err := validateMonthsInHalvingPeriod(p.MonthsInHalvingPeriod); err != nil {
		return err
	}

	return nil
}
func validateMinitingAddress(v string) error {
	return nil
}
func validateReceivingAddress(v string) error {

	return nil
}
func validateDenom(v string) error {

	return nil
}
func validateMaxSupply(v uint64) error {

	return nil
}
func validateDistributionStartDate(v string) error {

	return nil
}
func validateMonthsInHalvingPeriod(v uint64) error {

	return nil
}
