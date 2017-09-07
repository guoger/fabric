/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package capabilities

import (
	cb "github.com/hyperledger/fabric/protos/common"

	"github.com/op/go-logging"
	"github.com/pkg/errors"
)

var logger = logging.MustGetLogger("common/capabilities")

type Provider interface {
	// HasCapability should report whether the binary supports this capability
	HasCapability(capability string) bool

	// Type is used to make error messages more ledgible
	Type() string
}

type Registry struct {
	provider     Provider
	capabilities map[string]*cb.Capability
}

func newRegistry(p Provider, capabilities map[string]*cb.Capability) *Registry {
	return &Registry{
		provider:     p,
		capabilities: capabilities,
	}
}

// Supported checks that all of the required capabilities are supported by this binary
func (r *Registry) Supported() error {
	for capabilityName, capability := range r.capabilities {
		if r.provider.HasCapability(capabilityName) {
			continue
		}

		if capability.Required {
			return errors.Errorf("%s capability %s is required but not supported", r.provider.Type(), capabilityName)
		} else {
			logger.Debugf("Found unknown %s capability %s but it is not required", r.provider.Type(), capabilityName)
		}
	}
	return nil
}
