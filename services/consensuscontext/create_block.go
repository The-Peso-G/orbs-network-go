package consensuscontext

import (
	"github.com/orbs-network/orbs-network-go/crypto"
	"github.com/orbs-network/orbs-network-go/services/blockstorage"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"time"
)

func (s *service) createTransactionsBlock(blockHeight primitives.BlockHeight, prevBlockHash primitives.Sha256) (*protocol.TransactionsBlockContainer, error) {

	proposedTransactions, err := s.fetchTransactions(&services.GetTransactionsForOrderingInput{
		MaxNumberOfTransactions: 1,
	}, 2, 1000*time.Millisecond)
	if err != nil {
		return nil, err
	}
	txCount := len(proposedTransactions.SignedTransactions)

	txBlock := &protocol.TransactionsBlockContainer{
		Header: (&protocol.TransactionsBlockHeaderBuilder{
			ProtocolVersion:       blockstorage.ProtocolVersion,
			BlockHeight:           blockHeight,
			PrevBlockHashPtr:      prevBlockHash,
			NumSignedTransactions: uint32(txCount),
		}).Build(),
		Metadata:           (&protocol.TransactionsBlockMetadataBuilder{}).Build(),
		SignedTransactions: proposedTransactions.SignedTransactions,
		BlockProof:         nil,
	}
	return txBlock, nil
}

func (s *service) createResultsBlock(blockHeight primitives.BlockHeight, prevBlockHash primitives.Sha256, transactionsBlock *protocol.TransactionsBlockContainer) *protocol.ResultsBlockContainer {
	rxBlock := &protocol.ResultsBlockContainer{
		Header: (&protocol.ResultsBlockHeaderBuilder{
			ProtocolVersion:          blockstorage.ProtocolVersion,
			BlockHeight:              blockHeight,
			PrevBlockHashPtr:         prevBlockHash,
			TransactionsBlockHashPtr: crypto.CalcTransactionsBlockHash(transactionsBlock),
		}).Build(),
		TransactionReceipts: nil,
		ContractStateDiffs:  nil,
		BlockProof:          nil,
	}
	return rxBlock
}
