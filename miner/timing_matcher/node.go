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

package timing_matcher

import (
	"errors"
	"github.com/Loopring/relay-lib/eth/loopringaccessor"
	"github.com/Loopring/relay-lib/log"
	"github.com/Loopring/relay-lib/types"
	"github.com/Loopring/relay-lib/zklock"
	"github.com/ethereum/go-ethereum/common"
	"strings"
)

type Mode int

const (
	Cluster Mode = 1
	Single  Mode = 2
)

type NodeInterface interface {
	assignMarkets()
	init()
	start()
	stop()
}

func (market *Market) generateTask() zklock.Task {
	task := zklock.Task{}
	protocolAddrHex := strings.ToLower(market.protocolImpl.ContractAddress.Hex())
	tokenAHex := strings.ToLower(market.TokenA.Hex())
	tokenBHex := strings.ToLower(market.TokenB.Hex())
	if strings.Compare(tokenAHex, tokenBHex) >= 0 {
		task.Path = protocolAddrHex + "_" + tokenAHex + "_" + tokenBHex
	} else {
		task.Path = protocolAddrHex + "_" + tokenBHex + "_" + tokenAHex
	}

	return task
}

func (market *Market) fromTask(task zklock.Task, matcher *TimingMatcher) error {
	tokens := strings.Split(task.Path, "_")
	if len(tokens) > 2 {
		protocolAddr := common.HexToAddress(tokens[0])
		if protocolImpl, exists := loopringaccessor.ProtocolAddresses()[protocolAddr]; exists {
			market.protocolImpl = protocolImpl
			market.TokenA = common.HexToAddress(tokens[1])
			market.TokenB = common.HexToAddress(tokens[2])
			market.AtoBOrders = &OrdersState{Orders: make(map[common.Hash]*types.OrderState), OrderHashesExcludeNextRound: []common.Hash{}}
			market.BtoAOrders = &OrdersState{Orders: make(map[common.Hash]*types.OrderState), OrderHashesExcludeNextRound: []common.Hash{}}
			market.matcher = matcher
		} else {
			return errors.New("not exist protocol ")
		}
	} else {
		return errors.New("wrong format of task.Path")
	}
	return nil
}

type Markets []*Market

func (markets Markets) distinct() Markets {
	marketsTmp := Markets{}

	for _, market := range markets {
		inited := false
		if marketsTmp.contain(market.TokenA, market.TokenB) {
			inited = true
			break
		}
		if !inited {
			marketsTmp = append(marketsTmp, market)
		}
	}
	markets = marketsTmp
	return marketsTmp
}

//todo:protocolAddr
func (markets Markets) contain(tokenA, tokenB common.Address) bool {
	for _, market := range markets {
		if (tokenA == market.TokenA && tokenB == market.TokenB) ||
			(tokenA == market.TokenB && tokenB == market.TokenA) {
			return true
		}
	}
	return false
}

type SingleNode struct {
	matcher *TimingMatcher
}

func (node *SingleNode) init() {
	node.matcher.runingMarkets = Markets{}
}

func (node *SingleNode) start() {
	node.matcher.runingMarkets = node.matcher.localAllMarkets()
}

func (node *SingleNode) stop() {
	node.matcher.runingMarkets = Markets{}
}

func (node *SingleNode) assignMarkets() {
	return
}

type ClusterNode struct {
	toRunMarkets Markets
	zkBalancer   *zklock.ZkBalancer
	matcher      *TimingMatcher
}

func (node *ClusterNode) assignMarkets() {
	//release market
	for _, market := range node.matcher.runingMarkets {
		if !node.toRunMarkets.contain(market.TokenA, market.TokenB) {
			if err := node.zkBalancer.Released(market.generateTask()); nil != err {
				log.Errorf("err:%s", err.Error())
			}
		}
	}
	node.matcher.runingMarkets = Markets{}
	for _, market := range node.toRunMarkets {
		node.matcher.runingMarkets = append(node.matcher.runingMarkets, market)
	}
}

func (node *ClusterNode) init() {
	node.zkBalancer = &zklock.ZkBalancer{}
	node.toRunMarkets = Markets{}
	tasks := []zklock.Task{}

	markets := node.matcher.localAllMarkets()

	for _, market := range markets {
		tasks = append(tasks, market.generateTask())
	}
	node.zkBalancer.Init(tasks)
}

func (node *ClusterNode) start() {
	node.zkBalancer.OnAssign(node.handleOnAssign)
	node.zkBalancer.Start()
}

func (node *ClusterNode) stop() {
	node.toRunMarkets = Markets{}
	node.zkBalancer.Stop()
}

func (node *ClusterNode) handleOnAssign(tasks []zklock.Task) error {
	node.toRunMarkets = Markets{}
	for _, task := range tasks {
		market := &Market{}
		if err := market.fromTask(task, node.matcher); nil != err {
			log.Errorf("err:%s", err.Error())
			return err
		} else {
			node.toRunMarkets = append(node.toRunMarkets, market)
		}
	}
	return nil
}
