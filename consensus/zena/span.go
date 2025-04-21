package zena

import (
	"context"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/span"
	"github.com/zenanetwork/go-zenanet/consensus/zena/valset"
	"github.com/zenanetwork/go-zenanet/core"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/rpc"
)

//go:generate mockgen -destination=./span_mock.go -package=zena . Spanner
type Spanner interface {
	GetCurrentSpan(ctx context.Context, headerHash common.Hash) (*span.Span, error)
	GetCurrentValidatorsByHash(ctx context.Context, headerHash common.Hash, blockNumber uint64) ([]*valset.Validator, error)
	GetCurrentValidatorsByBlockNrOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash, blockNumber uint64) ([]*valset.Validator, error)
	CommitSpan(ctx context.Context, irisSpan span.IrisSpan, state *state.StateDB, header *types.Header, chainContext core.ChainContext) error
}
