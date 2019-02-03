package main

import (
	"github.com/orbs-network/orbs-network-go/instrumentation/log"
	"github.com/orbs-network/orbs-network-go/services/blockstorage/adapter/test"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"os"
	"time"
)

func main() {
	config := &localConfig{dir: os.Args[1], virtualChainId: 42}
	_, release, err := test.NewFilesystemAdapterDriver(log.GetLogger(), config)
	if err != nil {
		os.Exit(1)
	}

	defer release()
	time.Sleep(1 * time.Second)
}

type localConfig struct {
	dir            string
	virtualChainId primitives.VirtualChainId
}

func (l *localConfig) VirtualChainId() primitives.VirtualChainId {
	return l.virtualChainId
}

func (l *localConfig) BlockStorageFileSystemDataDir() string {
	return l.dir
}

func (l *localConfig) BlockStorageFileSystemMaxBlockSizeInBytes() uint32 {
	return 1000000000
}
