// Copyright 2015 The go-zenanet Authors
// This file is part of the go-zenanet library.
//
// The go-zenanet library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-zenanet library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-zenanet library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"context"
	"fmt"
	"math/big"

	"github.com/zenanetwork/go-zenanet/common"
	cmath "github.com/zenanetwork/go-zenanet/common/math"
	"github.com/zenanetwork/go-zenanet/consensus/misc"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/tracing"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/core/vm"
	"github.com/zenanetwork/go-zenanet/crypto"
	"github.com/zenanetwork/go-zenanet/params"
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *params.ChainConfig // Chain configuration options
	bc     *BlockChain         // Canonical header chain
	hc     *HeaderChain
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *params.ChainConfig, bc *BlockChain, hc *HeaderChain) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		hc:     hc,
	}
}

// Process processes the state changes according to the Zenanet rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config, interruptCtx context.Context) (types.Receipts, []*types.Log, uint64, error) {
	var (
		receipts    types.Receipts
		usedGas     = new(uint64)
		header      = block.Header()
		blockHash   = block.Hash()
		blockNumber = block.Number()
		allLogs     []*types.Log
		gp          = new(GasPool).AddGas(block.GasLimit())
	)

	// Mutate the block and state according to any hard-fork specs
	if p.config.DAOForkSupport && p.config.DAOForkBlock != nil && p.config.DAOForkBlock.Cmp(block.Number()) == 0 {
		misc.ApplyDAOHardFork(statedb)
	}
	var (
		context vm.BlockContext
		signer  = types.MakeSigner(p.config, header.Number, header.Time)
	)
	context = NewEVMBlockContext(header, p.hc, nil)
	vmenv := vm.NewEVM(context, vm.TxContext{}, statedb, p.config, cfg)
	if beaconRoot := block.BeaconRoot(); beaconRoot != nil {
		ProcessBeaconBlockRoot(*beaconRoot, vmenv, statedb)
	}
	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		if interruptCtx != nil {
			select {
			case <-interruptCtx.Done():
				return nil, nil, 0, interruptCtx.Err()
			default:
			}
		}

		msg, err := TransactionToMessage(tx, signer, header.BaseFee)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not apply tx %d [%v]: %w", i, tx.Hash().Hex(), err)
		}

		statedb.SetTxContext(tx.Hash(), i)

		receipt, err := ApplyTransactionWithEVM(msg, p.config, gp, statedb, blockNumber, blockHash, tx, usedGas, vmenv, interruptCtx)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not apply tx %d [%v]: %w", i, tx.Hash().Hex(), err)
		}

		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}
	// Fail if Shanghai not enabled and len(withdrawals) is non-zero.
	withdrawals := block.Withdrawals()
	if !p.config.IsShanghai(block.Number()) && withdrawals != nil {
		return nil, nil, 0, fmt.Errorf("withdrawals before shanghai")
	}
	// Zena does not support withdrawals
	// nolint
	if withdrawals != nil {
		withdrawals = nil
	}
	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	p.hc.engine.Finalize(p.bc, header, statedb, block.Body())

	return receipts, allLogs, *usedGas, nil
}

