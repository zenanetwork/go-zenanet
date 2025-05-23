package span

import (
	"github.com/zenanetwork/go-zenanet/consensus/zena/valset"
)

// Span Zena represents a current zena span
type Span struct {
	ID         uint64 `json:"span_id" yaml:"span_id"`
	StartBlock uint64 `json:"start_block" yaml:"start_block"`
	EndBlock   uint64 `json:"end_block" yaml:"end_block"`
}

// IrisSpan represents span from iris APIs
type IrisSpan struct {
	Span
	ValidatorSet      valset.ValidatorSet `json:"validator_set" yaml:"validator_set"`
	SelectedProducers []valset.Validator  `json:"selected_producers" yaml:"selected_producers"`
	ChainID           string              `json:"zena_chain_id" yaml:"zena_chain_id"`
}
