package derive

import (
	"bytes"
	"context"
	"io"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/log"
)

type BatchProvider struct {
	log log.Logger
	prev *L1Retrieval
}

var _ ResetableStage = (*BatchProvider)(nil)
var _ NextBatchProvider = (*BatchProvider)(nil)

func NewBatchProvider(log log.Logger, prev *L1Retrieval) *BatchProvider {
	return &BatchProvider{
		log: log,
		prev: prev,
	}
}

func (bp *BatchProvider) Origin() eth.L1BlockRef {
	return bp.prev.Origin()
}

func (bp *BatchProvider) NextBatch(ctx context.Context) (*BatchData, error) {
	data, err := bp.prev.NextData(ctx)
	if err != nil {
		return nil, err
	}

	// At minimum, select, block number and block hash need to be removed (4 + 32 + 32 = 68 bytes)
	if (len(data) < 68) {
		return nil, NotEnoughData
	}
	read, err := BatchReader(bytes.NewBuffer(data[68:]), bp.Origin())
	if err != nil {
        bp.log.Error("Error creating batch reader from batch data", "err", err)
        return nil, err
    } else if read == nil {
		// NOTE(norswap) just doing random stuff lol, not sure if this can happen
		return nil, io.EOF
	}

	batch, err := read()
    if err == io.EOF {
        return nil, NotEnoughData
    } else if err != nil {
        bp.log.Warn("failed to read batch from data", "err", err)
        return nil, NotEnoughData
    }
    return batch.Batch, nil
}

func (bp *BatchProvider) Reset(ctx context.Context, _ eth.L1BlockRef, _ eth.SystemConfig) error {
	return io.EOF
}
