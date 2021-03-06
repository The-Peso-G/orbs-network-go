// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package test

import (
	"context"
	"github.com/orbs-network/orbs-network-go/services/processor/native/repository/_Deployments"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"github.com/orbs-network/orbs-network-go/test/with"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProcessQuery_Success(t *testing.T) {
	with.Context(func(ctx context.Context) {
		with.Logging(t, func(parent *with.LoggingHarness) {

			h := newHarness(parent.Logger)
			h.expectSystemContractCalled(deployments_systemcontract.CONTRACT_NAME, deployments_systemcontract.METHOD_GET_INFO, nil, uint32(protocol.PROCESSOR_TYPE_NATIVE)) // assume all contracts are deployed

			h.expectStateStorageLastCommittedBlockInfoBlockHeightRequested(12)
			h.expectNativeContractMethodCalled("Contract1", "method1", func(executionContextId primitives.ExecutionContextId, inputArgs *protocol.ArgumentArray) (protocol.ExecutionResult, *protocol.ArgumentArray, error) {
				return protocol.EXECUTION_RESULT_SUCCESS, builders.ArgumentsArray(uint32(17), "hello", []byte{0x01, 0x02}), nil
			})

			result, outputArgs, refHeight, outputEvents, err := h.processQuery(ctx, "Contract1", "method1")
			require.NoError(t, err, "process query should not fail")
			require.Equal(t, protocol.EXECUTION_RESULT_SUCCESS, result, "process query should return successful result")
			expectedOutputArgs, err := protocol.PackedInputArgumentsFromNatives(builders.VarsToSlice(uint32(17), "hello", []byte{0x01, 0x02}))
			require.NoError(t, err, "output args must pack")
			require.EqualValues(t, expectedOutputArgs, outputArgs, "process query should return matching output args")
			require.EqualValues(t, 12, refHeight)
			require.Equal(t, (&protocol.EventsArrayBuilder{}).Build().RawEventsArray(), outputEvents)

			h.verifySystemContractCalled(t)
			h.verifyStateStorageBlockHeightRequested(t)
			h.verifyNativeContractMethodCalled(t)
		})
	})
}

func TestProcessQuery_ContractError(t *testing.T) {
	with.Context(func(ctx context.Context) {
		with.Logging(t, func(parent *with.LoggingHarness) {

			h := newHarness(parent.Logger)
			h.expectSystemContractCalled(deployments_systemcontract.CONTRACT_NAME, deployments_systemcontract.METHOD_GET_INFO, nil, uint32(protocol.PROCESSOR_TYPE_NATIVE)) // assume all contracts are deployed

			h.expectStateStorageLastCommittedBlockInfoBlockHeightRequested(12)
			h.expectNativeContractMethodCalled("Contract1", "method1", func(executionContextId primitives.ExecutionContextId, inputArgs *protocol.ArgumentArray) (protocol.ExecutionResult, *protocol.ArgumentArray, error) {
				return protocol.EXECUTION_RESULT_ERROR_SMART_CONTRACT, builders.ArgumentsArray(), errors.New("contract error")
			})

			result, outputArgs, refHeight, _, err := h.processQuery(ctx, "Contract1", "method1")
			require.Error(t, err, "process query should fail")
			require.Equal(t, protocol.EXECUTION_RESULT_ERROR_SMART_CONTRACT, result, "process query should return contract error")
			require.Equal(t, []byte{}, outputArgs, "process query should return matching output args")
			require.EqualValues(t, 12, refHeight)

			h.verifySystemContractCalled(t)
			h.verifyStateStorageBlockHeightRequested(t)
			h.verifyNativeContractMethodCalled(t)
		})
	})
}

func TestProcessQuery_UnexpectedError(t *testing.T) {
	with.Context(func(ctx context.Context) {
		with.Logging(t, func(parent *with.LoggingHarness) {

			h := newHarness(parent.Logger)
			h.expectSystemContractCalled(deployments_systemcontract.CONTRACT_NAME, deployments_systemcontract.METHOD_GET_INFO, nil, uint32(protocol.PROCESSOR_TYPE_NATIVE)) // assume all contracts are deployed

			h.expectStateStorageLastCommittedBlockInfoBlockHeightRequested(12)
			h.expectNativeContractMethodCalled("Contract1", "method1", func(executionContextId primitives.ExecutionContextId, inputArgs *protocol.ArgumentArray) (protocol.ExecutionResult, *protocol.ArgumentArray, error) {
				return protocol.EXECUTION_RESULT_ERROR_UNEXPECTED, builders.ArgumentsArray(), errors.New("unexpected error")
			})

			result, outputArgs, refHeight, _, err := h.processQuery(ctx, "Contract1", "method1")
			require.Error(t, err, "process query should fail")
			require.Equal(t, protocol.EXECUTION_RESULT_ERROR_UNEXPECTED, result, "process query should return unexpected error")
			require.Equal(t, []byte{}, outputArgs, "process query should return matching output args")
			require.EqualValues(t, 12, refHeight)

			h.verifySystemContractCalled(t)
			h.verifyStateStorageBlockHeightRequested(t)
			h.verifyNativeContractMethodCalled(t)
		})
	})
}

func TestProcessQuery_WithSpecificBlockHeight(t *testing.T) {
	with.Context(func(ctx context.Context) {
		with.Logging(t, func(parent *with.LoggingHarness) {

			h := newHarness(parent.Logger)

			h.expectStateStorageLastCommittedBlockInfoBlockHeightRequested(12)
			output, err := h.service.ProcessQuery(ctx, &services.ProcessQueryInput{
				BlockHeight: 123,
				SignedQuery: (&protocol.SignedQueryBuilder{
					Query: &protocol.QueryBuilder{
						Signer:             nil,
						ContractName:       "SomeContract",
						MethodName:         "SomeMethod",
						InputArgumentArray: []byte{},
					},
				}).Build(),
			})

			require.Error(t, err, "Run local method with specific block height is not yet supported")
			require.EqualValues(t, protocol.EXECUTION_RESULT_ERROR_INPUT, output.CallResult)
			require.EqualValues(t, 12, output.ReferenceBlockHeight)
			h.verifyStateStorageBlockHeightRequested(t)
		})
	})
}
