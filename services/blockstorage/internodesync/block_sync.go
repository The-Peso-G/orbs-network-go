// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package internodesync

import (
	"context"
	"github.com/orbs-network/govnr"
	"github.com/orbs-network/orbs-network-go/instrumentation/logfields"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/instrumentation/trace"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/protocol/gossipmessages"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/gossiptopics"
	"github.com/orbs-network/scribe/log"
	"time"
)

var LogTag = log.String("flow", "Block-sync")

// this is coupled to gossip because the entire service is (Block storage)
// nothing to gain right now in decoupling just the sync
type syncState interface {
	name() string
	String() string
	processState(ctx context.Context) syncState
}

type blockSyncConfig interface {
	NodeAddress() primitives.NodeAddress
	BlockSyncNumBlocksInBatch() uint32
	BlockSyncNoCommitInterval() time.Duration
	BlockSyncCollectResponseTimeout() time.Duration
	BlockSyncCollectChunksTimeout() time.Duration
	BlockSyncDescendingActivationDate() string
	BlockSyncReferenceMaxAllowedDistance() time.Duration
	ManagementReferenceGraceTimeout() time.Duration
}

type BlockSyncStorage interface {
	GetLastCommittedBlockHeight(ctx context.Context, input *services.GetLastCommittedBlockHeightInput) (*services.GetLastCommittedBlockHeightOutput, error)
	NodeSyncCommitBlock(ctx context.Context, input *services.CommitBlockInput) (*services.CommitBlockOutput, error)
	ValidateBlockForCommit(ctx context.Context, input *services.ValidateBlockForCommitInput) (*services.ValidateBlockForCommitOutput, error)
	UpdateConsensusAlgosAboutLastCommittedBlockInLocalPersistence(ctx context.Context)
	GetLastCommittedBlock() (*protocol.BlockPairContainer, error)
}

// state machine passes outside events into this channel type for consumption by the currently active state instance.
// within processState.processState() all states must read from the channel eagerly!
// keeping the channel clear for new incoming events and tossing out irrelevant messages.
type blockSyncConduit chan interface{}

func (c blockSyncConduit) drainAndCheckForShutdown(ctx context.Context) bool {
	for {
		select {
		case <-c: // nop
		case <-ctx.Done():
			return false // indicate a shutdown was signaled
		default:
			return true
		}
	}
}

type BlockSync struct {
	govnr.TreeSupervisor
	logger                     log.Logger
	factory                    *stateFactory
	gossip                     gossiptopics.BlockSync
	storage                    BlockSyncStorage
	conduit                    blockSyncConduit
	metrics                    *stateMachineMetrics
	config                     blockSyncConfig
	blocksOrderActivationTimer *time.Timer
}

type stateMachineMetrics struct {
	statesTransitioned *metric.Gauge
}

func newStateMachineMetrics(factory metric.Factory) *stateMachineMetrics {
	return &stateMachineMetrics{
		statesTransitioned: factory.NewGauge("BlockSync.StateTransitions.Count"),
	}
}

func newBlockSyncWithFactory(ctx context.Context, config blockSyncConfig, factory *stateFactory, gossip gossiptopics.BlockSync, storage BlockSyncStorage, logger log.Logger, metricFactory metric.Factory) *BlockSync {
	metrics := newStateMachineMetrics(metricFactory)

	bs := &BlockSync{
		logger:  logger,
		factory: factory,
		gossip:  gossip,
		storage: storage,
		conduit: factory.conduit,
		metrics: metrics,
		config:  config,
	}

	logger.Info("Block sync init",
		log.Stringable("no-commit-timeout", bs.factory.config.BlockSyncNoCommitInterval()),
		log.Stringable("collect-responses-timeout", bs.factory.config.BlockSyncCollectResponseTimeout()),
		log.Stringable("collect-chunks-timeout", bs.factory.config.BlockSyncCollectChunksTimeout()),
		log.Uint32("batch-size", bs.factory.config.BlockSyncNumBlocksInBatch()),
		log.String("descending-activation-date", bs.factory.config.BlockSyncDescendingActivationDate()),
		log.Stringable("max-reference-distance", bs.factory.config.BlockSyncReferenceMaxAllowedDistance()),
		log.Stringable("management-reference-grace-timeout", bs.factory.config.ManagementReferenceGraceTimeout()))

	setupSyncBlocksOrder(bs, config.BlockSyncDescendingActivationDate(), logger)

	bs.Supervise(govnr.Forever(ctx, "Node sync state machine", logfields.GovnrErrorer(logger), func() {
		bs.syncLoop(ctx)
	}))

	return bs
}

