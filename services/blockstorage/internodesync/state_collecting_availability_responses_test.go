// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package internodesync

import (
	"context"
	"github.com/orbs-network/orbs-network-go/synchronization"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"github.com/orbs-network/orbs-network-go/test/with"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStateCollectingAvailabilityResponses_ReturnsToIdleOnGossipError(t *testing.T) {
	with.Context(func(ctx context.Context) {
		with.Logging(t, func(harness *with.LoggingHarness) {
			h := newBlockSyncHarness(harness.Logger)

			h.expectUpdateConsensusAlgosAboutLastCommittedBlockInLocalPersistence(10)
			h.expectBroadcastOfBlockAvailabilityRequestToFail()

			state := h.factory.CreateCollectingAvailabilityResponseState()
			nextState := state.processState(ctx)

			require.IsType(t, &idleState{}, nextState, "next state should be idle on gossip error")
			h.eventuallyVerifyMocks(t, 1)
		})
	})
}

func TestStateCollectingAvailabilityResponses_ReturnsToIdleOnInvalidRequestSizeConfig(t *testing.T) {
	with.Context(func(ctx context.Context) {
		with.Logging(t, func(harness *with.LoggingHarness) {
			// this can probably happen only if BatchSize config is invalid
			h := newBlockSyncHarness(harness.Logger).withBatchSize(0)

			h.expectUpdateConsensusAlgosAboutLastCommittedBlockInLocalPersistence(0) // new server
			h.storage.When("GetSyncState").Return(nil).Times(1)

			state := h.factory.CreateCollectingAvailabilityResponseState()
			nextState := state.processState(ctx)

			require.IsType(t, &idleState{}, nextState, "next state should be idle on gossip error flow")
			h.eventuallyVerifyMocks(t, 1)
		})
	})
}

func TestStateCollectingAvailabilityResponses_MovesToFinishedCollecting(t *testing.T) {
	with.Context(func(ctx context.Context) {
		with.Logging(t, func(harness *with.LoggingHarness) {
			manualCollectResponsesTimer := synchronization.NewTimerWithManualTick()
			h := newBlockSyncHarnessWithCollectResponsesTimer(harness.Logger, func() *synchronization.Timer {
				return manualCollectResponsesTimer
			})

			h.expectUpdateConsensusAlgosAboutLastCommittedBlockInLocalPersistence(10)
			h.expectBroadcastOfBlockAvailabilityRequest()

			message := builders.BlockAvailabilityResponseInput().Build().Message
			state := h.factory.CreateCollectingAvailabilityResponseState()
			nextState := h.processStateInBackgroundAndWaitUntilFinished(ctx, state, func() {
				h.verifyBroadcastOfBlockAvailabilityRequest(t)
				h.factory.conduit <- message
				manualCollectResponsesTimer.ManualTick()
			})

			require.IsType(t, &finishedCARState{}, nextState, "state should transition to finished CAR")
			fcar := nextState.(*finishedCARState)
			require.Equal(t, 1, len(fcar.responses), "there should be one response")
			require.Equal(t, message.Sender, fcar.responses[0].Sender, "state sender should match message sender")
			require.Equal(t, message.SignedBatchRange, fcar.responses[0].SignedBatchRange, "state payload should match message")

			h.eventuallyVerifyMocks(t, 1)
		})
	})
}

func TestStateCollectingAvailabilityResponses_ContextTermination(t *testing.T) {
	with.Logging(t, func(harness *with.LoggingHarness) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		h := newBlockSyncHarnessWithCollectResponsesTimer(harness.Logger, func() *synchronization.Timer {
			return synchronization.NewTimerWithManualTick() // so that it deterministically doesn't tick
		})

		h.expectUpdateConsensusAlgosAboutLastCommittedBlockInLocalPersistence(10)
		h.expectBroadcastOfBlockAvailabilityRequest()

		state := h.factory.CreateCollectingAvailabilityResponseState()
		nextState := state.processState(ctx)

		require.Nil(t, nextState, "context terminated, next state should be nil")

		h.eventuallyVerifyMocks(t, 1)
	})
}
