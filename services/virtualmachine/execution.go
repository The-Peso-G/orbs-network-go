// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package virtualmachine

import (
	"context"
	"github.com/orbs-network/crypto-lib-go/crypto/digest"
	"github.com/orbs-network/orbs-network-go/instrumentation/logfields"
	"github.com/orbs-network/orbs-network-go/instrumentation/trace"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/scribe/log"
)

type TransactionOrQuery interface {
	String() string
	ContractName() primitives.ContractName
	MethodName() primitives.MethodName
	RawInputArgumentArrayWithHeader() []byte
	Signer() *protocol.Signer
}

func (s *service) runMethod(
	ctx context.Context,
	lastCommittedBlockHeight primitives.BlockHeight,
	currentBlockHeight primitives.BlockHeight,
	currentBlockTimestamp primitives.TimestampNano,
	currentBlockProposerAddress primitives.NodeAddress,
	currentBlockReferenceTime primitives.TimestampSeconds,
	lastBlockReferenceTime primitives.TimestampSeconds,
	transactionOrQuery TransactionOrQuery,
	accessScope protocol.ExecutionAccessScope,
	batchTransientState *transientState,
) (protocol.ExecutionResult, *protocol.ArgumentArray, *protocol.EventsArray, error) {

	// create execution context
	executionContextId, executionContext := s.contexts.allocateExecutionContext(lastCommittedBlockHeight, currentBlockHeight, currentBlockTimestamp, currentBlockProposerAddress, currentBlockReferenceTime, lastBlockReferenceTime, accessScope, transactionOrQuery)
	defer s.contexts.destroyExecutionContext(executionContextId)
	executionContext.batchTransientState = batchTransientState

	// get deployment info
	processor, err := s.getServiceDeployment(ctx, executionContext, transactionOrQuery.ContractName())
	if err != nil {
		s.logger.Info("get deployment info for contract failed", log.Error(err), log.Stringable("transaction-or-query", transactionOrQuery))
		return protocol.EXECUTION_RESULT_ERROR_CONTRACT_NOT_DEPLOYED, nil, nil, err
	}

	// modify execution context
	executionContext.serviceStackPush(transactionOrQuery.ContractName())
	defer executionContext.serviceStackPop()

	// execute the call
	inputArgs := protocol.ArgumentArrayReader(transactionOrQuery.RawInputArgumentArrayWithHeader())
	output, err := processor.ProcessCall(ctx, &services.ProcessCallInput{
		ContextId:              executionContextId,
		ContractName:           transactionOrQuery.ContractName(),
		MethodName:             transactionOrQuery.MethodName(),
		InputArgumentArray:     inputArgs,
		AccessScope:            accessScope,
		CallingPermissionScope: protocol.PERMISSION_SCOPE_SERVICE,
	})
	if err != nil {
		s.logger.Info("transaction execution failed", log.Stringable("result", output.CallResult), log.Error(err), log.Stringable("transaction-or-query", transactionOrQuery))
	}

	if batchTransientState != nil && output.CallResult == protocol.EXECUTION_RESULT_SUCCESS {
		executionContext.transientState.mergeIntoTransientState(batchTransientState)
	}

	outputEvents := (&protocol.EventsArrayBuilder{
		Events: executionContext.eventList,
	}).Build()

	return output.CallResult, output.OutputArgumentArray, outputEvents, err
}

func (s *service) processTransactionSet(
	ctx context.Context,
	currentBlockHeight primitives.BlockHeight,
	currentBlockTimestamp primitives.TimestampNano,
	currentBlockProposerAddress primitives.NodeAddress,
	currentBlockReferenceTime primitives.TimestampSeconds,
	lastBlockReferenceTime primitives.TimestampSeconds,
	signedTransactions []*protocol.SignedTransaction,
) ([]*protocol.TransactionReceipt, []*protocol.ContractStateDiff) {

	logger := s.logger.WithTags(trace.LogFieldFrom(ctx))
	lastCommittedBlockHeight := currentBlockHeight - 1

	// create batch transient state
	batchTransientState := newTransientState()

	// receipts for result
	receipts := make([]*protocol.TransactionReceipt, 0, len(signedTransactions))

	for _, signedTransaction := range signedTransactions {
		logger.Info("processing transaction", log.Stringable("contract", signedTransaction.Transaction().ContractName()), log.Stringable("method", signedTransaction.Transaction().MethodName()), logfields.BlockHeight(currentBlockHeight))
		callResult, outputArgs, outputEvents, _ := s.runMethod(ctx, lastCommittedBlockHeight, currentBlockHeight, currentBlockTimestamp, currentBlockProposerAddress, currentBlockReferenceTime, lastBlockReferenceTime, signedTransaction.Transaction(), protocol.ACCESS_SCOPE_READ_WRITE, batchTransientState)
		if outputArgs == nil {
			outputArgs = protocol.ArgumentsArrayEmpty()
		}
		if outputEvents == nil {
			outputEvents = (&protocol.EventsArrayBuilder{}).Build()
		}

		receipt := encodeTransactionReceipt(signedTransaction.Transaction(), callResult, outputArgs, outputEvents)
		receipts = append(receipts, receipt)
	}

	stateDiffs := encodeBatchTransientStateToStateDiffs(batchTransientState)
	return receipts, stateDiffs
}

func (s *service) getRecentCommittedBlockInfo(ctx context.Context) (primitives.BlockHeight, primitives.TimestampNano, primitives.TimestampSeconds, primitives.TimestampSeconds,  primitives.NodeAddress, error) {
	output, err := s.stateStorage.GetLastCommittedBlockInfo(ctx, &services.GetLastCommittedBlockInfoInput{})
	if err != nil {
		return 0, 0, 0, 0, []byte{}, err
	}

	return output.BlockHeight, output.BlockTimestamp, output.CurrentReferenceTime, output.PrevReferenceTime, output.BlockProposerAddress, nil
}

func encodeTransactionReceipt(transaction *protocol.Transaction, result protocol.ExecutionResult, outputArgs *protocol.ArgumentArray, outputEvents *protocol.EventsArray) *protocol.TransactionReceipt {
	return (&protocol.TransactionReceiptBuilder{
		Txhash:              digest.CalcTxHash(transaction),
		ExecutionResult:     result,
		OutputEventsArray:   outputEvents.RawEventsArray(),
		OutputArgumentArray: outputArgs.RawArgumentsArray(),
	}).Build()
}

func encodeBatchTransientStateToStateDiffs(batchTransientState *transientState) []*protocol.ContractStateDiff {
	res := []*protocol.ContractStateDiff{}
	for _, contractName := range batchTransientState.contractSortOrder {
		stateDiffs := []*protocol.StateRecordBuilder{}
		batchTransientState.forDirty(contractName, func(key []byte, value []byte) {
			stateDiffs = append(stateDiffs, &protocol.StateRecordBuilder{
				Key:   key,
				Value: value,
			})
		})
		if len(stateDiffs) > 0 {
			res = append(res, (&protocol.ContractStateDiffBuilder{
				ContractName: contractName,
				StateDiffs:   stateDiffs,
			}).Build())
		}
	}
	return res
}
