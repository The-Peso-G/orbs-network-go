// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package benchmarkconsensus

import (
	"context"
	"github.com/orbs-network/crypto-lib-go/crypto/signer"
	"github.com/orbs-network/govnr"
	"github.com/orbs-network/orbs-network-go/instrumentation/logfields"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/protocol/consensus"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/gossiptopics"
	"github.com/orbs-network/orbs-spec/types/go/services/handlers"
	"github.com/orbs-network/scribe/log"
	"math"
	"sync"
	"time"
)

const blockHeightNone = primitives.BlockHeight(math.MaxUint64)

var LogTag = log.Service("consensus-algo-benchmark")

type Config interface {
	NodeAddress() primitives.NodeAddress
	BenchmarkConsensusConstantLeader() primitives.NodeAddress
	ActiveConsensusAlgo() consensus.ConsensusAlgoType
	BenchmarkConsensusRetryInterval() time.Duration
	BenchmarkConsensusRequiredQuorumPercentage() uint32
}

type Service struct {
	govnr.TreeSupervisor
	gossip           gossiptopics.BenchmarkConsensus
	blockStorage     services.BlockStorage
	consensusContext services.ConsensusContext
	signer           signer.Signer
	logger           log.Logger
	config           Config
	network          []primitives.NodeAddress

	isLeader                bool
	successfullyVotedBlocks chan primitives.BlockHeight // leader only

	mutex                                           sync.RWMutex
	lastCommittedBlockUnderMutex                    *protocol.BlockPairContainer
	lastSuccessfullyVotedBlock                      primitives.BlockHeight // leader only
	lastCommittedBlockVotersUnderMutex              map[string]bool        // leader only
	lastCommittedBlockVotersReachedQuorumUnderMutex bool                   // leader only

	metrics *metrics
}

type metrics struct {
	consensusRoundTickTime     *metric.Histogram
	failedConsensusTicksRate   *metric.Rate
	timedOutConsensusTicksRate *metric.Rate
	votingTime                 *metric.Histogram
	lastCommittedTime          *metric.Gauge
}

func newMetrics(m metric.Factory, consensusTimeout time.Duration, collectVotesTimeout time.Duration) *metrics {
	return &metrics{
		consensusRoundTickTime:     m.NewLatency("ConsensusAlgo.Benchmark.RoundTick.Millis", consensusTimeout),
		votingTime:                 m.NewLatency("ConsensusAlgo.Benchmark.Voting.Millis", collectVotesTimeout),
		failedConsensusTicksRate:   m.NewRate("ConsensusAlgo.Benchmark.FailedTicks.PerSecond"),
		timedOutConsensusTicksRate: m.NewRate("ConsensusAlgo.Benchmark.TimedOutTicks.PerSecond"),
		lastCommittedTime:          m.NewGauge("ConsensusAlgo.Benchmark.LastCommitted.TimeNano"),
	}
}

func NewBenchmarkConsensusAlgo(
	ctx context.Context,
	gossip gossiptopics.BenchmarkConsensus,
	blockStorage services.BlockStorage,
	consensusContext services.ConsensusContext,
	network []primitives.NodeAddress,
	signer signer.Signer,
	parentLogger log.Logger,
	config Config,
	metricFactory metric.Factory,
) *Service {

	logger := parentLogger.WithTags(LogTag)

	s := &Service{
		gossip:           gossip,
		blockStorage:     blockStorage,
		consensusContext: consensusContext,
		signer:           signer,
		logger:           logger,
		config:           config,
		network:          network,

		isLeader:                   config.BenchmarkConsensusConstantLeader().Equal(config.NodeAddress()),
		successfullyVotedBlocks:    make(chan primitives.BlockHeight), // leader only
		lastSuccessfullyVotedBlock: blockHeightNone,                   // leader only

		lastCommittedBlockVotersUnderMutex:              make(map[string]bool), // leader only
		lastCommittedBlockVotersReachedQuorumUnderMutex: false,                 // leader only

		metrics: newMetrics(metricFactory, config.BenchmarkConsensusRetryInterval(), config.BenchmarkConsensusRetryInterval()),
	}

	gossip.RegisterBenchmarkConsensusHandler(s)
	blockStorage.RegisterConsensusBlocksHandler(s)

	if config.ActiveConsensusAlgo() == consensus.CONSENSUS_ALGO_TYPE_BENCHMARK_CONSENSUS && s.isLeader {
		logger.Info("NewBenchmarkConsensusAlgo() Benchmark Consensus is active algo, and this node is leader, starting goroutine now")
		s.Supervise(govnr.Forever(ctx, "Benchmark consensus main loop", logfields.GovnrErrorer(logger), func() {
			s.leaderConsensusRoundRunLoop(ctx)
		}))
	}

	return s
}

func (s *Service) HandleBlockConsensus(ctx context.Context, input *handlers.HandleBlockConsensusInput) (*handlers.HandleBlockConsensusOutput, error) {
	return nil, s.handleBlockConsensusFromHandler(input.Mode, input.BlockType, input.BlockPair, input.PrevCommittedBlockPair)
}

func (s *Service) HandleBenchmarkConsensusCommit(ctx context.Context, input *gossiptopics.BenchmarkConsensusCommitInput) (*gossiptopics.EmptyOutput, error) {
	if !s.isLeader {
		return nil, s.nonLeaderHandleCommit(ctx, input.Message.BlockPair)
	}
	return nil, nil
}

func (s *Service) HandleBenchmarkConsensusCommitted(ctx context.Context, input *gossiptopics.BenchmarkConsensusCommittedInput) (*gossiptopics.EmptyOutput, error) {
	if s.isLeader {
		return nil, s.leaderHandleCommittedVote(ctx, input.Message.Sender, input.Message.Status)
	}
	return nil, nil
}
