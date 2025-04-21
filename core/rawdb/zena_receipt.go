package rawdb

import (
	"math/big"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/params"
	"github.com/zenanetwork/go-zenanet/rlp"
)

var (
	// zena receipt key
	zenaReceiptKey = types.ZenaReceiptKey

	// zenaTxLookupPrefix + hash -> transaction/receipt lookup metadata
	zenaTxLookupPrefix = []byte(zenaTxLookupPrefixStr)
)

const (
	zenaTxLookupPrefixStr = "zena-iris-tx-lookup-"

	// freezerZenaReceiptTable indicates the name of the freezer zena receipts table.
	freezerZenaReceiptTable = "zena-iris-receipts"
)

// zenaTxLookupKey = zenaTxLookupPrefix + zena tx hash
func zenaTxLookupKey(hash common.Hash) []byte {
	return append(zenaTxLookupPrefix, hash.Bytes()...)
}

func ReadZenaReceiptRLP(db ethdb.Reader, hash common.Hash, number uint64) rlp.RawValue {
	var data []byte

	err := db.ReadAncients(func(reader ethdb.AncientReaderOp) error {
		// Check if the data is in ancients
		if isCanon(reader, number, hash) {
			data, _ = reader.Ancient(freezerZenaReceiptTable, number)

			return nil
		}

		// If not, try reading from leveldb
		data, _ = db.Get(zenaReceiptKey(number, hash))

		return nil
	})

	if err != nil {
		log.Warn("during ReadZenaReceiptRLP", "number", number, "hash", hash, "err", err)
	}

	return data
}

// ReadRawZenaReceipt retrieves the block receipt belonging to a block.
// The receipt metadata fields are not guaranteed to be populated, so they
// should not be used. Use ReadZenaReceipt instead if the metadata is needed.
func ReadRawZenaReceipt(db ethdb.Reader, hash common.Hash, number uint64) *types.Receipt {
	// Retrieve the flattened receipt slice
	data := ReadZenaReceiptRLP(db, hash, number)
	if len(data) == 0 {
		return nil
	}

	// Convert the receipts from their storage form to their internal representation
	var storageReceipt types.ReceiptForStorage
	if err := rlp.DecodeBytes(data, &storageReceipt); err != nil {
		log.Error("Invalid zena receipt RLP", "hash", hash, "err", err)
		return nil
	}

	return (*types.Receipt)(&storageReceipt)
}

// ReadZenaReceipt retrieves all the zena block receipts belonging to a block, including
// its corresponding metadata fields. If it is unable to populate these metadata
// fields then nil is returned.
func ReadZenaReceipt(db ethdb.Reader, hash common.Hash, number uint64, config *params.ChainConfig) *types.Receipt {
	if config != nil && config.Zena != nil && config.Zena.Sprint != nil && !config.Zena.IsSprintStart(number) {
		return nil
	}

	// We're deriving many fields from the block body, retrieve beside the receipt
	zenaReceipt := ReadRawZenaReceipt(db, hash, number)
	if zenaReceipt == nil {
		return nil
	}

	// We're deriving many fields from the block body, retrieve beside the receipt
	receipts := ReadRawReceipts(db, hash, number)
	if receipts == nil {
		return nil
	}

	body := ReadBody(db, hash, number)
	if body == nil {
		log.Error("Missing body but have zena receipt", "hash", hash, "number", number)
		return nil
	}

	if err := types.DeriveFieldsForZenaReceipt(zenaReceipt, hash, number, receipts); err != nil {
		log.Error("Failed to derive zena receipt fields", "hash", hash, "number", number, "err", err)
		return nil
	}

	return zenaReceipt
}

