package types

import "github.com/zenanetwork/go-zenanet/common"

// StateSyncData represents state received from Zenanet Blockchain
type StateSyncData struct {
	ID       uint64
	Contract common.Address
	Data     string
	TxHash   common.Hash
}
