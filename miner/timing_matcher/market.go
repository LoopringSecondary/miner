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
	"fmt"
	"github.com/Loopring/miner/dao"
	"github.com/Loopring/miner/datasource"
	"github.com/Loopring/miner/miner"
	"github.com/Loopring/relay-lib/eth/loopringaccessor"
	"github.com/Loopring/relay-lib/kafka"
	"github.com/Loopring/relay-lib/log"
	"github.com/Loopring/relay-lib/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"sort"
	"strings"
)

type Market struct {
	matcher      *TimingMatcher
	protocolImpl *loopringaccessor.ProtocolAddress

	TokenA common.Address
	TokenB common.Address

	AtoBOrders *OrdersState
	BtoAOrders *OrdersState

	//AtoBOrders map[common.Hash]*types.OrderState
	//BtoAOrders map[common.Hash]*types.OrderState
	//
	//AtoBOrderHashesExcludeNextRound []common.Hash
	//BtoAOrderHashesExcludeNextRound []common.Hash
}

type OrdersState struct {
	Orders                      map[common.Hash]*types.OrderState
	OrderHashesExcludeNextRound []common.Hash
}

func (orders *OrdersState) prepareOrders(delegateAddress common.Address, market *Market, tokenS, tokenB common.Address) {
	log.Debugf("prepareOrders, tokenS:%s, tokenB:%s", tokenS.Hex(), tokenB.Hex())
	currentRoundNumber := market.matcher.lastRoundNumber.Int64()
	deleyedNumber := market.matcher.delayedNumber + currentRoundNumber

	orders1 := datasource.MinerOrders(delegateAddress, tokenS, tokenB, market.matcher.roundOrderCount, market.matcher.reservedTime, int64(0), currentRoundNumber, &types.OrderDelayList{OrderHash: orders.OrderHashesExcludeNextRound, DelayedCount: deleyedNumber})

	if len(orders1) < market.matcher.roundOrderCount {
		orderCount := market.matcher.roundOrderCount - len(orders1)
		orders2 := datasource.MinerOrders(delegateAddress, tokenS, tokenB, orderCount, market.matcher.reservedTime, currentRoundNumber+1, currentRoundNumber+market.matcher.delayedNumber)
		orders1 = append(orders1, orders2...)
	}

	for _, order := range orders1 {
		market.reduceRemainedAmountBeforeMatch(order)
		if !market.isOrderFinished(order) {
			orders.Orders[order.RawOrder.Hash] = order
		} else {
			orders.OrderHashesExcludeNextRound = append(orders.OrderHashesExcludeNextRound, order.RawOrder.Hash)
		}
		log.Debugf("order status in this new round:%s, orderhash:%s, DealtAmountS:%s, ", market.matcher.lastRoundNumber.String(), order.RawOrder.Hash.Hex(), order.DealtAmountS.String())
	}
}

func (orders *OrdersState) applyExcludeHash(matchedOrderHashes map[common.Hash]bool, roundOrderCount int) {
	for orderHash, _ := range orders.Orders {
		fullFilled, exists := matchedOrderHashes[orderHash]
		if exists && fullFilled {
			orders.OrderHashesExcludeNextRound = append(orders.OrderHashesExcludeNextRound, orderHash)
		} else if !exists && (len(orders.Orders) >= roundOrderCount) {
			orders.OrderHashesExcludeNextRound = append(orders.OrderHashesExcludeNextRound, orderHash)
		}
	}
}

