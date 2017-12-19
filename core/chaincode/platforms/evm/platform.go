/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package evm

import (
	"archive/tar"
	pb "github.com/hyperledger/fabric/protos/peer"
	"io/ioutil"
	"github.com/pkg/errors"
	"encoding/hex"
)

// Platform for chaincodes written in Go
type Platform struct {
}

func (evmPlatform *Platform) ValidateSpec(spec *pb.ChaincodeSpec) error {
	return nil
}

func (evmPlatform *Platform) ValidateDeploymentSpec(spec *pb.ChaincodeDeploymentSpec) error {
	return nil
}

func (evmPlatform *Platform) GetDeploymentPayload(spec *pb.ChaincodeSpec) ([]byte, error) {
	raw, err := ioutil.ReadFile(spec.ChaincodeId.Path)
	if err != nil {
		return nil, errors.Errorf("enable to read file %s, because = %s", spec.ChaincodeId.Path, err)
	}

	codeByte, err := hex.DecodeString(string(raw))
	if err != nil {
		return nil, errors.Errorf("failed to decode bytecode = %s", err)
	}

	return codeByte, nil
}

func (evmPlatform *Platform) GenerateDockerfile(spec *pb.ChaincodeDeploymentSpec) (string, error) {
	return "", nil
}

func (evmPlatform *Platform) GenerateDockerBuild(spec *pb.ChaincodeDeploymentSpec, tw *tar.Writer) error {
	return nil
}

