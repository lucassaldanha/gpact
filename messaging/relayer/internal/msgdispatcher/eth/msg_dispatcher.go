/*
Package eth - Message dispatcher for Ethereum Clients.
*/
package eth

import (
	"context"
	"math/big"

	log "github.com/consensys/gpact/messaging/relayer/internal/logging"

	//gethbind "github.com/ethereum/go-ethereum/accounts/abi/bind"
	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	//gethcrypto "github.com/ethereum/go-ethereum/crypto"
	gethclient "github.com/ethereum/go-ethereum/ethclient"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
)

/*
 * Copyright 2021 ConsenSys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

// MsgDispatcher holds the context for submitting transactions
// to an Ethereum Client.
type MsgDispatcher struct {
	endpoint                  string     // URL without protocol specifier of Ethereum client.
	http                      bool       // HTTP or WS
	apiAuthKey                string     // Authentication key to access the Ethereum API.
	keyManager                KeyManager // Holds all keys for this dispatcher.
	crosschainControlContract *gethcommon.Address
	chainID                   *big.Int

	connection *gethclient.Client
}

// MsgDispatcherConfig holds variables needed to configure the message
// dispatcher component.
type MsgDispatcherConfig struct {
	endpoint   string // URL without protocol specifier of Ethereum client.
	http       bool   // HTTP or WS
	apiAuthKey string // Authentication key to access the Ethereum API.
	keyManager KeyManager

	crosschainControlContract *gethcommon.Address
}

// NewMsgDispatcher creates a new message dispatcher instance.
func NewMsgDispatcher(c *MsgDispatcherConfig) (*MsgDispatcher, error) {
	var m = MsgDispatcher{}
	m.endpoint = c.endpoint
	m.http = c.http
	m.apiAuthKey = c.apiAuthKey

	m.keyManager = c.keyManager

	log.Info("Message Dispatcher (Eth) for %v", c.endpoint)

	return &m, nil
}

// Connect attempts to use the configuration to connect to the end point.
func (m *MsgDispatcher) Connect() error {
	log.Info("Connecting to: %v", m.endpoint)
	var rpcClient *gethrpc.Client
	var err error
	// Start http or ws client
	if m.http {
		rpcClient, err = gethrpc.DialHTTP(m.endpoint)
	} else {
		rpcClient, err = gethrpc.DialContext(context.Background(), m.endpoint)
	}
	if err != nil {
		return err
	}
	m.connection = gethclient.NewClient(rpcClient)

	return nil
}

func (m *MsgDispatcher) SubmitTransaction(txData []byte) error {
	// TODO support Legacy transactions, for consortium blockchains and other blockchains that don't support EIP 1559.
	return m.submitEIP1559Transaction(txData)
}

// submitEIP1559Transaction submits a transaction to an EIP1559 blockchain.
func (m *MsgDispatcher) submitEIP1559Transaction(txData []byte) error {

	feeCap := big.NewInt(10000000)
	tip := big.NewInt(10000000)

	gas := uint64(100000)
	value := big.NewInt(0)
	//txData := make([]byte, 0)

	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   m.chainID,
		Nonce:     m.keyManager.nonce,
		GasFeeCap: feeCap,
		GasTipCap: tip,
		Gas:       gas,
		To:        m.crosschainControlContract,
		Value:     value,
		Data:      txData,
	})

	return m.connection.SendTransaction(context.Background(), tx)

}