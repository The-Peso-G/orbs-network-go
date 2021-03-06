// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package test

import (
	"context"
	"encoding/hex"
	"github.com/orbs-network/crypto-lib-go/crypto/hash"
	"github.com/orbs-network/go-mock"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/services/consensuscontext"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/scribe/log"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var validatorNodeAddressesForTest = []string{
	"dfc06c5be24a67adee80b35ab4f147bb1a35c55f",
	"92d469d7c004cc0b24a192d9457836bf38effa27",
	"a899b318e65915aa2de02841eeb72fe51fddad96",
	"58e7ed8169a151602b1349c990c84ca2fb2f62eb",
	"23f97918acf48728d3f25a39a5f091a1a9574c52",
	"07492c6612f78a47d7b6a18a17792a01917dec74",
	"43a4dbbf7a672c6689dbdd662fd89a675214b00d",
	"469bd276271aa6d59e387018cf76bd00f55c7029",
	"102073b28749be1e3daf5e5947605ec7d43c3183",
	"70d92324eb8d24b7c7ed646e1996f94dcd52934a",
}

type harness struct {
	transactionPool *services.MockTransactionPool
	virtualMachine  *services.MockVirtualMachine
	stateStorage    *services.MockStateStorage
	management      *services.MockManagement
	logger          log.Logger
	service         services.ConsensusContext
	config          config.ConsensusContextConfig
}

func (h *harness) requestTransactionsBlock(ctx context.Context) (*protocol.TransactionsBlockContainer, error) {
	output, err := h.service.RequestNewTransactionsBlock(ctx, &services.RequestNewTransactionsBlockInput{
		CurrentBlockHeight:      1,
		MaxBlockSizeKb:          0,
		MaxNumberOfTransactions: 0,
		PrevBlockHash:           hash.CalcSha256([]byte{1}),
		PrevBlockTimestamp:      primitives.TimestampNano(time.Now().UnixNano() - 100),
		PrevBlockReferenceTime:  primitives.TimestampSeconds(time.Now().Add(-10*time.Minute).Unix()),
	})
	if err != nil {
		return nil, err
	}
	return output.TransactionsBlock, nil
}

func (h *harness) requestResultsBlock(ctx context.Context, txBlockContainer *protocol.TransactionsBlockContainer) (*protocol.ResultsBlockContainer, error) {
	output, err := h.service.RequestNewResultsBlock(ctx, &services.RequestNewResultsBlockInput{
		CurrentBlockHeight: 1,
		PrevBlockHash:      hash.CalcSha256([]byte{1}),
		TransactionsBlock:  txBlockContainer,
		PrevBlockTimestamp: 0,
		PrevBlockReferenceTime: primitives.TimestampSeconds(time.Now().Add(-10*time.Minute).Unix()),
	})
	if err != nil {
		return nil, err
	}
	return output.ResultsBlock, nil
}

func (h *harness) expectTxPoolToReturnXTransactions(numTransactionsToReturn uint32) {
	output := &services.GetTransactionsForOrderingOutput{
		SignedTransactions: nil,
	}

	for i := uint32(0); i < numTransactionsToReturn; i++ {
		targetAddress := builders.ClientAddressForEd25519SignerForTests(2)
		output.SignedTransactions = append(output.SignedTransactions, builders.TransferTransaction().WithAmountAndTargetAddress(uint64(i+1)*10, targetAddress).Build())
		output.ProposedBlockTimestamp = primitives.TimestampNano(time.Now().UnixNano())
	}

	h.transactionPool.When("GetTransactionsForOrdering", mock.Any, mock.Any).Return(output, nil).Times(1)
}

func (h *harness) expectTransactionsNoLongerRequestedFromTransactionPool() {
	h.transactionPool.When("GetTransactionsForOrdering", mock.Any, mock.Any).Return(nil, nil).Times(0)
}

func (h *harness) expectVirtualMachineToReturnXTransactionReceipts(receiptsCount int) {
	receipts := make([]*protocol.TransactionReceipt, receiptsCount)
	for i := 0; i < receiptsCount; i++ {
		receipts[i] = (&protocol.TransactionReceiptBuilder{
			Txhash:              hash.CalcSha256([]byte{1, 1, 1}),
			ExecutionResult:     protocol.ExecutionResult(123),
			OutputArgumentArray: []byte{9, 9, 9},
		}).Build()
	}
	output := &services.ProcessTransactionSetOutput{
		TransactionReceipts: receipts,
		ContractStateDiffs:  nil,
	}
	h.virtualMachine.When("ProcessTransactionSet", mock.Any, mock.Any).Return(output, nil)
}

func (h *harness) verifyTransactionsRequestedFromTransactionPool(t *testing.T) {
	ok, _ := h.transactionPool.Verify()

	// TODO(v1): How to print err if it's sometimes nil
	require.True(t, ok)
}
func (h *harness) expectStateHashToReturn(hash []byte) {

	stateHashOutput := &services.GetStateHashOutput{
		StateMerkleRootHash: hash,
	}
	h.stateStorage.When("GetStateHash", mock.Any, mock.Any).Return(stateHashOutput, nil)
}

func setManagementValues(m *services.MockManagement, pv primitives.ProtocolVersion, ref primitives.TimestampSeconds, gen primitives.TimestampSeconds) {
	m.When("GetProtocolVersion", mock.Any, mock.Any).Return(&services.GetProtocolVersionOutput{ProtocolVersion: pv}, nil)
	m.When("GetCurrentReference", mock.Any, mock.Any).Return(&services.GetCurrentReferenceOutput{CurrentReference: ref}, nil)
	m.When("GetGenesisReference", mock.Any, mock.Any).Return(&services.GetGenesisReferenceOutput{CurrentReference: ref, GenesisReference: gen,}, nil)
}

func newHarness(logger log.Logger, enableTriggers bool) *harness {
	txPool := &services.MockTransactionPool{}
	machine := &services.MockVirtualMachine{}
	state := &services.MockStateStorage{}
	genesisValidatorNodes := make(map[string]config.ValidatorNode)
	for _, nodeAddress := range validatorNodeAddressesForTest {
		bytes, _ := hex.DecodeString(nodeAddress)
		genesisValidatorNodes[nodeAddress] = config.NewHardCodedValidatorNode(bytes)
	}

	cfg := config.ForConsensusContextTests(enableTriggers)

	management := &services.MockManagement{}
	setManagementValues(management, config.MAXIMAL_PROTOCOL_VERSION_SUPPORTED_VALUE, primitives.TimestampSeconds(time.Now().Unix()), 50)

	metricFactory := metric.NewRegistry()

	service := consensuscontext.NewConsensusContext(
		txPool,
		machine,
		state,
		management,
		cfg, logger, metricFactory)

	return &harness{
		transactionPool: txPool,
		virtualMachine:  machine,
		stateStorage:    state,
		management:      management,
		logger:          logger,
		service:         service,
		config:          cfg,
	}
}
