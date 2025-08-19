package types

func NewMsgMint(amount uint64, signer string) *MsgMint {
	return &MsgMint{
		Amount: amount,
		Signer: signer,
	}
}
