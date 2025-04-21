package eth

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/zena"
	"github.com/zenanetwork/go-zenanet/consensus/zena/clerk"
	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/checkpoint"
	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/milestone"
	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/span"
)

type mockIris struct {
	fetchCheckpoint         func(ctx context.Context, number int64) (*checkpoint.Checkpoint, error)
	fetchCheckpointCount    func(ctx context.Context) (int64, error)
	fetchMilestone          func(ctx context.Context) (*milestone.Milestone, error)
	fetchMilestoneCount     func(ctx context.Context) (int64, error)
	fetchNoAckMilestone     func(ctx context.Context, milestoneID string) error
	fetchLastNoAckMilestone func(ctx context.Context) (string, error)
}

func (m *mockIris) StateSyncEvents(ctx context.Context, fromID uint64, to int64) ([]*clerk.EventRecordWithTime, error) {
	return nil, nil
}
func (m *mockIris) Span(ctx context.Context, spanID uint64) (*span.IrisSpan, error) {
	//nolint:nilnil
	return nil, nil
}
func (m *mockIris) FetchCheckpoint(ctx context.Context, number int64) (*checkpoint.Checkpoint, error) {
	return m.fetchCheckpoint(ctx, number)
}
func (m *mockIris) FetchCheckpointCount(ctx context.Context) (int64, error) {
	return m.fetchCheckpointCount(ctx)
}
func (m *mockIris) FetchMilestone(ctx context.Context) (*milestone.Milestone, error) {
	return m.fetchMilestone(ctx)
}
func (m *mockIris) FetchMilestoneCount(ctx context.Context) (int64, error) {
	return m.fetchMilestoneCount(ctx)
}
func (m *mockIris) FetchNoAckMilestone(ctx context.Context, milestoneID string) error {
	return m.fetchNoAckMilestone(ctx, milestoneID)
}
func (m *mockIris) FetchLastNoAckMilestone(ctx context.Context) (string, error) {
	return m.fetchLastNoAckMilestone(ctx)
}

func (m *mockIris) FetchMilestoneID(ctx context.Context, milestoneID string) error {
	return m.fetchNoAckMilestone(ctx, milestoneID)
}

func (m *mockIris) Close() {}

func TestFetchWhitelistCheckpointAndMilestone(t *testing.T) {
	t.Parallel()

	// create an empty ethHandler
	handler := &ethHandler{}

	// create a mock checkpoint verification function and use it to create a verifier
	verify := func(ctx context.Context, eth *Zenanet, handler *ethHandler, start uint64, end uint64, hash string, isCheckpoint bool) (string, error) {
		return "", nil
	}

	verifier := newZenaVerifier()
	verifier.setVerify(verify)

	// Create a mock iris instance and use it for creating a zena instance
	var iris mockIris

	zena := &zena.Zena{IrisClient: &iris}

	fetchCheckpointTest(t, &iris, zena, handler, verifier)
	fetchMilestoneTest(t, &iris, zena, handler, verifier)
}

func (b *zenaVerifier) setVerify(verifyFn func(ctx context.Context, eth *Zenanet, handler *ethHandler, start uint64, end uint64, hash string, isCheckpoint bool) (string, error)) {
	b.verify = verifyFn
}

func fetchCheckpointTest(t *testing.T, iris *mockIris, zena *zena.Zena, handler *ethHandler, verifier *zenaVerifier) {
	t.Helper()

	var checkpoints []*checkpoint.Checkpoint
	// create a mock fetch checkpoint function
	iris.fetchCheckpoint = func(_ context.Context, number int64) (*checkpoint.Checkpoint, error) {
		if len(checkpoints) == 0 {
			return nil, errCheckpoint
		} else if number == -1 {
			return checkpoints[len(checkpoints)-1], nil
		} else {
			return checkpoints[number-1], nil
		}
	}

	// create a background context
	ctx := context.Background()

	_, _, err := handler.fetchWhitelistCheckpoint(ctx, zena, nil, verifier)
	require.ErrorIs(t, err, errCheckpoint)

	// create 4 mock checkpoints
	checkpoints = createMockCheckpoints(4)

	blockNum, blockHash, err := handler.fetchWhitelistCheckpoint(ctx, zena, nil, verifier)

	// Check if we have expected result
	require.Equal(t, err, nil)
	require.Equal(t, checkpoints[len(checkpoints)-1].EndBlock.Uint64(), blockNum)
	require.Equal(t, checkpoints[len(checkpoints)-1].RootHash, blockHash)
}

func fetchMilestoneTest(t *testing.T, iris *mockIris, zena *zena.Zena, handler *ethHandler, verifier *zenaVerifier) {
	t.Helper()

	var milestones []*milestone.Milestone
	// create a mock fetch checkpoint function
	iris.fetchMilestone = func(_ context.Context) (*milestone.Milestone, error) {
		if len(milestones) == 0 {
			return nil, errMilestone
		} else {
			return milestones[len(milestones)-1], nil
		}
	}

	// create a background context
	ctx := context.Background()

	_, _, err := handler.fetchWhitelistMilestone(ctx, zena, nil, verifier)
	require.ErrorIs(t, err, errMilestone)

	// create 4 mock checkpoints
	milestones = createMockMilestones(4)

	num, hash, err := handler.fetchWhitelistMilestone(ctx, zena, nil, verifier)

	// Check if we have expected result
	require.Equal(t, err, nil)
	require.Equal(t, milestones[len(milestones)-1].EndBlock.Uint64(), num)
	require.Equal(t, milestones[len(milestones)-1].Hash, hash)
}

func createMockCheckpoints(count int) []*checkpoint.Checkpoint {
	var (
		checkpoints []*checkpoint.Checkpoint = make([]*checkpoint.Checkpoint, count)
		startBlock  int64                    = 257 // any number can be used
	)

	for i := 0; i < count; i++ {
		checkpoints[i] = &checkpoint.Checkpoint{
			Proposer:    common.Address{},
			StartBlock:  big.NewInt(startBlock),
			EndBlock:    big.NewInt(startBlock + 255),
			RootHash:    common.Hash{},
			ZenaChainID: "137",
			Timestamp:   uint64(time.Now().Unix()),
		}
		startBlock += 256
	}

	return checkpoints
}

func createMockMilestones(count int) []*milestone.Milestone {
	var (
		milestones []*milestone.Milestone = make([]*milestone.Milestone, count)
		startBlock int64                  = 257 // any number can be used
	)

	for i := 0; i < count; i++ {
		milestones[i] = &milestone.Milestone{
			Proposer:    common.Address{},
			StartBlock:  big.NewInt(startBlock),
			EndBlock:    big.NewInt(startBlock + 255),
			Hash:        common.Hash{},
			ZenaChainID: "137",
			Timestamp:   uint64(time.Now().Unix()),
		}
		startBlock += 256
	}

	return milestones
}
