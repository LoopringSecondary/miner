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
	"github.com/Loopring/miner/miner"
	"github.com/ethereum/go-ethereum/common"
	"math/big"

	"github.com/Loopring/miner/config"
	"github.com/Loopring/miner/dao"
	"github.com/Loopring/miner/datasource"
	"github.com/Loopring/relay-lib/eth/loopringaccessor"
	"github.com/Loopring/relay-lib/kafka"
	"github.com/Loopring/relay-lib/log"
	"github.com/Loopring/relay-lib/marketcap"
	marketUtilLib "github.com/Loopring/relay-lib/marketutil"
	"github.com/Loopring/relay-lib/types"
	"strings"
)

/**
定时从ordermanager中拉取n条order数据进行匹配成环，如果成环则通过调用evaluator进行费用估计，然后提交到submitter进行提交到以太坊
*/

type TimingMatcher struct {
	//rounds          *RoundStates
	runingMarkets     Markets
	node              NodeInterface
	submitter         *miner.RingSubmitter
	evaluator         *miner.Evaluator
	lastRoundNumber   *big.Int
	duration          *big.Int
	lagBlocks         int64
	roundOrderCount   int
	reservedTime      int64
	maxFailedCount    int64
	marketCapProvider marketcap.MarketCapProvider

	maxCacheRoundsLength int
	delayedNumber        int64
	isOrdersReady        bool
	db                   dao.RdsServiceImpl

	messageProducer           *kafka.MessageProducer
	blockEndConsumer          *kafka.ConsumerRegister
	relayProcessedBlockNumber *big.Int

	balanceAndAllowances map[common.Address]*big.Rat

	stopFuncs []func()
}

func NewTimingMatcher(matcherOptions *config.TimingMatcher,
	submitter *miner.RingSubmitter,
	evaluator *miner.Evaluator,
	rds dao.RdsServiceImpl,
	marketcapProvider marketcap.MarketCapProvider,
	kafkaOptions kafka.KafkaOptions,
) *TimingMatcher {
	cacheTtl = matcherOptions.MaxCacheTime
	matcher := &TimingMatcher{}

	matcher.blockEndConsumer = &kafka.ConsumerRegister{}
	matcher.blockEndConsumer.Initialize(kafkaOptions.Brokers)

	matcher.messageProducer = &kafka.MessageProducer{}
	matcher.messageProducer.Initialize(kafkaOptions.Brokers)
	matcher.relayProcessedBlockNumber = big.NewInt(int64(0))
	matcher.submitter = submitter
	matcher.evaluator = evaluator
	matcher.marketCapProvider = marketcapProvider
	matcher.roundOrderCount = matcherOptions.RoundOrdersCount
	//matcher.rounds = NewRoundStates(matcherOptions.MaxCacheRoundsLength)
	matcher.isOrdersReady = false
	matcher.db = rds
	matcher.lagBlocks = matcherOptions.LagForCleanSubmitCacheBlocks
	if matcherOptions.ReservedSubmitTime > 0 {
		matcher.reservedTime = matcherOptions.ReservedSubmitTime
	} else {
		matcherOptions.ReservedSubmitTime = 45
	}
	if matcherOptions.MaxSumitFailedCount > 0 {
		matcher.maxFailedCount = matcherOptions.MaxSumitFailedCount
	} else {
		matcher.maxFailedCount = 3
	}

	matcher.runingMarkets = []*Market{}
	matcher.duration = big.NewInt(matcherOptions.Duration)
	matcher.delayedNumber = matcherOptions.DelayedNumber

	matcher.lastRoundNumber = big.NewInt(0)
	matcher.stopFuncs = []func(){}

	if "CLUSTER" == strings.ToUpper(matcherOptions.Mode) {
		matcher.node = &ClusterNode{matcher: matcher}
	} else {
		matcher.node = &SingleNode{matcher: matcher}
	}
	if err := matcher.node.init(); nil != err {
		log.Fatalf("err:%s", err.Error())
		return nil
	}
	matcher.stopFuncs = append(matcher.stopFuncs, matcher.node.stop)

	return matcher
}

