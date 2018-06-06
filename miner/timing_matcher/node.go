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
	"github.com/Loopring/relay-lib/marketutil"
	"github.com/Loopring/relay-lib/types"
	"github.com/Loopring/relay-lib/zklock"
	"github.com/ethereum/go-ethereum/common"
	"strings"
	"sync"
)

const (
	Task_Balancer_Name = "task_balancer_timingmatcher"
)

type NodeInterface interface {
	assignMarkets()
	init() error
	start()
	stop()
}

func (market *Market) generateTask() (zklock.Task, error) {
	task := zklock.Task{Weight: 10, Status: zklock.Init, Owner: "", Timestamp: 0}
	protocolAddrHex := strings.ToLower(market.protocolImpl.ContractAddress.Hex())
	if tokenA, err := marketutil.GetSymbolWithAddress(market.TokenA); nil != err {
		return task, err
	} else if tokenB, err1 := marketutil.GetSymbolWithAddress(market.TokenB); nil != err1 {
		return task, err1
	} else {
		tokenAHex := strings.ToLower(market.TokenA.Hex())
		tokenBHex := strings.ToLower(market.TokenB.Hex())
		if strings.Compare(tokenAHex, tokenBHex) >= 0 {
			task.Payload = protocolAddrHex + "_" + tokenAHex + "_" + tokenBHex
			task.Path = tokenA + "_" + tokenB
		} else {
			task.Payload = protocolAddrHex + "_" + tokenBHex + "_" + tokenAHex
			task.Path = tokenB + "_" + tokenA
		}
		return task, nil
	}
}

func (market *Market) fromTask(task zklock.Task, matcher *TimingMatcher) error {
	tokens := strings.Split(task.Payload, "_")
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

func (node *SingleNode) init() error {
	node.matcher.runingMarkets = Markets{}
	return nil
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
	assignedMarkets Markets
	zkBalancer   *zklock.ZkBalancer
	matcher      *TimingMatcher
	mtx sync.Mutex
}

func (node *ClusterNode) assignMarkets() {
	node.mtx.Lock()
	defer node.mtx.Unlock()

	//release market
	releasedTasks := []zklock.Task{}
	for _, market := range node.assignedMarkets {
		if !node.toRunMarkets.contain(market.TokenA, market.TokenB) {
			task, err := market.generateTask()
			if nil != err {
				log.Errorf("err:%s", err.Error())
				continue
			}
			log.Debugf("releasedtask path:%s, payload:%s", task.Path, task.Payload)
			releasedTasks = append(releasedTasks, task)
		}
	}
	if len(releasedTasks) > 0 {
		if err := node.zkBalancer.Released(releasedTasks); nil != err {
			log.Errorf("err:%s", err.Error())
		}
	}
	node.matcher.runingMarkets = Markets{}
	node.assignedMarkets = Markets{}
	for _, market := range node.toRunMarkets {
		log.Debugf("runningtask tokenA:%s, tokenB:%s", market.TokenA.Hex(), market.TokenB.Hex())
		node.matcher.runingMarkets = append(node.matcher.runingMarkets, market)
		node.assignedMarkets = append(node.assignedMarkets, market)
	}
}

func (node *ClusterNode) init() error {
	node.zkBalancer = &zklock.ZkBalancer{}
	node.mtx = sync.Mutex{}
	node.toRunMarkets = Markets{}
	node.assignedMarkets = Markets{}
	tasks := []zklock.Task{}

	markets := node.matcher.localAllMarkets()

	for _, market := range markets {
		task, err := market.generateTask()
		if nil != err {
			log.Errorf("err:%s", err.Error())
		}
		tasks = append(tasks, task)
	}
	if err := node.zkBalancer.Init(Task_Balancer_Name, tasks); nil != err {
		return err
	}
	return nil
}

func (node *ClusterNode) start() {
	node.zkBalancer.OnAssign(node.handleOnAssign)
	node.zkBalancer.Start()
}

func (node *ClusterNode) stop() {
	node.mtx.Lock()
	defer node.mtx.Unlock()

	node.toRunMarkets = Markets{}
	node.zkBalancer.Stop()
}

func (node *ClusterNode) handleOnAssign(tasks []zklock.Task) error {
	node.mtx.Lock()
	defer node.mtx.Unlock()

	node.toRunMarkets = Markets{}
	for _, task := range tasks {
		log.Debugf("handleOnAssign, assignedtask path:%s, payload:%s", task.Path, task.Payload)
		market := &Market{}
		if err := market.fromTask(task, node.matcher); nil != err {
			log.Errorf("err:%s", err.Error())
			return err
		} else {
			if !node.assignedMarkets.contain(market.TokenA, market.TokenB) {
				node.assignedMarkets = append(node.assignedMarkets, market)
			}
			node.toRunMarkets = append(node.toRunMarkets, market)
		}
	}
	return nil
}
