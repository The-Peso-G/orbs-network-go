package digest

import (
	"github.com/orbs-network/orbs-network-go/crypto/hash"
	"github.com/orbs-network/orbs-network-go/crypto/merkle"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
)

func CalcTransactionMetaDataHash(metaData *protocol.TransactionsBlockMetadata) primitives.Sha256 {
	return hash.CalcSha256(metaData.Raw())
}

func CalcTransactionsBlockHash(transactionsBlock *protocol.TransactionsBlockContainer) primitives.Sha256 {
	if transactionsBlock == nil || transactionsBlock.Header == nil {
		return nil
	}
	return hash.CalcSha256(transactionsBlock.Header.Raw())
}

func CalcResultsBlockHash(resultsBlock *protocol.ResultsBlockContainer) primitives.Sha256 {
	if resultsBlock == nil || resultsBlock.Header == nil {
		return nil
	}
	return hash.CalcSha256(resultsBlock.Header.Raw())
}

func CalcBlockHash(transactionsBlock *protocol.TransactionsBlockContainer, resultsBlock *protocol.ResultsBlockContainer) primitives.Sha256 {
	if transactionsBlock == nil || resultsBlock == nil {
		return nil
	}
	transactionsBlockHash := CalcTransactionsBlockHash(transactionsBlock)
	resultsBlockHash := CalcResultsBlockHash(resultsBlock)
	return hash.CalcSha256(transactionsBlockHash, resultsBlockHash)
}

func CalcTransactionsMerkleRoot(txs []*protocol.SignedTransaction) (primitives.Sha256, error) {
	txHashValues := make([]primitives.Sha256, len(txs))
	for i := 0; i < len(txs); i++ {
		txHashValues[i] = CalcTxHash(txs[i].Transaction())
	}
	return merkle.CalculateOrderedTreeRoot(txHashValues), nil
}

func CalcReceiptsMerkleRoot(receipts []*protocol.TransactionReceipt) (primitives.Sha256, error) {
	rptHashValues := make([]primitives.Sha256, len(receipts))
	for i := 0; i < len(receipts); i++ {
		rptHashValues[i] = CalcReceiptHash(receipts[i])
	}
	return merkle.CalculateOrderedTreeRoot(rptHashValues), nil
}

// TODO v1 Rewrite without Merkle tree and then rename the function
// See https://tree.taiga.io/project/orbs-network/us/651

func CalcStateDiffHash(stateDiffs []*protocol.ContractStateDiff) (primitives.Sha256, error) {
	stdHashValues := make([][]byte, len(stateDiffs))
	for i := 0; i < len(stateDiffs); i++ {
		stdHashValues[i] = CalcContractStateDiffHash(stateDiffs[i])
	}
	//return merkle.CalculateOrderedTreeRoot(stdHashValues), nil
	return hash.CalcSha256(stdHashValues...), nil
}

func CalcNewBlockTimestamp(prevBlockTimestamp primitives.TimestampNano, now primitives.TimestampNano) primitives.TimestampNano {
	if now > prevBlockTimestamp {
		return now + 1
	}
	return prevBlockTimestamp + 1
}

// CalcStateDiffHash
type CalcStateDiffHashAdapter interface {
	CalcStateDiffHash(stateDiffs []*protocol.ContractStateDiff) (primitives.Sha256, error)
}
type realCalcStateDiffHashAdapter struct {
	calcStateDiffHash func(stateDiffs []*protocol.ContractStateDiff) (primitives.Sha256, error)
}

func (r *realCalcStateDiffHashAdapter) CalcStateDiffHash(stateDiffs []*protocol.ContractStateDiff) (primitives.Sha256, error) {
	return r.calcStateDiffHash(stateDiffs)
}
func NewRealCalcStateDiffHashAdapter(f func(stateDiffs []*protocol.ContractStateDiff) (primitives.Sha256, error)) CalcStateDiffHashAdapter {
	return &realCalcStateDiffHashAdapter{
		calcStateDiffHash: f,
	}
}

// CalcReceiptsMerkleRoot
type CalcReceiptsMerkleRootAdapter interface {
	CalcReceiptsMerkleRoot(receipts []*protocol.TransactionReceipt) (primitives.Sha256, error)
}

type realCalcReceiptsMerkleRootAdapter struct {
	calcReceiptsMerkleRoot func(receipts []*protocol.TransactionReceipt) (primitives.Sha256, error)
}

func (r *realCalcReceiptsMerkleRootAdapter) CalcReceiptsMerkleRoot(receipts []*protocol.TransactionReceipt) (primitives.Sha256, error) {
	return r.calcReceiptsMerkleRoot(receipts)
}
func NewRealCalcReceiptsMerkleRootAdapter(f func(receipts []*protocol.TransactionReceipt) (primitives.Sha256, error)) CalcReceiptsMerkleRootAdapter {
	return &realCalcReceiptsMerkleRootAdapter{
		calcReceiptsMerkleRoot: f,
	}
}
