package filters

import (
	"context"
	"math/big"
	"testing"

	"github.com/zenanetwork/go-zenanet/common"
	types "github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/crypto"
	"github.com/zenanetwork/go-zenanet/params"

	gomock "github.com/golang/mock/gomock"
)

var (
	key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	addr    = crypto.PubkeyToAddress(key1.PublicKey)
)

func newTestHeader(blockNumber uint) *types.Header {
	return &types.Header{
		Number: big.NewInt(int64(blockNumber)),
	}
}

func newTestReceipt(contractAddr common.Address, topicAddress common.Hash) *types.Receipt {
	receipt := types.NewReceipt(nil, false, 0)
	receipt.Logs = []*types.Log{
		{
			Address: contractAddr,
			Topics:  []common.Hash{topicAddress},
		},
	}

	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return receipt
}

func (backend *MockBackend) expectZenaReceiptsFromMock(hashes []*common.Hash) {
	for _, h := range hashes {
		if h == nil {
			backend.EXPECT().GetZenaBlockReceipt(gomock.Any(), gomock.Any()).Return(nil, nil)
			continue
		}

		backend.EXPECT().GetZenaBlockReceipt(gomock.Any(), gomock.Any()).Return(newTestReceipt(addr, *h), nil)
	}
}

func TestZenaFilters(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		hash1 = common.BytesToHash([]byte("topic1"))
		hash2 = common.BytesToHash([]byte("topic2"))
		hash3 = common.BytesToHash([]byte("topic3"))
		hash4 = common.BytesToHash([]byte("topic4"))
		db    = NewMockDatabase(ctrl)

		testZenaConfig = params.TestChainConfig.Zena
	)

	backend := NewMockBackend(ctrl)

	// should return the following at all times
	backend.EXPECT().ChainDb().Return(db).AnyTimes()
	backend.EXPECT().HeaderByNumber(gomock.Any(), gomock.Any()).Return(newTestHeader(1), nil).AnyTimes()

	// Block 1
	backend.expectZenaReceiptsFromMock([]*common.Hash{nil, &hash1, &hash2, &hash3, &hash4})

	filter := NewZenaBlockLogsRangeFilter(backend, testZenaConfig, 0, 18, []common.Address{addr}, [][]common.Hash{{hash1, hash2, hash3, hash4}})
	logs, err := filter.Logs(context.Background())

	if err != nil {
		t.Error(err)
	}

	if len(logs) != 4 {
		t.Error("expected 4 log, got", len(logs))
	}

	// Block 2
	backend.expectZenaReceiptsFromMock([]*common.Hash{&hash1, &hash3})

	filter = NewZenaBlockLogsRangeFilter(backend, testZenaConfig, 990, 999, []common.Address{addr}, [][]common.Hash{{hash3}})
	logs, _ = filter.Logs(context.Background())

	if len(logs) != 1 {
		t.Error("expected 1 log, got", len(logs))
	}

	if len(logs) > 0 && logs[0].Topics[0] != hash3 {
		t.Errorf("expected log[0].Topics[0] to be %x, got %x", hash3, logs[0].Topics[0])
	}

	// Block 3
	backend.expectZenaReceiptsFromMock([]*common.Hash{&hash1, &hash2, &hash3})

	filter = NewZenaBlockLogsRangeFilter(backend, testZenaConfig, 992, 1000, []common.Address{addr}, [][]common.Hash{{hash3}})
	logs, _ = filter.Logs(context.Background())

	if len(logs) != 1 {
		t.Error("expected 1 log, got", len(logs))
	}

	if len(logs) > 0 && logs[0].Topics[0] != hash3 {
		t.Errorf("expected log[0].Topics[0] to be %x, got %x", hash3, logs[0].Topics[0])
	}

	// Block 4
	backend.expectZenaReceiptsFromMock([]*common.Hash{&hash1, &hash2, nil, &hash3})

	filter = NewZenaBlockLogsRangeFilter(backend, testZenaConfig, 1, 16, []common.Address{addr}, [][]common.Hash{{hash1, hash2}})

	logs, _ = filter.Logs(context.Background())

	if len(logs) != 2 {
		t.Error("expected 2 log, got", len(logs))
	}

	// Block 5
	backend.expectZenaReceiptsFromMock([]*common.Hash{&hash1, &hash2, nil, &hash3, &hash4, nil})

	failHash := common.BytesToHash([]byte("fail"))
	filter = NewZenaBlockLogsRangeFilter(backend, testZenaConfig, 0, 20, nil, [][]common.Hash{{failHash}})

	logs, _ = filter.Logs(context.Background())
	if len(logs) != 0 {
		t.Error("expected 0 log, got", len(logs))
	}

	// Block 6
	backend.expectZenaReceiptsFromMock([]*common.Hash{&hash1, &hash2, nil, &hash3, &hash4, nil})

	failAddr := common.BytesToAddress([]byte("failmenow"))
	filter = NewZenaBlockLogsRangeFilter(backend, testZenaConfig, 0, 20, []common.Address{failAddr}, nil)

	logs, _ = filter.Logs(context.Background())
	if len(logs) != 0 {
		t.Error("expected 0 log, got", len(logs))
	}

	// Block 7
	backend.expectZenaReceiptsFromMock([]*common.Hash{&hash1, &hash2, nil, &hash3, &hash4, nil})

	filter = NewZenaBlockLogsRangeFilter(backend, testZenaConfig, 0, 20, nil, [][]common.Hash{{failHash}, {hash1}})

	logs, _ = filter.Logs(context.Background())
	if len(logs) != 0 {
		t.Error("expected 0 log, got", len(logs))
	}
}
