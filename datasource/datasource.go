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

package datasource

import (
	"errors"
	"github.com/Loopring/miner/config"
	"github.com/Loopring/motan-go"
	"github.com/Loopring/relay-cluster/accountmanager"
	orderDao "github.com/Loopring/relay-cluster/dao"
	"github.com/Loopring/relay-cluster/ordermanager"
	"github.com/Loopring/relay-lib/dao"
	"github.com/Loopring/relay-lib/marketcap"
	libmotan "github.com/Loopring/relay-lib/motan"
	"github.com/Loopring/relay-lib/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"strings"
)

type Mode int

const (
	LOCAL Mode = 1
	MOTAN Mode = 2
)

type dataSource struct {
	mode         Mode
	orderManager ordermanager.OrderManager

	motanClient *motan.Client
}

var source dataSource

func Initialize(options config.DataSource, rdsOptions *dao.MysqlOptions, marketcapProvider marketcap.MarketCapProvider) {
	source = dataSource{}
	if "LOCAL" == strings.ToUpper(options.Type) {
		source.mode = LOCAL
		orderRds := orderDao.NewDb(rdsOptions)
		source.orderManager = ordermanager.NewOrderManager(&options.OrderManager, orderRds, marketcapProvider)
		accountmanager.Initialize(&options.AccountManager, []string{})
	} else {
		source.mode = MOTAN
		libmotan.InitClient(options.MotanClient)
	}
}

func GetBalanceAndAllowance(owner, token, spender common.Address) (balance, allowance *big.Int, err error) {
	switch source.mode {
	case LOCAL:
		return nil, nil, errors.New("")
	case MOTAN:
		return nil, nil, errors.New("")
	}
	return nil, nil, errors.New("error")
}

func MinerOrders(protocol, tokenS, tokenB common.Address, length int, reservedTime, startBlockNumber, endBlockNumber int64, filterOrderHashLists ...*types.OrderDelayList) []*types.OrderState {
	switch source.mode {
	case LOCAL:
		return []*types.OrderState{}
	case MOTAN:
		return []*types.OrderState{}
	}
	return []*types.OrderState{}
}
