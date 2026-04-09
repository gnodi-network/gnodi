package keeper

import (
	"bytes"

	"github.com/gnodi-network/gnodi/x/distro/types"
)

// IsAuthorized checks if the sender is authorized to mint.
// Both addresses are decoded to bytes before comparison so that equivalent
// EVM hex and bech32 representations of the same key are treated as equal.
func (k Keeper) IsAuthorized(params types.Params, signerBytes []byte) bool {
	mintingBytes, err := k.addressCodec.StringToBytes(params.MintingAddress)
	if err != nil {
		return false
	}
	return bytes.Equal(mintingBytes, signerBytes)
}