func (market *Market) match() {
	market.getOrdersForMatching(market.protocolImpl.DelegateAddress)
	matchedOrderHashes := make(map[common.Hash]bool) //true:fullfilled, false:partfilled
	candidateRingList := CandidateRingList{}

	//step 1: evaluate received
	for _, a2BOrder := range market.AtoBOrders.Orders {
		if failedCount, err1 := OrderExecuteFailedCount(a2BOrder.RawOrder.Hash); nil == err1 && failedCount > market.matcher.maxFailedCount {
			log.Debugf("orderhash:%s has been failed to submit %d times", a2BOrder.RawOrder.Hash.Hex(), failedCount)
			continue
		}
		for _, b2AOrder := range market.BtoAOrders.Orders {
			if failedCount, err1 := OrderExecuteFailedCount(b2AOrder.RawOrder.Hash); nil == err1 && failedCount > market.matcher.maxFailedCount {
				log.Debugf("orderhash:%s has been failed to submit %d times", b2AOrder.RawOrder.Hash.Hex(), failedCount)
				continue
			}
			//todo:move a2BOrder.RawOrder.Owner != b2AOrder.RawOrder.Owner after contract fix bug
			if miner.PriceValid(a2BOrder, b2AOrder) && a2BOrder.RawOrder.Owner != b2AOrder.RawOrder.Owner {
				if candidateRing, err := market.GenerateCandidateRing(a2BOrder, b2AOrder); nil != err {
					log.Errorf("err:%s", err.Error())
					continue
				} else {
					if candidateRing.received.Sign() > 0 {
						candidateRingList = append(candidateRingList, *candidateRing)
					} else {
						log.Debugf("timing_matchher, market ringForSubmit received not enough, received:%s, cost:%s ", candidateRing.received.FloatString(0), candidateRing.cost.FloatString(0))
					}
				}
			}
		}
	}

	log.Debugf("match round:%s, market: %s -> %s , candidateRingList.length:%d", market.matcher.lastRoundNumber, market.TokenA.Hex(), market.TokenB.Hex(), len(candidateRingList))
	//the ring that can get max received
	list := candidateRingList
	for {
		if len(list) <= 0 {
			break
		}

		sort.Sort(list)
		candidateRing := list[0]
		list = list[1:]
		orders := []*types.OrderState{}
		for hash, _ := range candidateRing.filledOrders {
			if o, exists := market.AtoBOrders.Orders[hash]; exists {
				orders = append(orders, o)
			} else {
				orders = append(orders, market.BtoAOrders.Orders[hash])
			}
		}
		if ringForSubmit, err := market.generateRingSubmitInfo(orders...); nil != err {
			log.Debugf("generate RingSubmitInfo err:%s", err.Error())
			continue
		} else {

			if exists, err := CachedMatchedRing(ringForSubmit.Ringhash); nil != err || exists {
				if nil != err {
					log.Error(err.Error())
				} else {
					log.Errorf("ringhash:%s has been submitted", ringForSubmit.Ringhash.Hex())
				}
				continue
			}
			uniqueId := ringForSubmit.RawRing.GenerateUniqueId()
			if failedCount, err := RingExecuteFailedCount(uniqueId); nil == err && failedCount > market.matcher.maxFailedCount {
				log.Debugf("ringSubmitInfo.UniqueId:%s , ringhash: %s , has been failed to submit %d times", uniqueId.Hex(), ringForSubmit.Ringhash.Hex(), failedCount)
				continue
			}

			//todo:for test, release this limit
			if ringForSubmit.RawRing.Received.Sign() > 0 {
				for _, filledOrder := range ringForSubmit.RawRing.Orders {
					orderState := market.reduceAmountAfterFilled(filledOrder)
					isFullFilled := market.isOrderFinished(orderState)
					matchedOrderHashes[filledOrder.OrderState.RawOrder.Hash] = isFullFilled

					list = market.reduceReceivedOfCandidateRing(list, filledOrder, isFullFilled)
				}
				AddMinedRing(ringForSubmit)
				//ringSubmitInfos = append(ringSubmitInfos, ringForSubmit)
				if err := market.saveAndSendSubmitInfo(ringForSubmit); nil != err {
					log.Errorf("err:%s", err.Error())
					RemoveMinedRingAndReturnOrderhashes(ringForSubmit.Ringhash)
				}
			} else {
				log.Debugf("ring:%s will not be submitted,because of received:%s", ringForSubmit.RawRing.Hash.Hex(), ringForSubmit.RawRing.Received.String())
			}
		}
	}

	market.AtoBOrders.applyExcludeHash(matchedOrderHashes, market.matcher.roundOrderCount)
	market.BtoAOrders.applyExcludeHash(matchedOrderHashes, market.matcher.roundOrderCount)
}

func (market *Market) saveAndSendSubmitInfo(ringState *types.RingSubmitInfo) error {
	daoInfo := &dao.RingSubmitInfo{}
	daoInfo.ConvertDown(ringState, errors.New(""))
	for _, filledOrder := range ringState.RawRing.Orders {
		daoOrder := &dao.FilledOrder{}
		daoOrder.ConvertDown(filledOrder, ringState.Ringhash)
		if err1 := market.matcher.db.Add(daoOrder); nil != err1 {
			log.Errorf("insert filled Order err:%s", err1.Error())
			return err1
		}
	}
	if err := market.matcher.db.Add(daoInfo); nil != err {
		log.Errorf("insert new ring err:%s", err.Error())
		return err
	}

	evt := &types.RingSubmitInfoEvent{}
	evt.SubmitInfoId = daoInfo.ID
	evt.Miner = ringState.Miner
	evt.Ringhash = ringState.Ringhash
	evt.UniqueId = ringState.RawRing.GenerateUniqueId()
	evt.ProtocolAddress = ringState.ProtocolAddress
	evt.ProtocolGas = ringState.ProtocolGas

	//todo:
	evt.ProtocolGas = big.NewInt(int64(400000))
	evt.ProtocolGasPrice = ringState.ProtocolGasPrice
	evt.ProtocolData = common.ToHex(ringState.ProtocolData)
	evt.ValidSinceTime = ringState.RawRing.ValidSinceTime()

	if _, _, err := market.matcher.messageProducer.SendMessage(kafka.Kafka_Topic_Miner_SubmitInfo_Prefix+strings.ToLower(evt.Miner.Hex()), evt, evt.Ringhash.Hex()); nil != err {
		log.Errorf("err:%s", err.Error())
		if err1 := market.matcher.db.UpdateRingSubmitInfoErrById(evt.SubmitInfoId, err); nil != err1 {
			log.Errorf("err:%s", err1.Error())
		}
		return err
	}
	return nil
}