// WriteZenaReceipt stores all the zena receipt belonging to a block.
func WriteZenaReceipt(db ethdb.KeyValueWriter, hash common.Hash, number uint64, zenaReceipt *types.ReceiptForStorage) {
	// Convert the zena receipt into their storage form and serialize them
	bytes, err := rlp.EncodeToBytes(zenaReceipt)
	if err != nil {
		log.Crit("Failed to encode zena receipt", "err", err)
	}

	// Store the flattened receipt slice
	if err := db.Put(zenaReceiptKey(number, hash), bytes); err != nil {
		log.Crit("Failed to store zena receipt", "err", err)
	}
}

// DeleteZenaReceipt removes receipt data associated with a block hash.
func DeleteZenaReceipt(db ethdb.KeyValueWriter, hash common.Hash, number uint64) {
	key := zenaReceiptKey(number, hash)

	if err := db.Delete(key); err != nil {
		log.Crit("Failed to delete zena receipt", "err", err)
	}
}

// ReadZenaTransactionWithBlockHash retrieves a specific zena (fake) transaction by tx hash and block hash, along with
// its added positional metadata.
func ReadZenaTransactionWithBlockHash(db ethdb.Reader, txHash common.Hash, blockHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	blockNumber := ReadZenaTxLookupEntry(db, txHash)
	if blockNumber == nil {
		return nil, common.Hash{}, 0, 0
	}

	body := ReadBody(db, blockHash, *blockNumber)
	if body == nil {
		log.Error("Transaction referenced missing", "number", blockNumber, "hash", blockHash)
		return nil, common.Hash{}, 0, 0
	}

	// fetch receipt and return it
	return types.NewZenaTransaction(), blockHash, *blockNumber, uint64(len(body.Transactions))
}

// ReadZenaTransaction retrieves a specific zena (fake) transaction by hash, along with
// its added positional metadata.
func ReadZenaTransaction(db ethdb.Reader, hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	blockNumber := ReadZenaTxLookupEntry(db, hash)
	if blockNumber == nil {
		return nil, common.Hash{}, 0, 0
	}

	blockHash := ReadCanonicalHash(db, *blockNumber)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}

	body := ReadBody(db, blockHash, *blockNumber)
	if body == nil {
		log.Error("Transaction referenced missing", "number", blockNumber, "hash", blockHash)
		return nil, common.Hash{}, 0, 0
	}

	// fetch receipt and return it
	return types.NewZenaTransaction(), blockHash, *blockNumber, uint64(len(body.Transactions))
}

//
// Indexes for reverse lookup
//

// ReadZenaTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the zena transaction or zena receipt using tx hash.
func ReadZenaTxLookupEntry(db ethdb.Reader, txHash common.Hash) *uint64 {
	data, _ := db.Get(zenaTxLookupKey(txHash))
	if len(data) == 0 {
		return nil
	}

	number := new(big.Int).SetBytes(data).Uint64()

	return &number
}

// WriteZenaTxLookupEntry stores a positional metadata for zena transaction using block hash and block number
func WriteZenaTxLookupEntry(db ethdb.KeyValueWriter, hash common.Hash, number uint64) {
	txHash := types.GetDerivedZenaTxHash(zenaReceiptKey(number, hash))
	if err := db.Put(zenaTxLookupKey(txHash), big.NewInt(0).SetUint64(number).Bytes()); err != nil {
		log.Crit("Failed to store zena transaction lookup entry", "err", err)
	}
}

// DeleteZenaTxLookupEntry removes zena transaction data associated with block hash and block number
func DeleteZenaTxLookupEntry(db ethdb.KeyValueWriter, hash common.Hash, number uint64) {
	txHash := types.GetDerivedZenaTxHash(zenaReceiptKey(number, hash))
	DeleteZenaTxLookupEntryByTxHash(db, txHash)
}

// DeleteZenaTxLookupEntryByTxHash removes zena transaction data associated with a zena tx hash.
func DeleteZenaTxLookupEntryByTxHash(db ethdb.KeyValueWriter, txHash common.Hash) {
	if err := db.Delete(zenaTxLookupKey(txHash)); err != nil {
		log.Crit("Failed to delete zena transaction lookup entry", "err", err)
	}
}
