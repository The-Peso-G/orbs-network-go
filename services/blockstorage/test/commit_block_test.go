// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package test

import (
	"context"
	"fmt"
	"github.com/orbs-network/go-mock"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"github.com/orbs-network/orbs-network-go/test/with"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/services/handlers"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCommitBlockSavesToPersistentStorage(t *testing.T) {
	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		harness := newBlockStorageHarness(parent).
			withSyncBroadcast(1).
			expectValidateConsensusAlgos().
			start(ctx)

		blockCreated := time.Now()
		blockHeight := primitives.BlockHeight(1)

		_, err := harness.commitBlock(ctx, builders.BlockPair().WithHeight(blockHeight).WithBlockCreated(blockCreated).Build())

		require.NoError(t, err)
		require.EqualValues(t, 1, harness.numOfWrittenBlocks())

		harness.verifyMocks(t, 1)

		lastCommittedBlockHeight := harness.getLastBlockHeight(ctx, t)

		require.EqualValues(t, blockHeight, lastCommittedBlockHeight.LastCommittedBlockHeight, "block height in storage should be the same")
		require.EqualValues(t, blockCreated.UnixNano(), lastCommittedBlockHeight.LastCommittedBlockTimestamp, "timestamp in storage should be the same")

	})
	// TODO(https://github.com/orbs-network/orbs-network-go/issues/569) Spec: If any of the intra block syncs (StateStorage, TransactionPool) is blocking and waiting, wake it up.
}

func TestCommitBlockDoesNotUpdateCommittedBlockHeightAndTimestampIfStorageFails(t *testing.T) {
	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		harness := newBlockStorageHarness(parent).
			withSyncBroadcast(1).
			expectValidateConsensusAlgos().
			start(ctx)

		blockCreated := time.Now()
		blockHeight := primitives.BlockHeight(1)

		harness.commitBlock(ctx, builders.BlockPair().WithHeight(blockHeight).WithBlockCreated(blockCreated).Build())
		require.EqualValues(t, 1, harness.numOfWrittenBlocks())

		harness.failNextBlocks()

		_, err := harness.commitBlock(ctx, builders.BlockPair().WithHeight(blockHeight+1).Build())
		require.EqualError(t, err, "intentionally failing (tampering with) WriteNextBlock() height 2", "error should be returned if storage fails")

		harness.verifyMocks(t, 1)

		lastCommittedBlockHeight := harness.getLastBlockHeight(ctx, t)

		require.EqualValues(t, blockHeight, lastCommittedBlockHeight.LastCommittedBlockHeight, "block height should not update as storage was unavailable")
		require.EqualValues(t, blockCreated.UnixNano(), lastCommittedBlockHeight.LastCommittedBlockTimestamp, "timestamp should not update as storage was unavailable")

	})
}

func TestCommitBlockReturnsErrorWhenProtocolVersionMismatches(t *testing.T) {
	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		harness := newBlockStorageHarness(parent).
			withSyncBroadcast(1).
			expectValidateConsensusAlgos().
			allowingErrorsMatching("protocol version mismatch in transactions block header").
			start(ctx)

		_, err := harness.commitBlock(ctx, builders.BlockPair().WithProtocolVersion(config.MAXIMAL_PROTOCOL_VERSION_SUPPORTED_VALUE+1).Build())

		require.EqualError(t, err, fmt.Sprintf("protocol version (%d) higher than maximal supported (%d) in transactions block header", config.MAXIMAL_PROTOCOL_VERSION_SUPPORTED_VALUE+1, config.MAXIMAL_PROTOCOL_VERSION_SUPPORTED_VALUE))
	})
}

func TestCommitBlockDiscardsBlockIfAlreadyExists(t *testing.T) {
	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		harness := newBlockStorageHarness(parent).
			withSyncBroadcast(1).
			expectValidateConsensusAlgos().
			start(ctx)
		block1 := builders.BlockPair().WithHeight(1).Build()
		block2 := builders.BlockPair().WithHeight(2).Build()

		_, err := harness.commitBlock(ctx, block1)
		require.NoError(t, err)

		_, err = harness.commitBlock(ctx, block2)
		require.NoError(t, err)

		_, err = harness.commitBlock(ctx, block1)
		require.NoError(t, err, "expected existing block to be silently ignored")

		_, err = harness.commitBlock(ctx, block2)
		require.NoError(t, err, "expected existing block to be silently ignored")

		require.EqualValues(t, 2, harness.numOfWrittenBlocks(), "block should be written only once")
		harness.verifyMocks(t, 1)
	})
}

