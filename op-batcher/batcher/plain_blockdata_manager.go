package batcher
import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/log"
)

type plainTxData struct {
	id big.Int // TODO rename to blockNumber everywhere
	blockHash common.Hash
	data []byte // block data encoded as a batch
}

type plainBlockdataManager struct {
	log  log.Logger
	// Data for blocks that haven't been posted yet.
	datas []plainTxData
	// last block hash - for reorg detection
	tip common.Hash
	// Set of unconfirmed tx for given L2 block num -> block data. For tx resubmission
	pendingTransactions map[[32]byte]plainTxData
	// Set of confirmed tx for given block num -> inclusion block. For timeouts
	confirmedTransactions map[[32]byte]eth.BlockID
	closed bool
}

func newPlainBlockdataManager(log log.Logger) *plainBlockdataManager {
	return &plainBlockdataManager{
		log:  log,
		pendingTransactions:   make(map[[32]byte]plainTxData),
		confirmedTransactions: make(map[[32]byte]eth.BlockID),
	}
}

func toBytes32(bigInt big.Int) [32]byte {
	var bytes32 [32]byte
	bigInt.FillBytes(bytes32[:])
	return bytes32
}

func (mgr *plainBlockdataManager) TxData(l1Head eth.BlockID) (plainTxData, error) {
	mgr.log.Debug("Requested tx data", "l1Head", l1Head, "data_pending", "blocks_pending", len(mgr.datas))

	if (mgr.closed) {
    		// NOTE(norswap): I assume this is the intended behaviour? Can't hurt.
    		return plainTxData{}, io.EOF
    	}

    // All blocks have been submitted already!
    if len(mgr.datas) == 0 {
        return plainTxData{}, io.EOF
    }

	block := mgr.datas[0]
	mgr.datas = mgr.datas[1:]

    mgr.log.Trace("returning next tx data")
	bytesID := [32]byte{}
	block.id.FillBytes(bytesID[:])
    mgr.pendingTransactions[bytesID] = block
    return block, nil
}

func (mgr *plainBlockdataManager) AddL2Block(block *types.Block) error {
	if mgr.tip != (common.Hash{}) && mgr.tip != block.ParentHash() {
        return ErrReorg
    }
	batch, _, err := derive.BlockToBatch(block) // middle is l1Info
    if err != nil {
        return fmt.Errorf("converting block to batch: %w", err)
    }

	mgr.log.Info("adding L2 block", "number", block.Number(), "txs", block.Transactions().Len())

    var buf bytes.Buffer
    if err := rlp.Encode(&buf, batch); err != nil {
        return err
    }
    data := plainTxData{*block.Number(), block.Hash(), buf.Bytes()}

    mgr.datas = append(mgr.datas, data)
    mgr.tip = block.Hash()
    return nil
}

// NOTE(norswap): Useful? Only in case the drive is restarted at best.
func (mgr *plainBlockdataManager) Clear() {
	mgr.log.Trace("clearing channel manager state")
    mgr.datas = mgr.datas[:0]
    mgr.tip = common.Hash{}
	mgr.pendingTransactions = make(map[[32]byte]plainTxData)
	mgr.confirmedTransactions = make(map[[32]byte]eth.BlockID)
	mgr.closed = false
}

// NOTE(norsawp): Useful? Only if we would request blocks after closing.
func (mgr *plainBlockdataManager) Close() error {
	// Yes, this could be simpler, but keep structure if there is a need to change.
    if mgr.closed {
        return nil
    }
    mgr.closed = true
	return nil
}

// TxFailed records a transaction as failed. It will attempt to resubmit the data
// in the failed transaction.
func (mgr *plainBlockdataManager) TxFailed(id big.Int) {
	bytesID := toBytes32(id)
	if data, ok := mgr.pendingTransactions[bytesID]; ok {
		mgr.log.Trace("handling failed tranasaction", "id", id)
		mgr.datas = append([]plainTxData{data}, mgr.datas...)
		delete(mgr.pendingTransactions, bytesID)
	} else {
		mgr.log.Warn("unknown transaction marked as failed", "id", id)
	}
}

// TxConfirmed marks a transaction as confirmed on L1.
func (mgr *plainBlockdataManager) TxConfirmed(id big.Int, inclusionBlock eth.BlockID) {
	mgr.log.Debug("marked transaction as confirmed", "id", id, "block", inclusionBlock)
	bytesID := toBytes32(id)
	if _, ok := mgr.pendingTransactions[bytesID]; !ok {
		mgr.log.Warn("unknown transaction marked as confirmed", "id", id, "block", inclusionBlock)
		return
	}
	delete(mgr.pendingTransactions, bytesID)
	mgr.confirmedTransactions[bytesID] = inclusionBlock
}