func setupSyncBlocksOrder(bs *BlockSync, descendingActivationDateStr string, logger log.Logger) {
	activationDate, err := time.Parse(time.RFC3339, descendingActivationDateStr)
	if err != nil {
		logger.Error("BlockSync failed to parse descending blocks order activation date", log.Error(err))
		bs.factory.SetSyncBlocksOrder(gossipmessages.SYNC_BLOCKS_ORDER_DESCENDING)
		return
	}

	if timeUntilActivation := activationDate.Sub(time.Now()); timeUntilActivation < 0 {
		bs.factory.SetSyncBlocksOrder(gossipmessages.SYNC_BLOCKS_ORDER_DESCENDING)
	} else {
		bs.factory.SetSyncBlocksOrder(gossipmessages.SYNC_BLOCKS_ORDER_ASCENDING)
		bs.blocksOrderActivationTimer = time.AfterFunc(timeUntilActivation, func() {
			bs.factory.SetSyncBlocksOrder(gossipmessages.SYNC_BLOCKS_ORDER_DESCENDING)
		})
	}
}

func NewBlockSync(ctx context.Context, config blockSyncConfig, gossip gossiptopics.BlockSync, storage BlockSyncStorage, management services.Management, parentLogger log.Logger, metricFactory metric.Factory) *BlockSync {
	logger := parentLogger.WithTags(LogTag)

	conduit := make(blockSyncConduit)
	return newBlockSyncWithFactory(
		ctx,
		config,
		NewStateFactory(config, gossip, storage, conduit, management, logger, metricFactory),
		gossip,
		storage,
		logger,
		metricFactory,
	)
}

func (bs *BlockSync) deactivateBlocksOrderTimer() {
	if bs.blocksOrderActivationTimer != nil {
		bs.blocksOrderActivationTimer.Stop()
	}
}

func (bs *BlockSync) syncLoop(parent context.Context) {
	defer bs.deactivateBlocksOrderTimer()
	for currentState := bs.factory.CreateCollectingAvailabilityResponseState(); currentState != nil; {
		ctx := trace.NewContext(parent, "BlockSync")
		bs.logger.Info("state transitioning", log.Stringable("current-state", currentState), trace.LogFieldFrom(ctx))

		currentState = currentState.processState(ctx)
		bs.metrics.statesTransitioned.Inc()
	}
}

func (bs *BlockSync) HandleBlockCommitted(ctx context.Context) {
	logger := bs.logger.WithTags(trace.LogFieldFrom(ctx))

	bs.updateStorageSyncState()
	select {
	case bs.conduit <- idleResetMessage{}:
	case <-ctx.Done():
		logger.Info("terminated on handle Block committed", log.Error(ctx.Err()))
	}
}

func (bs *BlockSync) HandleBlockAvailabilityResponse(ctx context.Context, input *gossiptopics.BlockAvailabilityResponseInput) (*gossiptopics.EmptyOutput, error) {
	logger := bs.logger.WithTags(trace.LogFieldFrom(ctx))

	select {
	case bs.conduit <- input.Message:
	case <-ctx.Done():
		logger.Info("terminated on writing new availability response",
			log.String("context-message", ctx.Err().Error()),
			log.Stringable("response-source", input.Message.Sender.SenderNodeAddress()))
	}
	return nil, nil
}

func (bs *BlockSync) HandleBlockSyncResponse(ctx context.Context, input *gossiptopics.BlockSyncResponseInput) (*gossiptopics.EmptyOutput, error) {
	logger := bs.logger.WithTags(trace.LogFieldFrom(ctx))

	select {
	case bs.conduit <- input.Message:
	case <-ctx.Done():
		logger.Info("terminated on writing new Block chunk message",
			log.String("context-message", ctx.Err().Error()),
			log.Stringable("message-sender", input.Message.Sender.SenderNodeAddress()))
	}

	return nil, nil
}


func (bs *BlockSync) GetStorageSyncState() *StorageSyncState {
	return bs.factory.GetTempStorageSyncState()
}

func (bs *BlockSync) updateStorageSyncState() {
	if topBlock, err := bs.storage.GetLastCommittedBlock(); err == nil{
		bs.factory.NotifyTempStorageSyncState(topBlock)
	} else {
		bs.logger.Info("failed to retrieve last committed block")
	}
}