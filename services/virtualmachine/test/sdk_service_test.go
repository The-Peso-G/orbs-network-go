package test

import (
	"github.com/orbs-network/orbs-network-go/services/processor/native"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSdkServiceIsNative(t *testing.T) {
	h := newHarness()

	h.expectStateStorageBlockHeightRequested(12)
	h.expectNativeContractMethodCalled("Contract1", "method1", func(contextId primitives.ExecutionContextId) (protocol.ExecutionResult, error) {
		t.Log("First isNative on unknown contract")

		_, err := h.handleSdkCall(contextId, native.SDK_SERVICE_CONTRACT_NAME, "isNative", "UnknownContract")
		require.Error(t, err, "handleSdkCall should fail")

		t.Log("Second isNative on known contract")

		_, err = h.handleSdkCall(contextId, native.SDK_SERVICE_CONTRACT_NAME, "isNative", "NativeContract")
		require.NoError(t, err, "handleSdkCall should not fail")

		return protocol.EXECUTION_RESULT_SUCCESS, nil
	})
	h.expectNativeContractInfoRequested("UnknownContract", errors.New("unknown contract"))
	h.expectNativeContractInfoRequested("NativeContract", nil)

	h.runLocalMethod("Contract1", "method1")

	h.verifyStateStorageBlockHeightRequested(t)
	h.verifyNativeContractMethodCalled(t)
	h.verifyNativeContractInfoRequested(t)
}

func TestSdkServiceCallMethodFailingCall(t *testing.T) {
	h := newHarness()

	h.expectNativeContractMethodCalled("Contract1", "method1", func(contextId primitives.ExecutionContextId) (protocol.ExecutionResult, error) {
		t.Log("CallMethod on failing contract")

		_, err := h.handleSdkCall(contextId, native.SDK_SERVICE_CONTRACT_NAME, "callMethod", "FailingContract", "method1")
		require.Error(t, err, "handleSdkCall should fail")

		return protocol.EXECUTION_RESULT_SUCCESS, nil
	})
	h.expectNativeContractMethodCalled("FailingContract", "method1", func(contextId primitives.ExecutionContextId) (protocol.ExecutionResult, error) {
		return protocol.EXECUTION_RESULT_ERROR_UNEXPECTED, errors.New("call error")
	})

	h.processTransactionSet([]*contractAndMethod{
		{"Contract1", "method1"},
	})

	h.verifyNativeContractMethodCalled(t)
}

func TestSdkServiceCallMethodMaintainsAddressSpaceUnderSameContract(t *testing.T) {
	h := newHarness()

	h.expectNativeContractMethodCalled("Contract1", "method1", func(contextId primitives.ExecutionContextId) (protocol.ExecutionResult, error) {
		t.Log("Write to key in first contract")

		_, err := h.handleSdkCall(contextId, native.SDK_STATE_CONTRACT_NAME, "write", []byte{0x01}, []byte{0x02, 0x03})
		require.NoError(t, err, "handleSdkCall should succeed")

		t.Log("CallMethod on a the same contract")

		_, err = h.handleSdkCall(contextId, native.SDK_SERVICE_CONTRACT_NAME, "callMethod", "Contract1", "method2")
		require.NoError(t, err, "handleSdkCall should succeed")

		return protocol.EXECUTION_RESULT_SUCCESS, nil
	})
	h.expectNativeContractMethodCalled("Contract1", "method2", func(contextId primitives.ExecutionContextId) (protocol.ExecutionResult, error) {
		t.Log("Read the same key in the first contract")

		res, err := h.handleSdkCall(contextId, native.SDK_STATE_CONTRACT_NAME, "read", []byte{0x01})
		require.NoError(t, err, "handleSdkCall should not fail")
		require.Equal(t, []byte{0x02, 0x03}, res[0].BytesValue(), "handleSdkCall result should be equal")

		return protocol.EXECUTION_RESULT_SUCCESS, nil
	})
	h.expectStateStorageNotRead()

	h.processTransactionSet([]*contractAndMethod{
		{"Contract1", "method1"},
	})

	h.verifyNativeContractMethodCalled(t)
	h.verifyStateStorageRead(t)
}

func TestSdkServiceCallMethodChangesAddressSpaceBetweenContracts(t *testing.T) {
	h := newHarness()

	h.expectNativeContractMethodCalled("Contract1", "method1", func(contextId primitives.ExecutionContextId) (protocol.ExecutionResult, error) {
		t.Log("Write to key in first contract")

		_, err := h.handleSdkCall(contextId, native.SDK_STATE_CONTRACT_NAME, "write", []byte{0x01}, []byte{0x02, 0x03})
		require.NoError(t, err, "handleSdkCall should succeed")

		t.Log("CallMethod on a different contract")

		_, err = h.handleSdkCall(contextId, native.SDK_SERVICE_CONTRACT_NAME, "callMethod", "Contract2", "method1")
		require.NoError(t, err, "handleSdkCall should succeed")

		t.Log("Read the same key in the first contract after the call")

		res, err := h.handleSdkCall(contextId, native.SDK_STATE_CONTRACT_NAME, "read", []byte{0x01})
		require.NoError(t, err, "handleSdkCall should not fail")
		require.Equal(t, []byte{0x02, 0x03}, res[0].BytesValue(), "handleSdkCall result should be equal")

		return protocol.EXECUTION_RESULT_SUCCESS, nil
	})
	h.expectNativeContractMethodCalled("Contract2", "method1", func(contextId primitives.ExecutionContextId) (protocol.ExecutionResult, error) {
		t.Log("Read the same key in the second contract")

		res, err := h.handleSdkCall(contextId, native.SDK_STATE_CONTRACT_NAME, "read", []byte{0x01})
		require.NoError(t, err, "handleSdkCall should not fail")
		require.Equal(t, []byte{0x04, 0x05, 0x06}, res[0].BytesValue(), "handleSdkCall result should be equal")

		return protocol.EXECUTION_RESULT_SUCCESS, nil
	})
	h.expectStateStorageRead(11, "Contract2", []byte{0x01}, []byte{0x04, 0x05, 0x06})

	h.processTransactionSet([]*contractAndMethod{
		{"Contract1", "method1"},
	})

	h.verifyNativeContractMethodCalled(t)
	h.verifyStateStorageRead(t)
}
