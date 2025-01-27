/*
 * Copyright 2021 ConsenSys Software Inc
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
package net.consensys.gpact.common;

/** Contains information about how to use a blockchain. */
public class BlockchainConfig {
  // Blockchain identifier.
  public BlockchainId bcId;
  // URI of the blockchain node to be used for connecting to the blockchain.
  public String uri;
  // Strategy to be used for setting gas prices.
  public DynamicGasProvider.Strategy gasPriceStrategy;
  // How often to poll the blockchain for a response. Typically set to the block period.
  public int period;

  /**
   * @param bcId Blockchain identifier.
   * @param uri URI of the blockchain node to be used for connecting to the blockchain.
   * @param gasPriceStrategy Strategy to be used for setting gas prices.
   * @param period How often to poll the blockchain for a response. Typically set to the block
   *     period.
   */
  public BlockchainConfig(String bcId, String uri, String gasPriceStrategy, String period) {
    this.bcId = new BlockchainId(bcId);
    this.uri = uri;
    this.gasPriceStrategy = DynamicGasProvider.Strategy.valueOf(gasPriceStrategy);
    this.period = Integer.parseInt(period);
  }
}
