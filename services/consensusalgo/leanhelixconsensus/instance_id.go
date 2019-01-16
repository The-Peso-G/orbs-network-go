package leanhelixconsensus

import (
	"encoding/binary"
	lhprimitives "github.com/orbs-network/lean-helix-go/spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
)

func CalcInstanceId(networkType protocol.SignerNetworkType, id primitives.VirtualChainId) lhprimitives.InstanceId {
	networkBytes := make([]byte, 2)
	vchainBytes := make([]byte, 4)
	res := make([]byte, 8)
	binary.LittleEndian.PutUint16(networkBytes, uint16(networkType))
	binary.LittleEndian.PutUint32(vchainBytes, uint32(id))
	res[0] = 0
	res[1] = 0
	res[2] = networkBytes[0]
	res[3] = networkBytes[1]
	res[4] = vchainBytes[0]
	res[5] = vchainBytes[1]
	res[6] = vchainBytes[2]
	res[7] = vchainBytes[3]

	return lhprimitives.InstanceId(binary.LittleEndian.Uint64(res))
}