func TestCommitBlockReturnsErrorIfBlockExistsButHasDifferentTimestamp(t *testing.T) {
	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		harness := newBlockStorageHarness(parent).
			allowingErrorsMatching("FORK!! block already in storage, timestamp mismatch").
			withSyncBroadcast(1).
			expectValidateConsensusAlgos().
			start(ctx)

		blockPair := builders.BlockPair()
		harness.commitBlock(ctx, blockPair.Build())

		mutatedBlockPair := blockPair.WithBlockCreated(time.Now().Add(1 * time.Hour)).Build()
		_, err := harness.commitBlock(ctx, mutatedBlockPair)

		require.EqualError(t, err, "FORK!! block already in storage, timestamp mismatch", "same block, different timestamp should return an error")
		require.EqualValues(t, 1, harness.numOfWrittenBlocks(), "only one block should have been written")
		harness.verifyMocks(t, 1)
	})
}

func TestCommitBlockReturnsErrorIfBlockExistsButHasDifferentTxBlock(t *testing.T) {
	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		harness := newBlockStorageHarness(parent).
			allowingErrorsMatching("FORK!! block already in storage, transaction block header mismatch").
			withSyncBroadcast(1).
			expectValidateConsensusAlgos().
			start(ctx)

		blockPair := builders.BlockPair()
		harness.commitBlock(ctx, blockPair.Build())

		mutatedBlock := blockPair.Build()
		mutatedBlock.TransactionsBlock.Header.MutateNumSignedTransactions(999)

		_, err := harness.commitBlock(ctx, mutatedBlock)

		require.EqualError(t, err, "FORK!! block already in storage, transaction block header mismatch", "same block, different timestamp should return an error")
		require.EqualValues(t, 1, harness.numOfWrittenBlocks(), "only one block should have been written")
		harness.verifyMocks(t, 1)
	})
}

func TestCommitBlockReturnsErrorIfBlockExistsButHasDifferentRxBlock(t *testing.T) {
	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		harness := newBlockStorageHarness(parent).
			allowingErrorsMatching("FORK!! block already in storage, results block header mismatch").
			withSyncBroadcast(1).
			expectValidateConsensusAlgos().
			start(ctx)

		blockPair := builders.BlockPair()
		harness.commitBlock(ctx, blockPair.Build())

		mutatedBlock := blockPair.Build()
		mutatedBlock.ResultsBlock.Header.MutateNumTransactionReceipts(999)

		_, err := harness.commitBlock(ctx, mutatedBlock)

		require.EqualError(t, err, "FORK!! block already in storage, results block header mismatch", "same block, different timestamp should return an error")
		require.EqualValues(t, 1, harness.numOfWrittenBlocks(), "only one block should have been written")
		harness.verifyMocks(t, 1)
	})
}

func TestCommitBlockReturnsErrorIfBlockInFuture(t *testing.T) {
	t.Skip("Does not comply with current implementation")
	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		harness := newBlockStorageHarness(parent).
			withSyncBroadcast(1).
			expectValidateConsensusAlgos().
			start(ctx)

		now := time.Now()
		block1 := builders.BlockPair().WithHeight(1).WithTransactions(1).WithBlockCreated(now).Build()
		block2 := builders.BlockPair().WithHeight(2).WithPrevBlock(block1).Build()

		_, err := harness.commitBlock(ctx, block1)
		require.NoError(t, err)
		_, err = harness.commitBlock(ctx, block2)
		require.NoError(t, err)

		futureBlock := builders.BlockPair().WithHeight(1000).Build()

		_, err = harness.commitBlock(ctx, futureBlock)
		require.EqualError(t, err, "attempt to write future block 1000. current top height is 2", "block height was mutate to be invalid, should return an error")
		require.EqualValues(t, 2, harness.numOfWrittenBlocks(), "only 2 blocks should have been written")
		harness.verifyMocks(t, 1)
	})
}

func TestCommitBlockUpdatesSync_NoRacesOnShutdown(t *testing.T) {
	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		harness := newBlockStorageHarness(parent).withSyncBroadcast(1)

		harness.consensus.When("HandleBlockConsensus", mock.Any, mock.Any).Timeout(20*time.Millisecond).Return(&handlers.HandleBlockConsensusOutput{}, nil).AtLeast(1)
		harness.start(ctx)

		_, err := harness.commitBlock(ctx, builders.BlockPair().WithHeight(1).Build())
		require.NoError(t, err)
	})
}