func (market *Market) reduceReceivedOfCandidateRing(list CandidateRingList, filledOrder *types.FilledOrder, isFullFilled bool) CandidateRingList {
	resList := CandidateRingList{}
	hash := filledOrder.OrderState.RawOrder.Hash
	for _, ring := range list {
		if amountS, exists := ring.filledOrders[hash]; exists {
			if isFullFilled {
				continue
			}
			availableAmountS := new(big.Rat)
			availableAmountS.Sub(filledOrder.AvailableAmountS, filledOrder.FillAmountS)
			if availableAmountS.Sign() > 0 {
				var remainedAmountS *big.Rat
				if amountS.Cmp(availableAmountS) >= 0 {
					remainedAmountS = availableAmountS
				} else {
					remainedAmountS = amountS
				}
				log.Debugf("reduceReceivedOfCandidateRing, filledOrder.availableAmountS:%s, filledOrder.FillAmountS:%s, amountS:%s", filledOrder.AvailableAmountS.FloatString(3), filledOrder.FillAmountS.FloatString(3), amountS.FloatString(3))
				rate := new(big.Rat)
				rate.Quo(remainedAmountS, amountS)
				remainedReceived := new(big.Rat).Add(ring.received, ring.cost)
				remainedReceived.Mul(remainedReceived, rate).Sub(remainedReceived, ring.cost)
				//todo:
				if remainedReceived.Sign() <= 0 {
					continue
				}
				for hash, amount := range ring.filledOrders {
					ring.filledOrders[hash] = amount.Mul(amount, rate)
				}
				resList = append(resList, ring)
			}
		} else {
			resList = append(resList, ring)
		}
	}
	return resList
}

/**
get orders from ordermanager
*/
func (market *Market) getOrdersForMatching(delegateAddress common.Address) {
	market.AtoBOrders.Orders = make(map[common.Hash]*types.OrderState)
	market.BtoAOrders.Orders = make(map[common.Hash]*types.OrderState)

	market.AtoBOrders.prepareOrders(delegateAddress, market, market.TokenA, market.TokenB)
	market.BtoAOrders.prepareOrders(delegateAddress, market, market.TokenB, market.TokenA)

	////log.Debugf("#### %s,%s %d,%d %d",market.TokenA.Hex(),market.TokenB.Hex(), len(atoBOrders), len(btoAOrders),market.matcher.roundOrderCount)
}

//sub the matched amount in new round.
func (market *Market) reduceRemainedAmountBeforeMatch(orderState *types.OrderState) {
	orderHash := orderState.RawOrder.Hash

	if amountS, amountB, err := DealtAmount(orderHash); nil != err {
		log.Errorf("err:%s", err.Error())
	} else {
		log.Debugf("reduceRemainedAmountBeforeMatch:%s, %s, %s", orderState.RawOrder.Owner.Hex(), amountS.String(), amountB.String())
		orderState.DealtAmountB.Add(orderState.DealtAmountB, ratToInt(amountB))
		orderState.DealtAmountS.Add(orderState.DealtAmountS, ratToInt(amountS))
	}
}

func (market *Market) reduceAmountAfterFilled(filledOrder *types.FilledOrder) *types.OrderState {
	filledOrderState := filledOrder.OrderState
	var orderState *types.OrderState

	//only one of DealtAmountB and DealtAmountS is precise
	if filledOrderState.RawOrder.TokenS == market.TokenA {
		orderState = market.AtoBOrders.Orders[filledOrderState.RawOrder.Hash]
		orderState.DealtAmountB.Add(orderState.DealtAmountB, ratToInt(filledOrder.FillAmountB))
		orderState.DealtAmountS.Add(orderState.DealtAmountS, ratToInt(filledOrder.FillAmountS))
	} else {
		orderState = market.BtoAOrders.Orders[filledOrderState.RawOrder.Hash]
		orderState.DealtAmountB.Add(orderState.DealtAmountB, ratToInt(filledOrder.FillAmountB))
		orderState.DealtAmountS.Add(orderState.DealtAmountS, ratToInt(filledOrder.FillAmountS))
	}
	log.Debugf("order status after matched, orderhash:%s,filledAmountS:%s, DealtAmountS:%s, ", orderState.RawOrder.Hash.Hex(), filledOrder.FillAmountS.String(), orderState.DealtAmountS.String())
	//reduced account balance

	return orderState
}

