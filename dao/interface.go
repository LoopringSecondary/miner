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

import ()
import (
	"github.com/Loopring/relay-lib/types"
	"github.com/ethereum/go-ethereum/common"
)

type RdsService interface {
	UpdateRingSubmitInfoResult(submitResult *types.RingSubmitResultEvent) error
	GetRingForSubmitByHash(ringhash common.Hash) (RingSubmitInfo, error)
	GetRingHashesByTxHash(txHash common.Hash) ([]*RingSubmitInfo, error)
	//GetRingminedMethods(lastId int, limit int) ([]types.RingMinedEvent, error)
	GetFilledOrderByRinghash(ringhash common.Hash) ([]*FilledOrder, error)
	UpdateRingSubmitInfoErrById(id int, err error) error
	GetPendingTx(createTime int64) (ringForSubmits []RingSubmitInfo, err error)
	GetSubmitterNonce(submitter string) (uint64,error)
	HasReSubmited(createTime int64, miner string, txNonce uint64) (bool, error)
}
