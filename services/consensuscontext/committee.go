// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package consensuscontext

import (
	"context"
	"fmt"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/pkg/errors"
	"strings"
)

func (s *service) RequestOrderingCommittee(ctx context.Context, input *services.RequestCommitteeInput) (*services.RequestCommitteeOutput, error) {
	return s.RequestValidationCommittee(ctx, input)
}

func (s *service) RequestValidationCommittee(ctx context.Context, input *services.RequestCommitteeInput) (*services.RequestCommitteeOutput, error) {
	committee, err := s.getOrderedCommittee(ctx, input.CurrentBlockHeight, input.PrevBlockReferenceTime)
	if err != nil {
		return nil, err
	}

	// get data of weights but need to order it. possible future move the weights into the ordering.
	managementCommitteeData, err := s.management.GetCommittee(ctx, &services.GetCommitteeInput{Reference: input.PrevBlockReferenceTime})
	if err != nil {
		return nil, err
	}
	orderedWeights, err := orderCommitteeWeights(committee, managementCommitteeData.Members, managementCommitteeData.Weights)
	if err != nil {
		return nil, err
	}

	s.metrics.committeeSize.Update(int64(len(committee)))
	committeeStringArray := make([]string, len(committee))
	for j, nodeAddress := range committee {
		committeeStringArray[j] = fmt.Sprintf("{\"Address:\": \"%v\", \"Weight\": %d}", nodeAddress, orderedWeights[j])  // %v is because NodeAddress has .String()
	}
	s.metrics.committeeMembers.Update("[" + strings.Join(committeeStringArray, ", ") + "]")
	s.metrics.committeeRefTime.Update(int64(input.PrevBlockReferenceTime))

	res := &services.RequestCommitteeOutput{
		NodeAddresses:            committee,
		NodeRandomSeedPublicKeys: nil,
		Weights:                  orderedWeights,
	}
	return res, nil
}

func orderCommitteeWeights(orderedCommittee []primitives.NodeAddress, committeeMembers []primitives.NodeAddress, committeeWeights []primitives.Weight) ([]primitives.Weight, error) {
	if len(orderedCommittee) != len(committeeMembers) || len(orderedCommittee) != len(committeeWeights) {
		return nil, errors.Errorf("order weights failed sizes don't match %v, %v, %v", orderedCommittee, committeeMembers, committeeWeights)
	}

	tempMap := make(map[string]primitives.Weight, len(orderedCommittee))
	orderedWeights := make([]primitives.Weight, len(orderedCommittee))

	for i := range committeeMembers {
		tempMap[committeeMembers[i].KeyForMap()] = committeeWeights[i]
	}

	for i := range orderedCommittee {
		if weight, ok := tempMap[orderedCommittee[i].KeyForMap()]; !ok {
			return nil, errors.Errorf("order weights failed committee and ordered don't have same addresses: %v, %v", orderedCommittee, committeeMembers)
		} else {
			orderedWeights[i] = weight
		}
	}

	return orderedWeights, nil
}


func (s *service) RequestBlockProofOrderingCommittee(ctx context.Context, input *services.RequestBlockProofCommitteeInput) (*services.RequestBlockProofCommitteeOutput, error) {
	return s.RequestBlockProofValidationCommittee(ctx, input)
}

func (s *service) RequestBlockProofValidationCommittee(ctx context.Context, input *services.RequestBlockProofCommitteeInput) (*services.RequestBlockProofCommitteeOutput, error) {
	out, err := s.management.GetCommittee(ctx, &services.GetCommitteeInput{Reference:input.PrevBlockReferenceTime})
	if err != nil {
		return nil, err
	}
	res := &services.RequestBlockProofCommitteeOutput{
		NodeAddresses: out.Members,
		Weights:       out.Weights,
	}
	return res, nil
}