// ApplyTransactionWithEVM attempts to apply a transaction to the given state database
// and uses the input parameters for its environment similar to ApplyTransaction. However,
// this method takes an already created EVM instance as input.
func ApplyTransactionWithEVM(msg *Message, config *params.ChainConfig, gp *GasPool, statedb *state.StateDB, blockNumber *big.Int, blockHash common.Hash, tx *types.Transaction, usedGas *uint64, evm *vm.EVM, interruptCtx context.Context) (receipt *types.Receipt, err error) {
	if evm.Config.Tracer != nil && evm.Config.Tracer.OnTxStart != nil {
		evm.Config.Tracer.OnTxStart(evm.GetVMContext(), tx, msg.From)
		if evm.Config.Tracer.OnTxEnd != nil {
			defer func() {
				evm.Config.Tracer.OnTxEnd(receipt, err)
			}()
		}
	}
	// Create a new context to be used in the EVM environment.
	txContext := NewEVMTxContext(msg)
	evm.Reset(txContext, statedb)

	var result *ExecutionResult

	backupMVHashMap := statedb.GetMVHashmap()

	// pause recording read and write
	statedb.SetMVHashmap(nil)

	coinbaseBalance := statedb.GetBalance(evm.Context.Coinbase)

	// resume recording read and write
	statedb.SetMVHashmap(backupMVHashMap)

	result, err = ApplyMessageNoFeeBurnOrTip(evm, *msg, gp, interruptCtx)
	if err != nil {
		return nil, err
	}

	// stop recording read and write
	statedb.SetMVHashmap(nil)

	if evm.ChainConfig().IsLondon(blockNumber) {
		statedb.AddBalance(result.BurntContractAddress, cmath.BigIntToUint256Int(result.FeeBurnt), tracing.BalanceChangeTransfer)
	}

	// TODO(raneet10) Double check
	statedb.AddBalance(evm.Context.Coinbase, cmath.BigIntToUint256Int(result.FeeTipped), tracing.BalanceChangeTransfer)
	output1 := new(big.Int).SetBytes(result.SenderInitBalance.Bytes())
	output2 := new(big.Int).SetBytes(coinbaseBalance.Bytes())

	// Deprecating transfer log and will be removed in future fork. PLEASE DO NOT USE this transfer log going forward. Parameters won't get updated as expected going forward with EIP1559
	// add transfer log
	AddFeeTransferLog(
		statedb,

		msg.From,
		evm.Context.Coinbase,

		result.FeeTipped,
		result.SenderInitBalance,
		coinbaseBalance.ToBig(),
		output1.Sub(output1, result.FeeTipped),
		output2.Add(output2, result.FeeTipped),
	)

	if result.Err == vm.ErrInterrupt {
		return nil, result.Err
	}

	// Update the state with pending changes.
	var root []byte

	if config.IsByzantium(blockNumber) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsEIP158(blockNumber)).Bytes()
	}

	*usedGas += result.UsedGas

	// Create a new receipt for the transaction, storing the intermediate root and gas used
	// by the tx.
	receipt = &types.Receipt{Type: tx.Type(), PostState: root, CumulativeGasUsed: *usedGas}
	if result.Failed() {
		receipt.Status = types.ReceiptStatusFailed
	} else {
		receipt.Status = types.ReceiptStatusSuccessful
	}

	receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas

	if tx.Type() == types.BlobTxType {
		receipt.BlobGasUsed = uint64(len(tx.BlobHashes()) * params.BlobTxBlobGasPerBlob)
		receipt.BlobGasPrice = evm.Context.BlobBaseFee
	}

	// If the transaction created a contract, store the creation address in the receipt.
	if msg.To == nil {
		receipt.ContractAddress = crypto.CreateAddress(evm.TxContext.Origin, tx.Nonce())
	}

	// Set the receipt logs and create the bloom filter.
	receipt.Logs = statedb.GetLogs(tx.Hash(), blockNumber.Uint64(), blockHash)
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	receipt.BlockHash = blockHash
	receipt.BlockNumber = blockNumber
	receipt.TransactionIndex = uint(statedb.TxIndex())

	return receipt, err
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config, interruptCtx context.Context) (*types.Receipt, error) {
	msg, err := TransactionToMessage(tx, types.MakeSigner(config, header.Number, header.Time), header.BaseFee)
	if err != nil {
		return nil, err
	}
	// Create a new context to be used in the EVM environment
	blockContext := NewEVMBlockContext(header, bc, author)
	txContext := NewEVMTxContext(msg)
	vmenv := vm.NewEVM(blockContext, txContext, statedb, config, cfg)
	return ApplyTransactionWithEVM(msg, config, gp, statedb, header.Number, header.Hash(), tx, usedGas, vmenv, interruptCtx)
}

// ProcessBeaconBlockRoot applies the EIP-4788 system call to the beacon block root
// contract. This method is exported to be used in tests.
func ProcessBeaconBlockRoot(beaconRoot common.Hash, vmenv *vm.EVM, statedb *state.StateDB) {
	if vmenv.Config.Tracer != nil && vmenv.Config.Tracer.OnSystemCallStart != nil {
		vmenv.Config.Tracer.OnSystemCallStart()
	}
	if vmenv.Config.Tracer != nil && vmenv.Config.Tracer.OnSystemCallEnd != nil {
		defer vmenv.Config.Tracer.OnSystemCallEnd()
	}

	// If EIP-4788 is enabled, we need to invoke the beaconroot storage contract with
	// the new root
	msg := &Message{
		From:      params.SystemAddress,
		GasLimit:  30_000_000,
		GasPrice:  common.Big0,
		GasFeeCap: common.Big0,
		GasTipCap: common.Big0,
		To:        &params.BeaconRootsAddress,
		Data:      beaconRoot[:],
	}
	vmenv.Reset(NewEVMTxContext(msg), statedb)
	statedb.AddAddressToAccessList(params.BeaconRootsAddress)
	_, _, _ = vmenv.Call(vm.AccountRef(msg.From), *msg.To, msg.Data, 30_000_000, common.U2560, nil)
	statedb.Finalise(true)
}
