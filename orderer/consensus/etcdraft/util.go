/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package etcdraft

import (
	"encoding/pem"
	"fmt"
	"net"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/orderer/common/cluster"
	"github.com/hyperledger/fabric/orderer/common/localconfig"
	"github.com/hyperledger/fabric/orderer/consensus"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/orderer/etcdraft"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
)

// TLSCACertificatesFromSupport extracts TLS CA certificates from the ConsenterSupport
func TLSCACertificatesFromSupport(support consensus.ConsenterSupport) ([][]byte, error) {
	lastBlockSeq := support.Height() - 1
	lastBlock := support.Block(lastBlockSeq)
	if lastBlock == nil {
		return nil, errors.Errorf("unable to retrieve block %d", lastBlockSeq)
	}
	lastConfigBlock, err := LastConfigBlock(lastBlock, support)
	if err != nil {
		return nil, err
	}
	tlsCACerts, err := cluster.TLSCACertsFromConfigBlock(lastConfigBlock)
	if err != nil {
		return nil, err
	}
	return tlsCACerts, nil
}

// LastConfigBlock returns the last config block relative to the given block.
func LastConfigBlock(block *common.Block, support consensus.ConsenterSupport) (*common.Block, error) {
	if block == nil {
		return nil, errors.New("nil block")
	}
	if support == nil {
		return nil, errors.New("nil support")
	}
	if block.Metadata == nil || len(block.Metadata.Metadata) <= int(common.BlockMetadataIndex_LAST_CONFIG) {
		return nil, errors.New("no metadata in block")
	}
	lastConfigBlockNum, err := utils.GetLastConfigIndexFromBlock(block)
	if err != nil {
		return nil, err
	}
	lastConfigBlock := support.Block(lastConfigBlockNum)
	if lastConfigBlock == nil {
		return nil, errors.Errorf("unable to retrieve last config block %d", lastConfigBlockNum)
	}
	return lastConfigBlock, nil
}

func consenterEndpoints(consenters []*etcdraft.Consenter) ([]string, error) {
	var res []string
	for _, c := range consenters {
		if c == nil {
			return nil, errors.New("nil consenter found in metadata")
		}
		res = append(res, net.JoinHostPort(c.Host, fmt.Sprintf("%d", c.Port)))
	}
	return res, nil
}

// newBlockPuller creates a new block puller
func newBlockPuller(support consensus.ConsenterSupport,
	consenters []*etcdraft.Consenter,
	baseDialer *cluster.PredicateDialer,
	clusterConfig localconfig.Cluster) (*cluster.BlockPuller, error) {

	endpoints, err := consenterEndpoints(consenters)
	if err != nil {
		return nil, err
	}

	verifyBlockSequence := func(blocks []*common.Block) error {
		return cluster.VerifyBlocks(blocks, support)
	}

	secureConfig, err := baseDialer.ClientConfig()
	if err != nil {
		return nil, err
	}
	stdDialer := &cluster.StandardDialer{
		Dialer: cluster.NewTLSPinningDialer(secureConfig),
	}

	// Extract the TLS CA certs from the configuration,
	tlsCACerts, err := TLSCACertificatesFromSupport(support)
	if err != nil {
		return nil, err
	}
	// and overwrite them.
	secureConfig.SecOpts.ServerRootCAs = tlsCACerts
	stdDialer.Dialer.SetConfig(secureConfig)

	der, _ := pem.Decode(secureConfig.SecOpts.Certificate)
	if der == nil {
		return nil, errors.Errorf("client certificate isn't in PEM format: %v",
			string(secureConfig.SecOpts.Certificate))
	}

	return &cluster.BlockPuller{
		VerifyBlockSequence: verifyBlockSequence,
		Logger:              flogging.MustGetLogger("orderer/common/cluster/puller"),
		RetryTimeout:        clusterConfig.ReplicationRetryTimeout,
		MaxTotalBufferBytes: clusterConfig.ReplicationBufferSize,
		FetchTimeout:        clusterConfig.ReplicationPullTimeout,
		Endpoints:           endpoints,
		Signer:              support,
		TLSCert:             der.Bytes,
		Channel:             support.ChainID(),
		Dialer:              stdDialer,
	}, nil
}
