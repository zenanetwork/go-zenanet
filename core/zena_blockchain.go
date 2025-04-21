package core

import (
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
	"github.com/zenanetwork/go-zenanet/core/types"
)

// GetZenaReceiptByHash retrieves the zena block receipt in a given block.
func (bc *BlockChain) GetZenaReceiptByHash(hash common.Hash) *types.Receipt {
	if receipt, ok := bc.zenaReceiptsCache.Get(hash); ok {
		return receipt
	}

	// read header from hash
	number := rawdb.ReadHeaderNumber(bc.db, hash)
	if number == nil {
		return nil
	}

	// read zena receipt by hash and number
	receipt := rawdb.ReadZenaReceipt(bc.db, hash, *number, bc.chainConfig)
	if receipt == nil {
		return nil
	}

	// add into zena receipt cache
	bc.zenaReceiptsCache.Add(hash, receipt)

	return receipt
}
