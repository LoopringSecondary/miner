/*

  Copyright 2017 Loopring Project Ltd (Loopring Foundation).

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

package dao

import (
	"github.com/Loopring/relay-lib/types"
	"github.com/ethereum/go-ethereum/common"
)

type RdsService interface {
	// create tables
	Prepare()

	// base functions
	Add(item interface{}) error
	Del(item interface{}) error
	First(item interface{}) error
	Last(item interface{}) error
	Save(item interface{}) error
	FindAll(item interface{}) error

	// ring mined table
	FindRingMined(txhash string) (*RingMinedEvent, error)
	RollBackRingMined(from, to int64) error

	// order table
	MarkMinerOrders(filterOrderhashs []string, blockNumber int64) error
	GetOrdersForMiner(protocol, tokenS, tokenB string, length int, filterStatus []types.OrderStatus, reservedTime, startBlockNumber, endBlockNumber int64) ([]*Order, error)
	GetOrdersByHash(orderhashs []string) (map[string]Order, error)
	GetOrderByHash(orderhash common.Hash) (*Order, error)

	// block table
	FindBlockByHash(blockhash common.Hash) (*Block, error)
	FindLatestBlock() (*Block, error)
	SetForkBlock(from, to int64) error
	SaveBlock(latest *Block) error

	// ring table
	UpdateRingSubmitInfoResult(submitResult *types.RingSubmitResultEvent) error
	GetRingForSubmitByHash(ringhash common.Hash) (RingSubmitInfo, error)
	GetRingHashesByTxHash(txHash common.Hash) ([]*RingSubmitInfo, error)
	RingMinedPageQuery(query map[string]interface{}, pageIndex, pageSize int) (res PageResult, err error)
	GetRingminedMethods(lastId int, limit int) ([]RingMinedEvent, error)
	GetFilledOrderByRinghash(ringhash common.Hash) ([]*FilledOrder, error)
}
