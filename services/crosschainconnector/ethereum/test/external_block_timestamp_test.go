// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package test

import (
	"context"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/orbs-network/orbs-network-go/services/crosschainconnector/ethereum"
	"github.com/orbs-network/orbs-network-go/services/crosschainconnector/ethereum/contract"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"github.com/orbs-network/orbs-network-go/test/with"
	"github.com/stretchr/testify/require"
	"math/big"
	"strings"
	"testing"
	"time"
)

//TODO this test does not deal well with gaps in ganache, meaning that a ganache that has been standing idle for more than a few seconds will fail this test while
// trying to assert that a contract cannot be called before it has been deployed
func TestFullFlowWithVaryingTimestamps(t *testing.T) {
	// the idea of this test is to make sure that the entire 'call-from-ethereum' logic works on a specific timestamp and different states in time (blocks)
	// it requires ganache or some other RPC backend to transact

	if !runningWithDocker() {
		t.Skip("this test relies on external components - ganache, and will be skipped unless running in docker")
	}

	with.Context(func(ctx context.Context) {
		with.Logging(t, func(parent *with.LoggingHarness) {

			h := newRpcEthereumConnectorHarness(parent.Logger, ConfigForExternalRPCConnection())

			// pad Ganache nicely so that any previous test
			h.moveBlocksInGanache(t, ctx, 100, 1)
			blockAtStart, err := h.rpcAdapter.HeaderByNumber(ctx, nil)
			require.NoError(t, err, "failed to get latest block in ganache")
			t.Logf("block at start: %d | %d", blockAtStart.BlockNumber, blockAtStart.TimeInSeconds)

			time.Sleep(time.Second)

			expectedTextFromEthereum := "test3"
			contractAddress, err := h.deployRpcStorageContract(expectedTextFromEthereum)
			require.NoError(t, err, "failed deploying contract to Ethereum")

			blockAtDeploy, err := h.rpcAdapter.HeaderByNumber(ctx, nil)
			require.NoError(t, err, "failed to get latest block in ganache")
			t.Logf("block at deploy: %d | %d", blockAtDeploy.BlockNumber, blockAtDeploy.TimeInSeconds)

			t.Logf("finality is %f seconds, %d blocks", h.config.finalityTimeComponent.Seconds(), h.config.finalityBlocksComponent)
			h.moveBlocksInGanache(t, ctx, int(h.config.finalityBlocksComponent+1), 1) // finality blocks + block we will request below of because of the finder algo

			blockAfterPad, err := h.rpcAdapter.HeaderByNumber(ctx, nil)
			require.NoError(t, err, "failed to get latest block in ganache")
			referenceTime := time.Unix(int64(blockAfterPad.TimeInSeconds), 0)

			methodToCall := "getValues"
			parsedABI, err := abi.JSON(strings.NewReader(contract.SimpleStorageABI))
			require.NoError(t, err, "abi parse failed for simple storage contract")

			ethCallData, err := ethereum.ABIPackFunctionInputArguments(parsedABI, methodToCall, nil)
			require.NoError(t, err, "this means we couldn't pack the params for ethereum, something is broken with the harness")

			// request at time now, which should be (with finality) after the contract was deployed
			input := builders.EthereumCallContractInput().
				WithTimestamp(referenceTime).
				WithContractAddress(contractAddress).
				WithAbi(contract.SimpleStorageABI).
				WithFunctionName(methodToCall).
				WithPackedArguments(ethCallData).
				Build()

			t.Logf("going to request from ethereum at time %d, rounded to secs %d", input.ReferenceTimestamp, time.Unix(0, int64(input.ReferenceTimestamp.KeyForMap())).Unix())
			output, err := h.connector.EthereumCallContract(ctx, input)
			require.NoError(t, err, "expecting call to succeed")
			require.True(t, len(output.EthereumAbiPackedOutput) > 0, "expecting output to have some data")
			ret := new(struct { // this is the expected return type of that ethereum call for the SimpleStorage contract getValues
				IntValue    *big.Int
				StringValue string
			})

			ethereum.ABIUnpackFunctionOutputArguments(parsedABI, ret, methodToCall, output.EthereumAbiPackedOutput)
			require.Equal(t, expectedTextFromEthereum, ret.StringValue, "text part from eth")

			timeAtStart := time.Unix(int64(blockAtStart.TimeInSeconds), 0)
			input = builders.EthereumCallContractInput().
				WithTimestamp(timeAtStart).
				WithContractAddress(contractAddress).
				WithAbi(contract.SimpleStorageABI).
				WithFunctionName(methodToCall).
				WithPackedArguments(ethCallData).
				Build()

			output, err = h.connector.EthereumCallContract(ctx, input)
			require.Error(t, err, "expecting call to fail as contract is not yet deployed in a past time block")
		})
	})
}
