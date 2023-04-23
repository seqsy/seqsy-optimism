package op_seqsy
import (
	"context"
	"github.com/ethereum/go-ethereum/rpc"
"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/client"
	"github.com/ethereum-optimism/optimism/op-node/sources"
	"github.com/ethereum-optimism/optimism/op-node/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

// TestSeqsyE2E sets up a L1 Geth node, a rollup node, and a L2 geth node and then confirms that
// TODO
func TestSeqsyE2E(t *testing.T) {
	InitParallel(t)
	cfg := DefaultSystemConfig(t)

	sys, err := cfg.Start()
	require.Nil(t, err, "Error starting up system")
	defer sys.Close()

	log := testlog.Logger(t, log.LvlInfo)
	log.Info("genesis", "l2", sys.RollupConfig.Genesis.L2, "l1", sys.RollupConfig.Genesis.L1, "l2_time", sys.RollupConfig.Genesis.L2Time)

	// l1Client := sys.Clients["l1"]
	l2Seq := sys.Clients["sequencer"]
	l2Verif := sys.Clients["verifier"]

	// Transactor Account
	ethPrivKey := sys.cfg.Secrets.Alice

	// Send Transaction & wait for success
	fromAddr := sys.cfg.Secrets.Addresses().Alice

	_ = l2Seq
	_ = l2Verif
	_ = ethPrivKey
	_ = fromAddr

	// get start balance
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	startBalance, err := l2Verif.BalanceAt(ctx, fromAddr, nil)
	require.Nil(t, err)

	value := big.NewInt(1_000_000_000)

	// Submit TX to L2 sequencer node
    receipt := SendL2Tx(t, cfg, l2Seq, ethPrivKey, func(opts *TxOpts) {
        opts.Value = value
        // opts.Nonce = 1 // Already have deposit
        opts.ToAddr = &common.Address{0xff, 0xff}
        opts.VerifyOnClients(l2Verif)
    })

	// Verify blocks match after batch submission on verifiers and sequencers
	seqBlock, err := l2Seq.BlockByNumber(context.Background(), receipt.BlockNumber)
    require.Nil(t, err)
	verifBlock, err := l2Verif.BlockByNumber(context.Background(), receipt.BlockNumber)
    require.Nil(t, err)
	require.Equal(t, verifBlock.NumberU64(), seqBlock.NumberU64(), "Verifier and sequencer blocks not the same after including a batch tx")
    require.Equal(t, verifBlock.ParentHash(), seqBlock.ParentHash(), "Verifier and sequencer blocks parent hashes not the same after including a batch tx")
    require.Equal(t, verifBlock.Hash(), seqBlock.Hash(), "Verifier and sequencer blocks not the same after including a batch tx")

	// get end balance
    ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    endBalance, err := l2Verif.BalanceAt(ctx, fromAddr, nil)
    require.Nil(t, err)

	// check difference
	diff := new(big.Int)
    diff = diff.Sub(endBalance, startBalance)
	diff = diff.Sub(diff, value)
	tmp := new(big.Int)
    tmp.SetUint64(receipt.GasUsed) // only 21k, not accurate!
	diff = diff.Sub(diff, tmp)
	tmp.SetUint64(50_000) // 50k, real diff is more like 28k
	// condition: we get the correct balance change up to 50k
	require.True(t, tmp.Cmp(diff) == 1, "Did not get expected balance change")

	rollupRPCClient, err := rpc.DialContext(context.Background(), sys.RollupNodes["sequencer"].HTTPEndpoint())
	require.Nil(t, err)
	rollupClient := sources.NewRollupClient(client.NewBaseRPCClient(rollupRPCClient))
	// basic check that sync status works
	seqStatus, err := rollupClient.SyncStatus(context.Background())
	require.Nil(t, err)
	require.LessOrEqual(t, seqBlock.NumberU64(), seqStatus.UnsafeL2.Number)
	// basic check that version endpoint works
	seqVersion, err := rollupClient.Version(context.Background())
	require.Nil(t, err)
	require.NotEqual(t, "", seqVersion)
}
