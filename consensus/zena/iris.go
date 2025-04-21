package zena

import (
	"context"

	"github.com/zenanetwork/go-zenanet/consensus/zena/clerk"
	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/checkpoint"
	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/milestone"
	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/span"
)

//go:generate mockgen -destination=../../tests/zena/mocks/IIrisClient.go -package=mocks . IIrisClient
type IIrisClient interface {
	StateSyncEvents(ctx context.Context, fromID uint64, to int64) ([]*clerk.EventRecordWithTime, error)
	Span(ctx context.Context, spanID uint64) (*span.IrisSpan, error)
	FetchCheckpoint(ctx context.Context, number int64) (*checkpoint.Checkpoint, error)
	FetchCheckpointCount(ctx context.Context) (int64, error)
	FetchMilestone(ctx context.Context) (*milestone.Milestone, error)
	FetchMilestoneCount(ctx context.Context) (int64, error)
	FetchNoAckMilestone(ctx context.Context, milestoneID string) error // Fetch the bool value whether milestone corresponding to the given id failed in the Iris
	FetchLastNoAckMilestone(ctx context.Context) (string, error)       // Fetch latest failed milestone id
	FetchMilestoneID(ctx context.Context, milestoneID string) error    // Fetch the bool value whether milestone corresponding to the given id is in process in Iris
	Close()
}