func (matcher *TimingMatcher) cleanMissedCache() {
	//如果程序不正确的停止，清除错误的缓存数据
	if ringhashes, err := CachedRinghashes(); nil == err {
		for _, ringhash := range ringhashes {

			if submitInfo, err1 := matcher.db.GetRingForSubmitByHash(ringhash); nil == err1 {
				if submitInfo.ID <= 0 {
					RemoveMinedRingAndReturnOrderhashes(ringhash)
					//cache.Del(RingHashPrefix + strings.ToLower(ringhash.Hex()))
				}
			} else {
				if strings.Contains(err1.Error(), "record not found") {
					RemoveMinedRingAndReturnOrderhashes(ringhash)
				}
				log.Errorf("err:%s", err1.Error())
			}
		}
	} else {
		log.Errorf("err:%s", err.Error())
	}
}

func (matcher *TimingMatcher) Start() {
	matcher.listenSubmitEvent()
	matcher.listenOrderReady()
	matcher.cleanMissedCache()
	matcher.node.start()
	matcher.listenTimingRound()
}

func (matcher *TimingMatcher) Stop() {
	for _, stop := range matcher.stopFuncs {
		stop()
	}
}

func (matcher *TimingMatcher) GetAccountAvailableAmount(address, tokenAddress, spender common.Address) (*big.Rat, error) {
	//log.Debugf("address: %s , token: %s , spender: %s", address.Hex(), tokenAddress.Hex(), spender.Hex())
	availableAmount := big.NewRat(0,1)
	if availableAmount1,ok := matcher.balanceAndAllowances[address]; ok {
		availableAmount.Set(availableAmount1)
	} else {
		if balance, allowance, err := datasource.GetBalanceAndAllowance(address, tokenAddress, spender); nil != err {
			return nil, err
		} else {
			availableAmount.SetInt(balance)
			allowanceAmount := new(big.Rat).SetInt(allowance)
			if availableAmount.Cmp(allowanceAmount) > 0 {
				availableAmount = allowanceAmount
			}
			matcher.balanceAndAllowances[address] = availableAmount
		}
	}

	matchedAmountS, _ := FilledAmountS(address, tokenAddress)

	availableAmount.Sub(availableAmount, matchedAmountS)
	log.Debugf("owner:%s, token:%s, spender:%s, availableAmount:%s, matchedAmountS:%s", address.Hex(), tokenAddress.Hex(), spender.Hex(), availableAmount.FloatString(2), matchedAmountS.FloatString(2))

	return availableAmount, nil
	//if balance, allowance, err := datasource.GetBalanceAndAllowance(address, tokenAddress, spender); nil != err {
	//	return nil, err
	//} else {
	//	availableAmount := new(big.Rat).SetInt(balance)
	//	allowanceAmount := new(big.Rat).SetInt(allowance)
	//	if availableAmount.Cmp(allowanceAmount) > 0 {
	//		availableAmount = allowanceAmount
	//	}
	//
	//}
}

func (matcher *TimingMatcher) localAllMarkets() Markets {
	markets := Markets{}
	for _, tokenPair := range marketUtilLib.AllTokenPairs {
		if !markets.contain(tokenPair.TokenB, tokenPair.TokenS) {
			for _, protocolImpl := range loopringaccessor.ProtocolAddresses() {
				market := &Market{}
				market.protocolImpl = protocolImpl
				market.TokenA = tokenPair.TokenS
				market.TokenB = tokenPair.TokenB
				market.matcher = matcher
				market.AtoBOrders = &OrdersState{Orders: make(map[common.Hash]*types.OrderState), OrderHashesExcludeNextRound: []common.Hash{}}
				market.BtoAOrders = &OrdersState{Orders: make(map[common.Hash]*types.OrderState), OrderHashesExcludeNextRound: []common.Hash{}}
				markets = append(markets, market)
			}
		}
	}
	return markets
}
