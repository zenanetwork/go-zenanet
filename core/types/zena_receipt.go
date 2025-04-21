package types

import (
	"encoding/binary"
	"math/big"
	"sort"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/crypto"
)

// TenToTheFive - To be used while sorting zena logs
//
// Sorted using ( blockNumber * (10 ** 5) + logIndex )
const TenToTheFive uint64 = 100000

var (
	zenaReceiptPrefix = []byte("zena-iris-receipt-") // zenaReceiptPrefix + number + block hash -> zena block receipt

	// SystemAddress address for system sender
	SystemAddress = common.HexToAddress("0xffffFFFfFFffffffffffffffFfFFFfffFFFfFFfE")
)

// ZenaReceiptKey = zenaReceiptPrefix + num (uint64 big endian) + hash
func ZenaReceiptKey(number uint64, hash common.Hash) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)

	return append(append(zenaReceiptPrefix, enc...), hash.Bytes()...)
}

// GetDerivedZenaTxHash get derived tx hash from receipt key
func GetDerivedZenaTxHash(receiptKey []byte) common.Hash {
	return common.BytesToHash(crypto.Keccak256(receiptKey))
}

// NewZenaTransaction create new zena transaction for zena receipt
func NewZenaTransaction() *Transaction {
	return NewTransaction(0, common.Address{}, big.NewInt(0), 0, big.NewInt(0), make([]byte, 0))
}

// DeriveFieldsForZenaReceipt fills the receipts with their computed fields based on consensus
// data and contextual infos like containing block and transactions.
func DeriveFieldsForZenaReceipt(receipt *Receipt, hash common.Hash, number uint64, receipts Receipts) error {
	// get derived tx hash
	txHash := GetDerivedZenaTxHash(ZenaReceiptKey(number, hash))
	txIndex := uint(len(receipts))

	// set tx hash and tx index
	receipt.TxHash = txHash
	receipt.TransactionIndex = txIndex
	receipt.BlockHash = hash
	receipt.BlockNumber = big.NewInt(0).SetUint64(number)

	logIndex := 0
	for i := 0; i < len(receipts); i++ {
		logIndex += len(receipts[i].Logs)
	}

	// The derived log fields can simply be set from the block and transaction
	for j := 0; j < len(receipt.Logs); j++ {
		receipt.Logs[j].BlockNumber = number
		receipt.Logs[j].BlockHash = hash
		receipt.Logs[j].TxHash = txHash
		receipt.Logs[j].TxIndex = txIndex
		receipt.Logs[j].Index = uint(logIndex)
		logIndex++
	}

	return nil
}

// DeriveFieldsForZenaLogs fills the receipts with their computed fields based on consensus
// data and contextual infos like containing block and transactions.
func DeriveFieldsForZenaLogs(logs []*Log, hash common.Hash, number uint64, txIndex uint, logIndex uint) {
	// get derived tx hash
	txHash := GetDerivedZenaTxHash(ZenaReceiptKey(number, hash))

	// the derived log fields can simply be set from the block and transaction
	for j := 0; j < len(logs); j++ {
		logs[j].BlockNumber = number
		logs[j].BlockHash = hash
		logs[j].TxHash = txHash
		logs[j].TxIndex = txIndex
		logs[j].Index = logIndex
		logIndex++
	}
}

// MergeZenaLogs merges receipt logs and block receipt logs
func MergeZenaLogs(logs []*Log, zenaLogs []*Log) []*Log {
	result := append(logs, zenaLogs...)

	sort.SliceStable(result, func(i int, j int) bool {
		return (result[i].BlockNumber*TenToTheFive + uint64(result[i].Index)) < (result[j].BlockNumber*TenToTheFive + uint64(result[j].Index))
	})

	return result
}
