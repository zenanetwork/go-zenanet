package irisgrpc

import (
	"context"
	"math/big"

	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/checkpoint"
	"github.com/zenanetwork/go-zenanet/log"

	proto "github.com/zenanetwork/zenaproto/iris"
	protoutils "github.com/zenanetwork/zenaproto/utils"
)

func (h *IrisGRPCClient) FetchCheckpointCount(ctx context.Context) (int64, error) {
	log.Info("Fetching checkpoint count")

	res, err := h.client.FetchCheckpointCount(ctx, nil)
	if err != nil {
		return 0, err
	}

	log.Info("Fetched checkpoint count")

	return res.Result.Result, nil
}

func (h *IrisGRPCClient) FetchCheckpoint(ctx context.Context, number int64) (*checkpoint.Checkpoint, error) {
	req := &proto.FetchCheckpointRequest{
		ID: number,
	}

	log.Info("Fetching checkpoint", "number", number)

	res, err := h.client.FetchCheckpoint(ctx, req)
	if err != nil {
		return nil, err
	}

	log.Info("Fetched checkpoint", "number", number)

	checkpoint := &checkpoint.Checkpoint{
		StartBlock:  new(big.Int).SetUint64(res.Result.StartBlock),
		EndBlock:    new(big.Int).SetUint64(res.Result.EndBlock),
		RootHash:    protoutils.ConvertH256ToHash(res.Result.RootHash),
		Proposer:    protoutils.ConvertH160toAddress(res.Result.Proposer),
		ZenaChainID: res.Result.ZenaChainID,
		Timestamp:   uint64(res.Result.Timestamp.GetSeconds()),
	}

	return checkpoint, nil
}
