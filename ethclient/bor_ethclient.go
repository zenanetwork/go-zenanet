package ethclient

import (
	"context"

	zenanet "github.com/zenanetwork/go-zenanet"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
)

const (
	zeroAddress = "0x0000000000000000000000000000000000000000"
)

// GetRootHash returns the merkle root of the block headers
func (ec *Client) GetRootHash(ctx context.Context, startBlockNumber uint64, endBlockNumber uint64) (string, error) {
	var rootHash string
	if err := ec.c.CallContext(ctx, &rootHash, "zena_getRootHash", startBlockNumber, endBlockNumber); err != nil {
		return "", err
	}

	return rootHash, nil
}

// GetRootHash returns the merkle root of the block headers
func (ec *Client) GetVoteOnHash(ctx context.Context, startBlockNumber uint64, endBlockNumber uint64, hash string, milestoneID string) (bool, error) {
	var value bool
	if err := ec.c.CallContext(ctx, &value, "zena_getVoteOnHash", startBlockNumber, endBlockNumber, hash, milestoneID); err != nil {
		return false, err
	}

	return value, nil
}

// GetZenaBlockReceipt returns zena block receipt
func (ec *Client) GetZenaBlockReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	var r *types.Receipt

	err := ec.c.CallContext(ctx, &r, "eth_getZenaBlockReceipt", hash)
	if err == nil && r == nil {
		return nil, zenanet.NotFound
	}

	return r, err
}
