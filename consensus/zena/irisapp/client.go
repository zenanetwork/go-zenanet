package irisapp

import (
	"github.com/cosmos/cosmos-sdk/types"

	"github.com/zenanetwork/go-zenanet/log"

	"github.com/zenanetwork/iris/app"
	"github.com/zenanetwork/iris/cmd/irisd/service"

	abci "github.com/tendermint/tendermint/abci/types"
)

const (
	stateFetchLimit = 50
)

type IrisAppClient struct {
	hApp *app.IrisApp
}

func NewIrisAppClient() *IrisAppClient {
	return &IrisAppClient{
		hApp: service.GetIrisApp(),
	}
}

func (h *IrisAppClient) Close() {
	// Nothing to close as of now
	log.Warn("Shutdown detected, Closing Iris App conn")
}

func (h *IrisAppClient) NewContext() types.Context {
	return h.hApp.NewContext(true, abci.Header{Height: h.hApp.LastBlockHeight()})
}
