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

package node

import (
	"github.com/Loopring/miner/config"
	"github.com/Loopring/miner/dao"
	"github.com/Loopring/miner/datasource"
	"github.com/Loopring/miner/miner"
	"github.com/Loopring/miner/miner/timing_matcher"
	"github.com/Loopring/relay-lib/cache"
	"github.com/Loopring/relay-lib/crypto"
	relayLibEth "github.com/Loopring/relay-lib/eth"
	"github.com/Loopring/relay-lib/eth/gasprice_evaluator"
	"github.com/Loopring/relay-lib/log"
	"github.com/Loopring/relay-lib/marketcap"
	"github.com/Loopring/relay-lib/marketutil"
	"github.com/Loopring/relay-lib/zklock"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"sync"
)

type Node struct {
	globalConfig      *config.GlobalConfig
	rdsService        dao.RdsServiceImpl
	marketCapProvider marketcap.MarketCapProvider
	miner             *miner.Miner

	stop chan struct{}
	lock sync.RWMutex
}

func NewNode(globalConfig *config.GlobalConfig) *Node {
	n := &Node{}
	n.globalConfig = globalConfig
	// register
	n.registerMysql()
	cache.NewCache(n.globalConfig.Redis)
	marketutil.Initialize(&n.globalConfig.MarketUtil)
	n.registerMarketCap()
	n.registerAccessor()
	ks := keystore.NewKeyStore(n.globalConfig.Keystore.Keydir, keystore.StandardScryptN, keystore.StandardScryptP)
	n.registerCrypto(ks)
	n.registerMiner()
	if _, err := zklock.Initialize(globalConfig.ZkLock); nil != err {
		log.Fatalf("err:%s", err.Error())
	}
	gasprice_evaluator.InitGasPriceEvaluator()
	datasource.Initialize(globalConfig.DataSource, globalConfig.Mysql, n.marketCapProvider)
	return n
}

func (n *Node) Start() {
	n.marketCapProvider.Start()
	n.miner.Start()
}

func (n *Node) Wait() {
	n.lock.RLock()

	// TODO(fk): states should be judged

	stop := n.stop
	n.lock.RUnlock()

	<-stop
}

func (n *Node) Stop() {
	n.lock.RLock()
	n.miner.Stop()
	n.lock.RUnlock()
}

func (n *Node) registerCrypto(ks *keystore.KeyStore) {
	c := crypto.NewKSCrypto(true, ks)
	crypto.Initialize(c)
}

func (n *Node) registerMysql() {
	n.rdsService = dao.NewRdsService(n.globalConfig.Mysql)
}

func (n *Node) registerAccessor() {
	if err := relayLibEth.InitializeAccessor(n.globalConfig.Accessor, n.globalConfig.LoopringAccessor); nil != err {
		log.Fatalf("err:%s", err.Error())
	}
}

func (n *Node) registerDataSource() {
	datasource.Initialize(n.globalConfig.DataSource, n.globalConfig.Mysql, n.marketCapProvider)
}

func (n *Node) registerMiner() {
	submitter, err := miner.NewSubmitter(n.globalConfig.Miner, n.rdsService)
	if nil != err {
		log.Fatalf("failed to init submitter, error:%s", err.Error())
	}
	evaluator := miner.NewEvaluator(n.marketCapProvider, n.globalConfig.Miner)
	matcher := timing_matcher.NewTimingMatcher(n.globalConfig.Miner.TimingMatcher, submitter, evaluator, n.rdsService, n.marketCapProvider)
	evaluator.SetMatcher(matcher)
	n.miner = miner.NewMiner(submitter, matcher, evaluator, n.marketCapProvider)
}

func (n *Node) registerMarketCap() {
	n.marketCapProvider = marketcap.NewMarketCapProvider(&n.globalConfig.MarketCap)
}
