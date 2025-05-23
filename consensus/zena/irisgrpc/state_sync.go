package irisgrpc

import (
	"context"
	"errors"
	"io"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/zena/clerk"

	proto "github.com/zenanetwork/zenaproto/iris"
)

func (h *IrisGRPCClient) StateSyncEvents(ctx context.Context, fromID uint64, to int64) ([]*clerk.EventRecordWithTime, error) {
	eventRecords := make([]*clerk.EventRecordWithTime, 0)

	req := &proto.StateSyncEventsRequest{
		FromID: fromID,
		ToTime: uint64(to),
		Limit:  uint64(stateFetchLimit),
	}

	var (
		res    proto.Iris_StateSyncEventsClient
		events *proto.StateSyncEventsResponse
		err    error
	)

	res, err = h.client.StateSyncEvents(ctx, req)
	if err != nil {
		return nil, err
	}

	for {
		events, err = res.Recv()
		if errors.Is(err, io.EOF) {
			return eventRecords, nil
		}

		if err != nil {
			return nil, err
		}

		for _, event := range events.Result {
			eventRecord := &clerk.EventRecordWithTime{
				EventRecord: clerk.EventRecord{
					ID:       event.ID,
					Contract: common.HexToAddress(event.Contract),
					Data:     common.Hex2Bytes(event.Data[2:]),
					TxHash:   common.HexToHash(event.TxHash),
					LogIndex: event.LogIndex,
					ChainID:  event.ChainID,
				},
				Time: event.Time.AsTime(),
			}
			eventRecords = append(eventRecords, eventRecord)
		}
	}
}
