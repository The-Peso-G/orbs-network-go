// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package transactionpool

import (
	"context"
	"github.com/orbs-network/crypto-lib-go/crypto/signer"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/services/transactionpool/adapter"
	"github.com/orbs-network/orbs-network-go/synchronization"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/gossiptopics"
	"github.com/orbs-network/orbs-spec/types/go/services/handlers"
	"github.com/orbs-network/scribe/log"
)

func NewTransactionPool(ctx context.Context,
	maybeClock adapter.Clock,
	gossip gossiptopics.TransactionRelay,
	virtualMachine services.VirtualMachine,
	signer signer.Signer,
	blockHeightReporter BlockHeightReporter,
	config config.TransactionPoolConfig,
	parent log.Logger,
	metricFactory metric.Factory) *service {

	if blockHeightReporter == nil {
		blockHeightReporter = synchronization.NopHeightReporter{}
	}
	waiter := newTransactionWaiter()
	onNewTransaction := func() { waiter.inc(ctx) }
	pendingPool := NewPendingPool(config.TransactionPoolPendingPoolSizeInBytes, metricFactory, onNewTransaction)
	committedPool := NewCommittedPool(config.TransactionPoolFutureTimestampGraceTimeout, metricFactory)

	logger := parent.WithTags(LogTag)

	txForwarder := NewTransactionForwarder(ctx, logger, signer, config, gossip)

	s := &service{
		clock:           createClockIfNeeded(maybeClock),
		gossip:          gossip,
		virtualMachine:  virtualMachine,
		config:          config,
		logger:          logger,

		pendingPool:                         pendingPool,
		committedPool:                       committedPool,
		blockTracker:                        synchronization.NewBlockTracker(logger, 0, uint16(config.BlockTrackerGraceDistance())),
		blockHeightReporter:                 blockHeightReporter,
		transactionForwarder:                txForwarder,
		transactionWaiter:                   waiter,
		addNewTransactionConcurrencyLimiter: NewRequestConcurrencyLimiter(100),
	}

	s.validationContext = s.createValidationContext()
	s.lastCommitted.timestamp = primitives.TimestampNano(0) // this is so that we reject transactions on startup, before any block has been committed
	s.metrics.blockHeight = metricFactory.NewGauge("TransactionPool.BlockHeight")
	s.metrics.lastCommittedTimestamp = metricFactory.NewGauge("TransactionPool.LastCommitted.TimeNano")
	s.metrics.commitRate = metricFactory.NewRate("TransactionPool.CommitRate.PerSecond")
	s.metrics.commitCount = metricFactory.NewGauge("TransactionPool.TotalCommits.Count")

	gossip.RegisterTransactionRelayHandler(s)
	pendingPool.onTransactionRemoved = s.onTransactionError

	s.Supervise(startCleaningProcess(ctx, "committed pool", config.TransactionPoolCommittedPoolClearExpiredInterval, config.TransactionExpirationWindow, s.committedPool, s.lastCommittedBlockHeightAndTime, logger))
	s.Supervise(startCleaningProcess(ctx, "pending pool", config.TransactionPoolPendingPoolClearExpiredInterval, config.TransactionExpirationWindow, s.pendingPool, s.lastCommittedBlockHeightAndTime, logger))
	s.Supervise(txForwarder)

	return s
}

func createClockIfNeeded(maybeClock adapter.Clock) adapter.Clock {
	if maybeClock == nil {
		return adapter.NewSystemClock()
	}

	return maybeClock
}

func (s *service) onTransactionError(ctx context.Context, txHash primitives.Sha256, removalReason protocol.TransactionStatus) {
	bh, ts := s.lastCommittedBlockHeightAndTime()
	if removalReason != protocol.TRANSACTION_STATUS_COMMITTED {
		s.transactionResultsHandlers.RLock()
		defer s.transactionResultsHandlers.RUnlock()
		for _, trh := range s.transactionResultsHandlers.handlers {
			_, err := trh.HandleTransactionError(ctx, &handlers.HandleTransactionErrorInput{
				Txhash:            txHash,
				TransactionStatus: removalReason,
				BlockTimestamp:    ts,
				BlockHeight:       bh,
			})
			if err != nil {
				s.logger.Info("notify tx error failed", log.Error(err))
			}
		}
	}
}
