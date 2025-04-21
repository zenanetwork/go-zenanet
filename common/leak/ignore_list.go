package leak

import "go.uber.org/goleak"

func IgnoreList() []goleak.Option {
	return []goleak.Option{
		// a list of goroutne leaks that hard to fix due to external dependencies or too big refactoring needed
		goleak.IgnoreTopFunction("github.com/zenanetwork/go-zenanet/core.(*txSenderCacher).cache"),
		goleak.IgnoreTopFunction("github.com/rjeczalik/notify.(*recursiveTree).dispatch"),
		goleak.IgnoreTopFunction("github.com/rjeczalik/notify.(*recursiveTree).internal"),
		goleak.IgnoreTopFunction("github.com/rjeczalik/notify.(*nonrecursiveTree).dispatch"),
		goleak.IgnoreTopFunction("github.com/rjeczalik/notify.(*nonrecursiveTree).internal"),
		goleak.IgnoreTopFunction("github.com/rjeczalik/notify._Cfunc_CFRunLoopRun"),

		// todo: this leaks should be fixed
		goleak.IgnoreTopFunction("github.com/zenanetwork/go-zenanet/accounts/abi/bind/backends.nullSubscription.func1"),
		goleak.IgnoreTopFunction("github.com/zenanetwork/go-zenanet/accounts/abi/bind/backends.(*filterBackend).SubscribeNewTxsEvent.func1"),
		goleak.IgnoreTopFunction("github.com/zenanetwork/go-zenanet/accounts/abi/bind/backends.(*filterBackend).SubscribePendingLogsEvent.nullSubscription.func1"),
		goleak.IgnoreTopFunction("github.com/zenanetwork/go-zenanet/consensus/ethash.(*remoteSealer).loop"),
		goleak.IgnoreTopFunction("github.com/zenanetwork/go-zenanet/core.(*BlockChain).updateFutureBlocks"),
		goleak.IgnoreTopFunction("github.com/zenanetwork/go-zenanet/core/state/snapshot.(*diskLayer).generate"),
		goleak.IgnoreTopFunction("github.com/zenanetwork/go-zenanet/core/state.(*subfetcher).loop"),
		goleak.IgnoreTopFunction("github.com/zenanetwork/go-zenanet/eth/filters.(*EventSystem).eventLoop"),
		goleak.IgnoreTopFunction("github.com/zenanetwork/go-zenanet/event.NewSubscription.func1"),
		goleak.IgnoreTopFunction("github.com/zenanetwork/go-zenanet/event.NewSubscription"),
		goleak.IgnoreTopFunction("github.com/zenanetwork/go-zenanet/metrics.(*meterArbiter).tick"),
	}
}
