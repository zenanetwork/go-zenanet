package zena

import (
	"math/big"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/zena/clerk"
	"github.com/zenanetwork/go-zenanet/consensus/zena/statefull"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
)

//go:generate mockgen -destination=./genesis_contract_mock.go -package=zena . GenesisContract
type GenesisContract interface {
	CommitState(event *clerk.EventRecordWithTime, state *state.StateDB, header *types.Header, chCtx statefull.ChainContext) (uint64, error)
	LastStateId(state *state.StateDB, number uint64, hash common.Hash) (*big.Int, error)
}
