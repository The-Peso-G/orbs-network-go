package native

import "github.com/orbs-network/orbs-network-go/services/processor/native/sanitizer"

func (s *service) sanitizeDeployedSourceCode(code string) (string, error) {
	if s.config.ProcessorSanitizeDeployedContracts() {
		return s.sanitizer.Process(code)
	} else {
		return code, nil
	}
}

func (s *service) createSanitizer() *sanitizer.Sanitizer {
	sanitizerConfig := &sanitizer.SanitizerConfig{
		ImportWhitelist: map[string]bool{
			`"github.com/orbs-network/orbs-contract-sdk/go/sdk/v1"`:          true,
			`"github.com/orbs-network/orbs-contract-sdk/go/sdk/v1/address"`:  true,
			`"github.com/orbs-network/orbs-contract-sdk/go/sdk/v1/env"`:      true,
			`"github.com/orbs-network/orbs-contract-sdk/go/sdk/v1/ethereum"`: true,
			`"github.com/orbs-network/orbs-contract-sdk/go/sdk/v1/events"`:   true,
			`"github.com/orbs-network/orbs-contract-sdk/go/sdk/v1/safemath"`: true,
			`"github.com/orbs-network/orbs-contract-sdk/go/sdk/v1/service"`:  true,
			`"github.com/orbs-network/orbs-contract-sdk/go/sdk/v1/state"`:    true,
		},
	}
	return sanitizer.NewSanitizer(sanitizerConfig)
}
