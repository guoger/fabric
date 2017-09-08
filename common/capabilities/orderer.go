/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package capabilities

import (
	cb "github.com/hyperledger/fabric/protos/common"
)

const (
	ordererTypeName    = "Orderer"
	OrdererV11BugFixes = "V1.1_Orderer_BugFixes"
)

// OrdererProvider provides capabilities information for orderer level config
type OrdererProvider struct {
	*Registry
	v11BugFixes bool
}

// NewOrderer creates a channel capabilities provider
func NewOrderer(capabilities map[string]*cb.Capability) *OrdererProvider {
	cp := &OrdererProvider{}
	cp.Registry = newRegistry(cp, capabilities)
	_, cp.v11BugFixes = capabilities[OrdererV11BugFixes]
	return cp
}

// Type returns a descriptive string for logging purposes
func (cp *OrdererProvider) Type() string {
	return ordererTypeName
}

// HasCapability returns true if the capability is supported by this binary
func (cp *OrdererProvider) HasCapability(capability string) bool {
	switch capability {
	// Add new capability names here
	default:
		return false
	}
}

// SetChannelModPolicyDuringCreate specifies whether the v1.0 undesirable behavior of setting the /Channel
// group's mod_policy to "" should be fixed or not.
func (cp *OrdererProvider) SetChannelModPolicyDuringCreate() bool {
	return cp.v11BugFixes
}
