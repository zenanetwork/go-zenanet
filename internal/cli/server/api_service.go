package server

import (
	"context"
	"errors"

	"github.com/zenanetwork/go-zenanet/common/hexutil"
	"github.com/zenanetwork/go-zenanet/common/math"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/rpc"

	protoutil "github.com/zenanetwork/zenaproto/utils"
	protozena "github.com/zenanetwork/zenaproto/zena"
)

func (s *Server) GetRootHash(ctx context.Context, req *protozena.GetRootHashRequest) (*protozena.GetRootHashResponse, error) {
	rootHash, err := s.backend.APIBackend.GetRootHash(ctx, req.StartBlockNumber, req.EndBlockNumber)
	if err != nil {
		return nil, err
	}

	return &protozena.GetRootHashResponse{RootHash: rootHash}, nil
}

func (s *Server) GetVoteOnHash(ctx context.Context, req *protozena.GetVoteOnHashRequest) (*protozena.GetVoteOnHashResponse, error) {
	vote, err := s.backend.APIBackend.GetVoteOnHash(ctx, req.StartBlockNumber, req.EndBlockNumber, req.Hash, req.MilestoneId)
	if err != nil {
		return nil, err
	}

	return &protozena.GetVoteOnHashResponse{Response: vote}, nil
}

func headerToProtozenaHeader(h *types.Header) *protozena.Header {
	return &protozena.Header{
		Number:     h.Number.Uint64(),
		ParentHash: protoutil.ConvertHashToH256(h.ParentHash),
		Time:       h.Time,
	}
}

func (s *Server) HeaderByNumber(ctx context.Context, req *protozena.GetHeaderByNumberRequest) (*protozena.GetHeaderByNumberResponse, error) {
	bN, err := getRpcBlockNumberFromString(req.Number)
	if err != nil {
		return nil, err
	}
	header, err := s.backend.APIBackend.HeaderByNumber(ctx, bN)
	if err != nil {
		return nil, err
	}

	if header == nil {
		return nil, errors.New("header not found")
	}

	return &protozena.GetHeaderByNumberResponse{Header: headerToProtozenaHeader(header)}, nil
}

func (s *Server) BlockByNumber(ctx context.Context, req *protozena.GetBlockByNumberRequest) (*protozena.GetBlockByNumberResponse, error) {
	bN, err := getRpcBlockNumberFromString(req.Number)
	if err != nil {
		return nil, err
	}
	block, err := s.backend.APIBackend.BlockByNumber(ctx, bN)
	if err != nil {
		return nil, err
	}

	if block == nil {
		return nil, errors.New("block not found")
	}

	return &protozena.GetBlockByNumberResponse{Block: blockToProtoBlock(block)}, nil
}

func blockToProtoBlock(h *types.Block) *protozena.Block {
	return &protozena.Block{
		Header: headerToProtozenaHeader(h.Header()),
	}
}

func (s *Server) TransactionReceipt(ctx context.Context, req *protozena.ReceiptRequest) (*protozena.ReceiptResponse, error) {
	_, _, blockHash, _, txnIndex, err := s.backend.APIBackend.GetTransaction(ctx, protoutil.ConvertH256ToHash(req.Hash))
	if err != nil {
		return nil, err
	}

	receipts, err := s.backend.APIBackend.GetReceipts(ctx, blockHash)
	if err != nil {
		return nil, err
	}

	if receipts == nil {
		return nil, errors.New("no receipts found")
	}

	if len(receipts) <= int(txnIndex) {
		return nil, errors.New("transaction index out of bounds")
	}

	return &protozena.ReceiptResponse{Receipt: ConvertReceiptToProtoReceipt(receipts[txnIndex])}, nil
}

func (s *Server) BorBlockReceipt(ctx context.Context, req *protozena.ReceiptRequest) (*protozena.ReceiptResponse, error) {
	receipt, err := s.backend.APIBackend.GetZenaBlockReceipt(ctx, protoutil.ConvertH256ToHash(req.Hash))
	if err != nil {
		return nil, err
	}

	return &protozena.ReceiptResponse{Receipt: ConvertReceiptToProtoReceipt(receipt)}, nil
}

func getRpcBlockNumberFromString(blockNumber string) (rpc.BlockNumber, error) {
	switch blockNumber {
	case "latest":
		return rpc.LatestBlockNumber, nil
	case "earliest":
		return rpc.EarliestBlockNumber, nil
	case "pending":
		return rpc.PendingBlockNumber, nil
	case "finalized":
		return rpc.FinalizedBlockNumber, nil
	case "safe":
		return rpc.SafeBlockNumber, nil
	default:
		blckNum, err := hexutil.DecodeUint64(blockNumber)
		if err != nil {
			return rpc.BlockNumber(0), errors.New("invalid block number")
		}
		if blckNum > math.MaxInt64 {
			return rpc.BlockNumber(0), errors.New("block number out of range")
		}
		return rpc.BlockNumber(blckNum), nil
	}
}
