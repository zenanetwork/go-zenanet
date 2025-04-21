package web3ext

// ZenaJs zena related apis
const ZenaJs = `
web3._extend({
	property: 'zena',
	methods: [
		new web3._extend.Method({
			name: 'getSnapshot',
			call: 'zena_getSnapshot',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'getAuthor',
			call: 'zena_getAuthor',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'getSnapshotProposer',
			call: 'zena_getSnapshotProposer',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'getSnapshotProposerSequence',
			call: 'zena_getSnapshotProposerSequence',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'getSnapshotAtHash',
			call: 'zena_getSnapshotAtHash',
			params: 1
		}),
		new web3._extend.Method({
			name: 'getSigners',
			call: 'zena_getSigners',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'getSignersAtHash',
			call: 'zena_getSignersAtHash',
			params: 1
		}),
		new web3._extend.Method({
			name: 'getCurrentProposer',
			call: 'zena_getCurrentProposer',
			params: 0
		}),
		new web3._extend.Method({
			name: 'getCurrentValidators',
			call: 'zena_getCurrentValidators',
			params: 0
		}),
		new web3._extend.Method({
			name: 'getRootHash',
			call: 'zena_getRootHash',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'getVoteOnHash',
			call: 'zena_getVoteOnHash',
			params: 4,
		}),
		new web3._extend.Method({
			name: 'sendRawTransactionConditional',
			call: 'zena_sendRawTransactionConditional',
			params: 2,
			inputFormatter: [null]
		}),
	]
});
`
