package helpers

import (
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/integration/nwo"
	"github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protoutil"
)

// SetACLPolicy sets the ACL policy for a running network. It resets all
// previously defined ACL policies, generates the config update, signs the
// configuration with Org2's signer, and then submits the config update using
// Org1.
func SetACLPolicy(network *nwo.Network, channel, policyName, policy string, ordererName string) {
	orderer := network.Orderer(ordererName)
	submitter := network.Peer("Org1", "peer0")
	signer := network.Peer("Org2", "peer0")

	config := nwo.GetConfig(network, submitter, orderer, channel)
	updatedConfig := proto.Clone(config).(*common.Config)

	// set the policy
	updatedConfig.ChannelGroup.Groups["Application"].Values["ACLs"] = &common.ConfigValue{
		ModPolicy: "Admins",
		Value: protoutil.MarshalOrPanic(&pb.ACLs{
			Acls: map[string]*pb.APIResource{
				policyName: {PolicyRef: policy},
			},
		}),
	}

	nwo.UpdateConfig(network, orderer, channel, config, updatedConfig, true, submitter, signer)
}
