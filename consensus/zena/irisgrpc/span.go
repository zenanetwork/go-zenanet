package irisgrpc

import (
	"context"

	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/span"
	"github.com/zenanetwork/go-zenanet/consensus/zena/valset"
	"github.com/zenanetwork/go-zenanet/log"

	proto "github.com/zenanetwork/zenaproto/iris"
	protoutils "github.com/zenanetwork/zenaproto/utils"
)

func (h *IrisGRPCClient) Span(ctx context.Context, spanID uint64) (*span.IrisSpan, error) {
	req := &proto.SpanRequest{
		ID: spanID,
	}

	log.Info("Fetching span", "spanID", spanID)

	res, err := h.client.Span(ctx, req)
	if err != nil {
		return nil, err
	}

	log.Info("Fetched span", "spanID", spanID)

	return parseSpan(res.Result), nil
}

func parseSpan(protoSpan *proto.Span) *span.IrisSpan {
	resp := &span.IrisSpan{
		Span: span.Span{
			ID:         protoSpan.ID,
			StartBlock: protoSpan.StartBlock,
			EndBlock:   protoSpan.EndBlock,
		},
		ValidatorSet:      valset.ValidatorSet{},
		SelectedProducers: []valset.Validator{},
		ChainID:           protoSpan.ChainID,
	}

	for _, validator := range protoSpan.ValidatorSet.Validators {
		resp.ValidatorSet.Validators = append(resp.ValidatorSet.Validators, parseValidator(validator))
	}

	resp.ValidatorSet.Proposer = parseValidator(protoSpan.ValidatorSet.Proposer)

	for _, validator := range protoSpan.SelectedProducers {
		resp.SelectedProducers = append(resp.SelectedProducers, *parseValidator(validator))
	}

	return resp
}

func parseValidator(validator *proto.Validator) *valset.Validator {
	return &valset.Validator{
		ID:               validator.ID,
		Address:          protoutils.ConvertH160toAddress(validator.Address),
		VotingPower:      validator.VotingPower,
		ProposerPriority: validator.ProposerPriority,
	}
}