func (market *Market) GenerateCandidateRing(orders ...*types.OrderState) (*CandidateRing, error) {
	filledOrders := []*types.FilledOrder{}
	//miner will received nothing, if miner set FeeSelection=1 and he doesn't have enough lrc
	for _, order := range orders {
		if filledOrder, err := market.generateFilledOrder(order); nil != err {
			log.Errorf("err:%s", err.Error())
			return nil, err
		} else {
			filledOrders = append(filledOrders, filledOrder)
		}
	}

	ringTmp := miner.NewRing(filledOrders)
	if err := market.matcher.evaluator.ComputeRing(ringTmp); nil != err {
		return nil, err
	} else {
		candidateRing := &CandidateRing{cost: ringTmp.LegalCost, received: ringTmp.Received, filledOrders: make(map[common.Hash]*big.Rat)}
		for _, filledOrder := range ringTmp.Orders {
			log.Debugf("match, orderhash:%s, filledOrder.FilledAmountS:%s", filledOrder.OrderState.RawOrder.Hash.Hex(), filledOrder.FillAmountS.FloatString(3))
			candidateRing.filledOrders[filledOrder.OrderState.RawOrder.Hash] = filledOrder.FillAmountS
		}
		return candidateRing, nil
	}
}

func (market *Market) generateFilledOrder(order *types.OrderState) (*types.FilledOrder, error) {

	lrcTokenBalance, err := market.matcher.GetAccountAvailableAmount(order.RawOrder.Owner, market.protocolImpl.LrcTokenAddress, market.protocolImpl.DelegateAddress)
	if nil != err {
		return nil, err
	}

	tokenSBalance, err := market.matcher.GetAccountAvailableAmount(order.RawOrder.Owner, order.RawOrder.TokenS, market.protocolImpl.DelegateAddress)
	if nil != err {
		return nil, err
	}
	if tokenSBalance.Sign() <= 0 {
		return nil, fmt.Errorf("owner:%s token:%s balance or allowance is zero", order.RawOrder.Owner.Hex(), order.RawOrder.TokenS.Hex())
	}
	//todo:
	if market.isDustValue(order.RawOrder.TokenS, tokenSBalance) {
		return nil, fmt.Errorf("owner:%s token:%s balance or allowance is not enough", order.RawOrder.Owner.Hex(), order.RawOrder.TokenS.Hex())
	}
	return types.ConvertOrderStateToFilledOrder(*order, lrcTokenBalance, tokenSBalance, market.protocolImpl.LrcTokenAddress), nil
}

func (market *Market) generateRingSubmitInfo(orders ...*types.OrderState) (*types.RingSubmitInfo, error) {
	filledOrders := []*types.FilledOrder{}
	//miner will received nothing, if miner set FeeSelection=1 and he doesn't have enough lrc
	for _, order := range orders {
		if filledOrder, err := market.generateFilledOrder(order); nil != err {
			log.Errorf("err:%s", err.Error())
			return nil, err
		} else {
			filledOrders = append(filledOrders, filledOrder)
		}
	}

	ringTmp := miner.NewRing(filledOrders)
	if err := market.matcher.evaluator.ComputeRing(ringTmp); nil != err {
		return nil, err
	} else {
		res, err := market.matcher.submitter.GenerateRingSubmitInfo(ringTmp)
		return res, err
	}
}

func NewMarket(protocolAddress *loopringaccessor.ProtocolAddress, tokenS, tokenB common.Address, matcher *TimingMatcher) *Market {

	m := &Market{}
	m.protocolImpl = protocolAddress
	m.matcher = matcher
	m.TokenA = tokenS
	m.TokenB = tokenB
	return m
}

func ratToInt(rat *big.Rat) *big.Int {
	return new(big.Int).Div(rat.Num(), rat.Denom())
}

func (market *Market) isOrderFinished(orderState *types.OrderState) bool {
	return market.matcher.marketCapProvider.IsOrderValueDust(orderState)
}

func (market *Market) isDustValue(tokenAddr common.Address, value *big.Rat) bool {
	legalValue, _ := market.matcher.marketCapProvider.LegalCurrencyValue(tokenAddr, value)
	return market.matcher.marketCapProvider.IsValueDusted(legalValue)
}
