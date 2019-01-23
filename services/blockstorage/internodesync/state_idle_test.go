package internodesync

import (
	"context"
	"github.com/orbs-network/orbs-network-go/synchronization"
	"github.com/orbs-network/orbs-network-go/test"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStateIdle_StaysIdleOnIdleReset(t *testing.T) {
	test.WithContext(func(ctx context.Context) {
		manualNoCommitTimer := synchronization.NewTimerWithManualTick()
		h := newBlockSyncHarnessWithManualNoCommitTimeoutTimer(func() *synchronization.Timer {
			return manualNoCommitTimer
		})

		state := h.factory.CreateIdleState()
		nextState := h.processStateInBackgroundAndWaitUntilFinished(ctx, state, func() {
			h.factory.conduit.idleReset <- struct{}{}
			manualNoCommitTimer.ManualTick() // not required, added for completion (like in state_availability_requests_test)
		})

		require.IsType(t, &idleState{}, nextState, "nextState should still be idle")
		require.True(t, nextState != state, "processState state should be a different idle state (which was recreated so the timer starts from be beginning)")
	})
}

func TestStateIdle_MovesToCollectingAvailabilityResponsesOnNoCommitTimeout(t *testing.T) {
	test.WithContext(func(ctx context.Context) {
		h := newBlockSyncHarness()

		state := h.factory.CreateIdleState()
		nextState := state.processState(ctx)

		require.IsType(t, &collectingAvailabilityResponsesState{}, nextState, "processState state should be collecting availability responses")
	})
}

func TestStateIdle_TerminatesOnContextTermination(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	h := newBlockSyncHarness()

	cancel()
	state := h.factory.CreateIdleState()
	nextState := state.processState(ctx)

	require.Nil(t, nextState, "context termination should return a nil new state")
}
