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
	"github.com/Loopring/relay-cluster/usermanager"
	"github.com/Loopring/relay-lib/dao"
	"github.com/Loopring/relay-lib/log"
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

type localDataSource struct {
	orderView ordermanager.OrderViewer
}

func (source *localDataSource) GetBalanceAndAllowance(owner, token, spender common.Address) (balance, allowance *big.Int, err error) {
	return accountmanager.GetBalanceAndAllowance(owner, token, spender)
}

func (source *localDataSource) MinerOrders(protocol, tokenS, tokenB common.Address, length int, reservedTime, startBlockNumber, endBlockNumber int64, filterOrderHashLists ...*types.OrderDelayList) []*types.OrderState {
	return source.orderView.MinerOrders(protocol, tokenS, tokenB, length, reservedTime, startBlockNumber, endBlockNumber, filterOrderHashLists...)
}

type motanDataSource struct {
	motanClient *motan.Client
}

func (source *motanDataSource) GetBalanceAndAllowance(owner, token, spender common.Address) (balance, allowance *big.Int, err error) {
	req := &libmotan.AccountBalanceAndAllowanceReq{
		Owner:   owner,
		Token:   token,
		Spender: spender,
	}
	res := &libmotan.AccountBalanceAndAllowanceRes{}
	if err := source.motanClient.Call("getBalanceAndAllowance", []interface{}{req}, res); nil != err || "" != res.Err {
		if "" != res.Err {
			if nil != err {
				err = errors.New(err.Error() + " ; " + res.Err)
			} else {
				err = errors.New(res.Err)
			}
		}
		return nil, nil, err
	} else {
		return res.Balance, res.Allowance, nil
	}
}

func (source *motanDataSource) MinerOrders(protocol, tokenS, tokenB common.Address, length int, reservedTime, startBlockNumber, endBlockNumber int64, filterOrderHashLists ...*types.OrderDelayList) []*types.OrderState {
	orders := []*types.OrderState{}
	req := &libmotan.MinerOrdersReq{
		Delegate:             protocol,
		TokenS:               tokenS,
		TokenB:               tokenB,
		Length:               length,
		ReservedTime:         reservedTime,
		StartBlockNumber:     startBlockNumber,
		EndBlockNumber:       endBlockNumber,
		FilterOrderHashLists: filterOrderHashLists,
	}
	res := &libmotan.MinerOrdersRes{}
	if err := source.motanClient.Call("getMinerOrders", []interface{}{req}, res); nil != err {
		log.Errorf("err:%s", err.Error())
	} else {
		orders = res.List
	}
	return orders
}

type dataSource interface {
	GetBalanceAndAllowance(owner, token, spender common.Address) (balance, allowance *big.Int, err error)
	MinerOrders(protocol, tokenS, tokenB common.Address, length int, reservedTime, startBlockNumber, endBlockNumber int64, filterOrderHashLists ...*types.OrderDelayList) []*types.OrderState
}

var source dataSource

func initializeLocal(options config.DataSource, rdsOptions *dao.MysqlOptions, marketcapProvider marketcap.MarketCapProvider) {
	dSource := &localDataSource{}
	orderRds := orderDao.NewDb(rdsOptions)
	um := &usermanager.UserManagerImpl{}
	dSource.orderView = ordermanager.NewOrderViewer(&options.OrderManager, orderRds, marketcapProvider, um)
	accountmanager.Initialize(&options.AccountManager, []string{})
	source = dSource
}

func initializeMotan(options config.DataSource) {
	dSource := &motanDataSource{}
	dSource.motanClient = libmotan.InitClient(options.MotanClient)
	source = dSource
}

func Initialize(options config.DataSource, rdsOptions *dao.MysqlOptions, marketcapProvider marketcap.MarketCapProvider) {
	if "LOCAL" == strings.ToUpper(options.Type) {
		initializeLocal(options, rdsOptions, marketcapProvider)
	} else {
		initializeMotan(options)
	}
}

func GetBalanceAndAllowance(owner, token, spender common.Address) (balance, allowance *big.Int, err error) {
	return source.GetBalanceAndAllowance(owner, token, spender)
}

func MinerOrders(protocol, tokenS, tokenB common.Address, length int, reservedTime, startBlockNumber, endBlockNumber int64, filterOrderHashLists ...*types.OrderDelayList) []*types.OrderState {
	return source.MinerOrders(protocol, tokenS, tokenB, length, reservedTime, startBlockNumber, endBlockNumber, filterOrderHashLists...)
}
