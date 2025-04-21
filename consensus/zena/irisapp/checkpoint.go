package irisapp

import (
	"context"
	"math/big"

	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/checkpoint"
	"github.com/zenanetwork/go-zenanet/log"

	hmTypes "github.com/zenanetwork/iris/types"
)

func (h *IrisAppClient) FetchCheckpointCount(_ context.Context) (int64, error) {
	log.Info("Fetching checkpoint count")

	res := h.hApp.CheckpointKeeper.GetACKCount(h.NewContext())

	log.Info("Fetched checkpoint count")

	return int64(res), nil
}

func (h *IrisAppClient) FetchCheckpoint(_ context.Context, number int64) (*checkpoint.Checkpoint, error) {
	log.Info("Fetching checkpoint", "number", number)

	res, err := h.hApp.CheckpointKeeper.GetCheckpointByNumber(h.NewContext(), uint64(number))
	if err != nil {
		return nil, err
	}

	log.Info("Fetched checkpoint", "number", number)

	return toZenaCheckpoint(res), nil
}

func toZenaCheckpoint(hdCheckpoint hmTypes.Checkpoint) *checkpoint.Checkpoint {
	return &checkpoint.Checkpoint{
		Proposer:    hdCheckpoint.Proposer.EthAddress(),
		StartBlock:  big.NewInt(int64(hdCheckpoint.StartBlock)),
		EndBlock:    big.NewInt(int64(hdCheckpoint.EndBlock)),
		RootHash:    hdCheckpoint.RootHash.EthHash(),
		ZenaChainID: hdCheckpoint.ZenaChainID,
		Timestamp:   hdCheckpoint.TimeStamp,
	}
}
