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
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	cutil "github.com/hyperledger/fabric/core/container/util"
	pb "github.com/hyperledger/fabric/protos/peer"
)

// Platform for chaincodes in EVM bytecode.
type Platform struct {
}

func fileExists(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err == nil && stat.Mode().IsRegular() {
		return true, nil
	}
	return false, err
}

func (evmPlatform *Platform) ValidateSpec(spec *pb.ChaincodeSpec) error {
	path, err := filepath.Abs(spec.ChaincodeId.Path)
	if err != nil {
		return err
	}

	exists, err := fileExists(path)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("file %s does not exist", path)
	}

	return nil
}

func (evmPlatform *Platform) ValidateDeploymentSpec(cds *pb.ChaincodeDeploymentSpec) error {
	return nil
}

func (evmPlatform *Platform) GetDeploymentPayload(spec *pb.ChaincodeSpec) ([]byte, error) {
	raw, err := ioutil.ReadFile(spec.ChaincodeId.Path)
	if err != nil {
		return nil, fmt.Errorf("enable to read file %s, because = %s", spec.ChaincodeId.Path, err)
	}

	// make sure bytecode can be decoded into hex
	if _, err := hex.DecodeString(string(raw)); err != nil {
		return nil, fmt.Errorf("failed to decode bytecode = %s", err)
	}

	payload := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(payload)
	tw := tar.NewWriter(gw)

	if err := cutil.WriteBytesToPackage(spec.ChaincodeId.Name, raw, tw); err != nil {
		return nil, fmt.Errorf("failed to write bytes to tar: %s", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed closing tar writer: %s", err)
	}

	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("failed closing gzip writer: %s", err)
	}

	return payload.Bytes(), nil
}

// EVM bytecode will be executed in an evm system chaincode, therefore not necessary to implement these
func (evmPlatform *Platform) GenerateDockerfile(spec *pb.ChaincodeDeploymentSpec) (string, error) {
	return "", nil
}

func (evmPlatform *Platform) GenerateDockerBuild(spec *pb.ChaincodeDeploymentSpec, tw *tar.Writer) error {
	return nil
}
