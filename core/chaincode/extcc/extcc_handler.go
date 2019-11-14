/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package extcc

import (
	"context"
	"fmt"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/comm"
	"github.com/hyperledger/fabric/core/container/ccintf"
	"github.com/pkg/errors"

	pb "github.com/hyperledger/fabric-protos-go/peer"

	"google.golang.org/grpc"
)

var extccLogger = flogging.MustGetLogger("extcc")

// StreamHandler handles the `Chaincode` gRPC service with peer as client
type StreamHandler interface {
	HandleChaincodeStream(stream ccintf.ChaincodeStream) error
}

type ExternalChaincodeRuntime struct {
}

// createConnection - standard grpc client creating using ClientConfig info (surprised there isn't
// a helper method for this)
func (i *ExternalChaincodeRuntime) createConnection(ccid string, ccinfo *ccintf.ChaincodeServerInfo) (*grpc.ClientConn, error) {
	if ccinfo == nil {
		return nil, errors.New("cannot start connector without connection properties")
	}

	grpcClient, err := comm.NewGRPCClient(ccinfo.ClientConfig)
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("error creating grpc client to %s", ccid))
	}

	conn, err := grpcClient.NewConnection(ccinfo.Address)
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("error creating grpc connection to %s", ccinfo.Address))
	}

	extccLogger.Debugf("created external chaincode connection: %s", ccid)

	return conn, nil
}

func (i *ExternalChaincodeRuntime) Run(ccid string, ccinfo *ccintf.ChaincodeServerInfo, sHandler StreamHandler) error {
	extccLogger.Debugf("starting external chaincode connection: %s", ccid)
	conn, err := i.createConnection(ccid, ccinfo)
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("error cannot create connection for %s", ccid))
	}

	//remove connection after processing (or error)
	defer conn.Close()

	//create the client and start streaming
	client := pb.NewChaincodeClient(conn)

	stream, err := client.Connect(context.Background())
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("error creating grpc client connection to %s", ccid))
	}

	//peer as client has to initiate the stream. Rest of the process is unchanged
	sHandler.HandleChaincodeStream(stream)

	extccLogger.Debugf("external chaincode %s client exited", ccid)

	return nil
}
