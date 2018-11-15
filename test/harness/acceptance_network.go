package harness

import (
	"context"
	"github.com/orbs-network/orbs-contract-sdk/go/sdk"
	"github.com/orbs-network/orbs-network-go/bootstrap/inmemory"
	"github.com/orbs-network/orbs-network-go/instrumentation/log"
	"github.com/orbs-network/orbs-network-go/test/harness/contracts"
	blockStorageAdapter "github.com/orbs-network/orbs-network-go/test/harness/services/blockstorage/adapter"
	testGossipAdapter "github.com/orbs-network/orbs-network-go/test/harness/services/gossip/adapter"
	nativeProcessorAdapter "github.com/orbs-network/orbs-network-go/test/harness/services/processor/native/adapter"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
)

type TestNetworkDriver interface {
	inmemory.NetworkDriver
	GetBenchmarkTokenContract() contracts.BenchmarkTokenClient
	TransportTamperer() testGossipAdapter.Tamperer
	Description() string
	BlockPersistence(nodeIndex int) blockStorageAdapter.InMemoryBlockPersistence
	DumpState()
	WaitForTransactionInNodeState(ctx context.Context, txhash primitives.Sha256, nodeIndex int,)
	MockContract(fakeContractInfo *sdk.ContractInfo, code string)
}

type acceptanceNetwork struct {
	inmemory.Network

	tamperingTransport testGossipAdapter.Tamperer
	description        string
}

func (n *acceptanceNetwork) Start(ctx context.Context, numOfNodesToStart int) {
	n.CreateAndStartNodes(ctx, numOfNodesToStart) // needs to start first so that nodes can register their listeners to it
}

func (n *acceptanceNetwork) WaitForTransactionInNodeState(ctx context.Context, txhash primitives.Sha256, nodeIndex int, ) {
	n.Nodes[nodeIndex].WaitForTransactionInState(ctx, txhash)
}

func (n *acceptanceNetwork) Description() string {
	return n.description
}

func (n *acceptanceNetwork) TransportTamperer() testGossipAdapter.Tamperer {
	return n.tamperingTransport
}

func (n *acceptanceNetwork) BlockPersistence(nodeIndex int) blockStorageAdapter.InMemoryBlockPersistence {
	return n.GetBlockPersistence(nodeIndex)
}

func (n *acceptanceNetwork) GetBenchmarkTokenContract() contracts.BenchmarkTokenClient {
	return contracts.NewContractClient(n)
}

func (n *acceptanceNetwork) DumpState() {
	for i := range n.Nodes {
		n.Logger.Info("state dump", log.Int("node", i), log.String("data", n.GetStatePersistence(i).Dump()))
	}
}

func (n *acceptanceNetwork) MockContract(fakeContractInfo *sdk.ContractInfo, code string) {

	// if needed, provide a fake implementation of this contract to all nodes
	for _, node := range n.Nodes {
		if fakeCompiler, ok := node.GetCompiler().(nativeProcessorAdapter.FakeCompiler); ok {
			fakeCompiler.ProvideFakeContract(fakeContractInfo, code)
		}
	}
}



