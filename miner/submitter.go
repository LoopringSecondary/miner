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

package miner

import (
	"errors"
	"math/big"

	"github.com/Loopring/miner/config"
	"github.com/Loopring/miner/dao"
	"github.com/Loopring/relay-lib/eth/accessor"
	"github.com/Loopring/relay-lib/eth/contract"
	"github.com/Loopring/relay-lib/eth/loopringaccessor"
	libEthTypes "github.com/Loopring/relay-lib/eth/types"
	"github.com/Loopring/relay-lib/eventemitter"
	"github.com/Loopring/relay-lib/kafka"
	"github.com/Loopring/relay-lib/log"
	"github.com/Loopring/relay-lib/types"
	"github.com/Loopring/relay-lib/zklock"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"strings"
)

//保存ring，并将ring发送到区块链，同样需要分为待完成和已完成
type RingSubmitter struct {
	minerAccountForSign accounts.Account
	feeReceipt          common.Address
	currentBlockTime    int64

	maxGasLimit *big.Int
	minGasLimit *big.Int

	normalMinerAddresses  []*NormalSenderAddress
	percentMinerAddresses []*SplitMinerAddress

	dbService dao.RdsServiceImpl
	matcher   Matcher

	messageProducer *kafka.MessageProducer

	newRingSubmitInfoConsumer *kafka.ConsumerRegister

	stopFuncs []func()
}

type RingSubmitFailed struct {
	RingState *types.Ring
	err       error
}

func getKafkaGroup() string {
	return "submitter_"
}

func NewSubmitter(options config.MinerOptions, dbService dao.RdsServiceImpl, brokers []string) (*RingSubmitter, error) {
	submitter := &RingSubmitter{}
	submitter.maxGasLimit = big.NewInt(options.MaxGasLimit)
	submitter.minGasLimit = big.NewInt(options.MinGasLimit)
	if common.IsHexAddress(options.FeeReceipt) {
		submitter.feeReceipt = common.HexToAddress(options.FeeReceipt)
	} else {
		return submitter, errors.New("miner.feeReceipt must be a address")
	}

	submitter.newRingSubmitInfoConsumer = &kafka.ConsumerRegister{}
	submitter.newRingSubmitInfoConsumer.Initialize(brokers)

	for _, addr := range options.NormalMiners {
		var nonce types.Big
		normalAddr := common.HexToAddress(addr.Address)
		if err := accessor.GetTransactionCount(&nonce, normalAddr, "pending"); nil != err {
			log.Errorf("err:%s", err.Error())
		}
		miner := &NormalSenderAddress{}
		miner.Address = normalAddr
		miner.GasPriceLimit = big.NewInt(addr.GasPriceLimit)
		miner.MaxPendingCount = addr.MaxPendingCount
		miner.MaxPendingTtl = addr.MaxPendingTtl
		miner.Nonce = nonce.BigInt()
		submitter.normalMinerAddresses = append(submitter.normalMinerAddresses, miner)
	}

	for _, addr := range options.PercentMiners {
		var nonce types.Big
		normalAddr := common.HexToAddress(addr.Address)
		if err := accessor.GetTransactionCount(&nonce, normalAddr, "pending"); nil != err {
			log.Errorf("err:%s", err.Error())
		}
		miner := &SplitMinerAddress{}
		miner.Nonce = nonce.BigInt()
		miner.Address = normalAddr
		miner.FeePercent = addr.FeePercent
		miner.StartFee = addr.StartFee
		submitter.percentMinerAddresses = append(submitter.percentMinerAddresses, miner)
	}

	submitter.dbService = dbService

	if len(brokers) > 0 {
		submitter.messageProducer = &kafka.MessageProducer{}
		if err := submitter.messageProducer.Initialize(brokers); nil != err {
			log.Fatalf("Failed init producerWrapped %s", err.Error())
		}
	} else {
		log.Errorf("There is not brokers of kafka to send msg.")
	}
	submitter.stopFuncs = []func(){}
	return submitter, nil
}

func (submitter *RingSubmitter) listenBlockNew() {
	blockEventChan := make(chan *types.BlockEvent)
	go func() {
		for {
			select {
			case blockEvent := <-blockEventChan:
				submitter.currentBlockTime = blockEvent.BlockTime
			}
		}
	}()

	watcher := &eventemitter.Watcher{
		Concurrent: false,
		Handle: func(eventData eventemitter.EventData) error {
			e := eventData.(*types.BlockEvent)
			log.Debugf("submitter.listenBlockNew blockNumber:%s, blocktime:%d", e.BlockNumber.String(), e.BlockTime)
			blockEventChan <- e
			return nil
		},
	}
	eventemitter.On(eventemitter.Block_New, watcher)
	submitter.stopFuncs = append(submitter.stopFuncs, func() {
		close(blockEventChan)
		eventemitter.Un(eventemitter.Block_New, watcher)
	})
}

//func (submitter *RingSubmitter) listenNewRings() {
//	//ringSubmitInfoChan := make(chan []*types.RingSubmitInfo)
//	//go func() {
//	//	for {
//	//		select {
//	//		case ringInfos := <-ringSubmitInfoChan:
//	//			if nil != ringInfos {
//	//				for _, ringState := range ringInfos {
//	//					txHash, status, err1 := submitter.submitRing(ringState)
//	//					ringState.SubmitTxHash = txHash
//	//
//	//					daoInfo := &dao.RingSubmitInfo{}
//	//					daoInfo.ConvertDown(ringState, err1)
//	//					if err := submitter.dbService.Add(daoInfo); nil != err {
//	//						log.Errorf("Miner submitter,insert new ring err:%s", err.Error())
//	//					} else {
//	//						for _, filledOrder := range ringState.RawRing.Orders {
//	//							daoOrder := &dao.FilledOrder{}
//	//							daoOrder.ConvertDown(filledOrder, ringState.Ringhash)
//	//							if err1 := submitter.dbService.Add(daoOrder); nil != err1 {
//	//								log.Errorf("Miner submitter,insert filled Order err:%s", err1.Error())
//	//							}
//	//						}
//	//					}
//	//					submitter.submitResult(ringState.Ringhash, ringState.RawRing.GenerateUniqueId(), txHash, status, big.NewInt(0), big.NewInt(0), big.NewInt(0), err1)
//	//				}
//	//			}
//	//		}
//	//	}
//	//}()
//	watcher := &eventemitter.Watcher{
//		Concurrent: false,
//		Handle: func(eventData eventemitter.EventData) error {
//			ringInfos := eventData.([]*types.RingSubmitInfo)
//			log.Debugf("received ringstates length:%d", len(ringInfos))
//			//ringSubmitInfoChan <- e
//			if nil != ringInfos {
//				for _, ringState := range ringInfos {
//					txHash, status, err1 := submitter.submitRing(ringState)
//					ringState.SubmitTxHash = txHash
//
//					daoInfo := &dao.RingSubmitInfo{}
//					daoInfo.ConvertDown(ringState, err1)
//					if err := submitter.dbService.Add(daoInfo); nil != err {
//						log.Errorf("Miner submitter,insert new ring err:%s", err.Error())
//					} else {
//						for _, filledOrder := range ringState.RawRing.Orders {
//							daoOrder := &dao.FilledOrder{}
//							daoOrder.ConvertDown(filledOrder, ringState.Ringhash)
//							if err1 := submitter.dbService.Add(daoOrder); nil != err1 {
//								log.Errorf("Miner submitter,insert filled Order err:%s", err1.Error())
//							}
//						}
//					}
//					submitter.submitResult(ringState.Ringhash, ringState.RawRing.GenerateUniqueId(), txHash, status, big.NewInt(0), big.NewInt(0), big.NewInt(0), err1)
//				}
//			}
//			return nil
//		},
//	}
//	eventemitter.On(eventemitter.Miner_NewRing, watcher)
//	submitter.stopFuncs = append(submitter.stopFuncs, func() {
//		//close(ringSubmitInfoChan)
//		eventemitter.Un(eventemitter.Miner_NewRing, watcher)
//	})
//}

func (submitter *RingSubmitter) handleNewRing(input interface{}) error {
	if evt, ok := input.(*types.RingSubmitInfoEvent); ok {
		txhash, status, tx, err := submitter.submitRing(evt.Miner, evt.ProtocolAddress, evt.Ringhash, evt.ProtocolGas, evt.ProtocolGasPrice, common.FromHex(evt.ProtocolData))
		if nil != err {
			log.Errorf("err:%s", err.Error())
		}
		if types.TX_STATUS_PENDING == status {
			submitter.sendPendingTransaction(tx, evt.Miner)
		}
		submitter.submitResult(evt.SubmitInfoId, evt.Ringhash, evt.UniqueId, txhash, status, big.NewInt(0), big.NewInt(0), big.NewInt(0), err)
	} else {
		log.Errorf("receive submitInfo ,but type not match")
		return errors.New("type not match")
	}
	return nil
}

func (submitter *RingSubmitter) submitRing(miner, protocolAddress common.Address, ringhash common.Hash, gas, gasPrice *big.Int, callData []byte) (common.Hash, types.TxStatus, *ethTypes.Transaction, error) {
	status := types.TX_STATUS_PENDING
	//ordersStr, _ := json.Marshal(ringSubmitInfo.RawRing.Orders)
	//log.Debugf("submitring hash:%s, orders:%s", ringSubmitInfo.Ringhash.Hex(), string(ordersStr))

	txHash := types.NilHash
	var err error

	var tx *ethTypes.Transaction
	if nil == err {
		txHashStr := "0x"
		txHashStr, tx, err = accessor.SignAndSendTransaction(miner, protocolAddress, gas, gasPrice, nil, callData, false)
		if nil != err {
			log.Errorf("submitring hash:%s, err:%s", ringhash, err.Error())
			status = types.TX_STATUS_FAILED
		}

		txHash = common.HexToHash(txHashStr)
	} else {
		log.Errorf("submitring hash:%s, protocol:%s, err:%s", ringhash.Hex(), protocolAddress.Hex(), err.Error())
		status = types.TX_STATUS_FAILED
	}

	return txHash, status, tx, err
}

func (submitter *RingSubmitter) sendPendingTransaction(tx *ethTypes.Transaction, from common.Address) {
	if nil != tx {
		//hash,nonce,from,to,value,gasprice,gas,input
		libTx := &libEthTypes.Transaction{}
		libTx.Hash = tx.Hash().Hex()
		libTx.From = from.Hex()
		libTx.To = tx.To().Hex()
		gas, gasPrice, nonce, value := new(types.Big), new(types.Big), new(types.Big), new(types.Big)
		gas.SetInt(new(big.Int).SetUint64(tx.Gas()))
		gasPrice.SetInt(tx.GasPrice())
		nonce.SetInt(new(big.Int).SetUint64(tx.Nonce()))
		value.SetInt(tx.Value())
		libTx.Value = *value
		libTx.Gas = *gas
		libTx.GasPrice = *gasPrice
		libTx.Input = common.ToHex(tx.Data())
		libTx.Nonce = *nonce
		if nil != submitter.messageProducer {
			if _, _, err2 := submitter.messageProducer.SendMessage(kafka.Kafka_Topic_Extractor_PendingTransaction, libTx, libTx.Hash); nil != err2 {
				log.Errorf("err:%s", err2.Error())
			}
		} else {
			log.Debugf("submitter.messageProducer is nil, and the submitorderEvent will not be send.")
		}
	}
}

func (submitter *RingSubmitter) listenSubmitRingMethodEvent() {
	watcher := &eventemitter.Watcher{
		Concurrent: false,
		Handle: func(eventData eventemitter.EventData) error {
			var (
				txhash      common.Hash
				status      types.TxStatus
				blockNumber *big.Int
				gasUsed     *big.Int
				eventErr    error
			)
			if e, ok := eventData.(*types.SubmitRingMethodEvent); ok {
				txhash = e.TxHash
				status = e.Status
				blockNumber = e.BlockNumber
				gasUsed = e.GasUsed
				eventErr = errors.New(e.Err)
			} else if e1, ok1 := eventData.(*types.RingMinedEvent); ok1 {
				txhash = e1.TxHash
				status = e1.Status
				blockNumber = e1.BlockNumber
				gasUsed = e1.GasUsed
				eventErr = errors.New(e1.Err)
			}
			log.Debugf("eventemitter.Watchereventemitter.Watcher txhash:%s", txhash.Hex())
			if infos, err := submitter.dbService.GetRingHashesByTxHash(txhash); nil != err {
				log.Errorf("err:%s", err.Error())
			} else {
				for _, info := range infos {
					ringhash := common.HexToHash(info.RingHash)
					uniqueId := common.HexToHash(info.UniqueId)
					submitter.submitResult(0, ringhash, uniqueId, txhash, status, big.NewInt(0), blockNumber, gasUsed, eventErr)
				}
			}
			return nil
		},
	}
	eventemitter.On(eventemitter.Miner_SubmitRing_Method, watcher)
	eventemitter.On(eventemitter.RingMined, watcher)
	submitter.stopFuncs = append(submitter.stopFuncs, func() {
		eventemitter.Un(eventemitter.Miner_SubmitRing_Method, watcher)
		eventemitter.Un(eventemitter.RingMined, watcher)
	})
}

func (submitter *RingSubmitter) submitResult(recordId int, ringhash, uniqeId, txhash common.Hash, status types.TxStatus, ringIndex, blockNumber, usedGas *big.Int, err error) {
	if nil == err {
		err = errors.New("")
	}
	resultEvt := &types.RingSubmitResultEvent{
		RecordId:     recordId,
		RingHash:     ringhash,
		RingUniqueId: uniqeId,
		TxHash:       txhash,
		Status:       status,
		Err:          err.Error(),
		RingIndex:    ringIndex,
		BlockNumber:  blockNumber,
		UsedGas:      usedGas,
	}
	if err := submitter.dbService.UpdateRingSubmitInfoResult(resultEvt); nil != err {
		log.Errorf("err:%s", err.Error())
	}
	eventemitter.Emit(eventemitter.Miner_RingSubmitResult, resultEvt)
}

////提交错误，执行错误
//func (submitter *RingSubmitter) submitFailed(ringhashes []common.Hash, err error) {
//	if err := submitter.dbService.UpdateRingSubmitInfoFailed(ringhashes, err.Error()); nil != err {
//		log.Errorf("err:%s", err.Error())
//	} else {
//		for _, ringhash := range ringhashes {
//			failedEvent := &types.RingSubmitResultEvent{RingHash: ringhash, Status:types.TX_STATUS_FAILED}
//			eventemitter.Emit(eventemitter.Miner_RingSubmitResult, failedEvent)
//		}
//	}
//}

func (submitter *RingSubmitter) GenerateRingSubmitInfo(ringState *types.Ring) (*types.RingSubmitInfo, error) {
	//todo:change to advice protocolAddress
	protocolAddress := ringState.Orders[0].OrderState.RawOrder.Protocol
	var (
	//signer *types.NameRegistryInfo
	//err error
	)

	ringSubmitInfo := &types.RingSubmitInfo{RawRing: ringState, ProtocolGasPrice: ringState.GasPrice, ProtocolGas: ringState.Gas}
	if types.IsZeroHash(ringState.Hash) {
		ringState.Hash = ringState.GenerateHash(submitter.feeReceipt)
	}

	ringSubmitInfo.ProtocolAddress = protocolAddress
	ringSubmitInfo.OrdersCount = big.NewInt(int64(len(ringState.Orders)))
	ringSubmitInfo.Ringhash = ringState.Hash

	protocolAbi := loopringaccessor.ProtocolImplAbi()
	if senderAddress, err := submitter.selectSenderAddress(); nil != err {
		return ringSubmitInfo, err
	} else {
		ringSubmitInfo.Miner = senderAddress
	}
	//submitter.computeReceivedAndSelectMiner(ringSubmitInfo)
	if protocolData, err := contract.GenerateSubmitRingMethodInputsData(ringState, submitter.feeReceipt, protocolAbi); nil != err {
		return nil, err
	} else {
		ringSubmitInfo.ProtocolData = protocolData
	}
	//if nil != err {
	//	return nil, err
	//}
	//预先判断是否会提交成功
	lastTime := ringSubmitInfo.RawRing.ValidSinceTime()
	if submitter.currentBlockTime > 0 && lastTime <= submitter.currentBlockTime {
		var err error
		_, _, err = accessor.EstimateGas(ringSubmitInfo.ProtocolData, ringSubmitInfo.ProtocolAddress, "latest")
		//ringSubmitInfo.ProtocolGas, ringSubmitInfo.ProtocolGasPrice, err = ethaccessor.EstimateGas(ringSubmitInfo.ProtocolData, protocolAddress, "latest")
		if nil != err {
			log.Errorf("can't generate ring ,err:%s", err.Error())
			return nil, err
		}
	}
	//if nil != err {
	//	return nil, err
	//}
	if submitter.maxGasLimit.Sign() > 0 && ringSubmitInfo.ProtocolGas.Cmp(submitter.maxGasLimit) > 0 {
		ringSubmitInfo.ProtocolGas.Set(submitter.maxGasLimit)
	}
	if submitter.minGasLimit.Sign() > 0 && ringSubmitInfo.ProtocolGas.Cmp(submitter.minGasLimit) < 0 {
		ringSubmitInfo.ProtocolGas.Set(submitter.minGasLimit)
	}
	return ringSubmitInfo, nil
}

func (submitter *RingSubmitter) stop() {
	for _, stop := range submitter.stopFuncs {
		stop()
	}
}

const (
	ZKLOCK_SUBMITTER_MINER_ADDR_PRE = "zklock_submitter_miner_addr_"
)

func (submitter *RingSubmitter) listenNewRings() {
	for _, minerAddr := range submitter.normalMinerAddresses {
		go func(minerAddr common.Address) {
			addr := strings.ToLower(minerAddr.Hex())
			zklock.TryLock(ZKLOCK_SUBMITTER_MINER_ADDR_PRE + addr)
			submitter.stopFuncs = append(submitter.stopFuncs, func() {
				zklock.ReleaseLock(ZKLOCK_SUBMITTER_MINER_ADDR_PRE + addr)
			})
			submitter.newRingSubmitInfoConsumer.RegisterTopicAndHandler(kafka.Kafka_Topic_Miner_SubmitInfo_Prefix+addr, getKafkaGroup(), types.RingSubmitInfoEvent{}, submitter.handleNewRing)
		}(minerAddr.Address)
	}
}

func (submitter *RingSubmitter) start() {
	submitter.listenNewRings()
	//submitter.listenSubmitRingMethodEventFromMysql()
	submitter.listenBlockNew()
	submitter.listenSubmitRingMethodEvent()
}

func (submitter *RingSubmitter) availableSenderAddresses() []*NormalSenderAddress {
	senderAddresses := []*NormalSenderAddress{}
	for _, minerAddress := range submitter.normalMinerAddresses {
		var blockedTxCount, txCount types.Big
		//todo:change it by event
		accessor.GetTransactionCount(&blockedTxCount, minerAddress.Address, "latest")
		accessor.GetTransactionCount(&txCount, minerAddress.Address, "pending")
		//todo:check ethbalance
		pendingCount := big.NewInt(int64(0))
		pendingCount.Sub(txCount.BigInt(), blockedTxCount.BigInt())
		if pendingCount.Int64() <= minerAddress.MaxPendingCount {
			senderAddresses = append(senderAddresses, minerAddress)
		}
	}

	if len(senderAddresses) <= 0 {
		senderAddresses = append(senderAddresses, submitter.normalMinerAddresses[0])
	}
	return senderAddresses
}

func (submitter *RingSubmitter) selectSenderAddress() (common.Address, error) {
	senderAddresses := submitter.availableSenderAddresses()
	if len(senderAddresses) <= 0 {
		return types.NilAddress, errors.New("there isn't an available sender address")
	} else {
		return senderAddresses[0].Address, nil
	}
}

//func (submitter *RingSubmitter) computeReceivedAndSelectMiner(ringSubmitInfo *types.RingSubmitInfo) error {
//	ringState := ringSubmitInfo.RawRing
//	ringState.LegalFee = new(big.Rat).SetInt(big.NewInt(int64(0)))
//	ethPrice, _ := submitter.marketCapProvider.GetEthCap()
//	ethPrice = ethPrice.Quo(ethPrice, new(big.Rat).SetInt(util.AllTokens["WETH"].Decimals))
//	lrcAddress := loopringaccessor.ProtocolAddresses()[ringSubmitInfo.ProtocolAddress].LrcTokenAddress
//	spenderAddress := loopringaccessor.ProtocolAddresses()[ringSubmitInfo.ProtocolAddress].DelegateAddress
//	useSplit := false
//	//for _,splitMiner := range submitter.splitMinerAddresses {
//	//	//todo:optimize it
//	//	if lrcFee > splitMiner.StartFee || splitFee > splitMiner.StartFee || len(submitter.normalMinerAddresses) <= 0  {
//	//		useSplit = true
//	//		ringState.Miner = splitMiner.Address
//	//		minerLrcBalance, _ := submitter.matcher.GetAccountAvailableAmount(splitMiner.Address, lrcAddress)
//	//		//the lrcreward should be send to order.owner when miner selects MarginSplit as the selection of fee
//	//		//be careful！！！ miner will received nothing, if miner set FeeSelection=1 and he doesn't have enough lrc
//	//
//	//
//	//		if ringState.LrcLegalFee.Cmp(ringState.SplitLegalFee) < 0 && minerLrcBalance.Cmp(filledOrder.LrcFee) > 0 {
//	//			filledOrder.FeeSelection = 1
//	//			splitPer := new(big.Rat).SetInt64(int64(filledOrder.OrderState.RawOrder.MarginSplitPercentage))
//	//			legalAmountOfSaving.Mul(legalAmountOfSaving, splitPer)
//	//			filledOrder.LrcReward = legalAmountOfLrc
//	//			legalAmountOfSaving.Sub(legalAmountOfSaving, legalAmountOfLrc)
//	//			filledOrder.LegalFee = legalAmountOfSaving
//	//
//	//			minerLrcBalance.Sub(minerLrcBalance, filledOrder.LrcFee)
//	//			//log.Debugf("Miner,lrcReward:%s  legalFee:%s", lrcReward.FloatString(10), filledOrder.LegalFee.FloatString(10))
//	//		} else {
//	//			filledOrder.FeeSelection = 0
//	//			filledOrder.LegalFee = legalAmountOfLrc
//	//		}
//	//
//	//		ringState.LegalFee.Add(ringState.LegalFee, filledOrder.LegalFee)
//	//	}
//	//}
//	minerAddresses := submitter.availableSenderAddresses()
//	if !useSplit {
//		for _, normalMinerAddress := range minerAddresses {
//			minerLrcBalance, _ := submitter.matcher.GetAccountAvailableAmount(normalMinerAddress.Address, lrcAddress, spenderAddress)
//			legalFee := new(big.Rat).SetInt(big.NewInt(int64(0)))
//			feeSelections := []uint8{}
//			legalFees := []*big.Rat{}
//			lrcRewards := []*big.Rat{}
//			for _, filledOrder := range ringState.Orders {
//				lrcFee := new(big.Rat).SetInt(big.NewInt(int64(2)))
//				lrcFee.Mul(lrcFee, filledOrder.LegalLrcFee)
//				log.Debugf("lrcFee:%s, filledOrder.LegalFeeS:%s, minerLrcBalance:%s, filledOrder.LrcFee:%s", lrcFee.FloatString(3), filledOrder.LegalFeeS.FloatString(3), minerLrcBalance.FloatString(3), filledOrder.LrcFee.FloatString(3))
//				if lrcFee.Cmp(filledOrder.LegalFeeS) < 0 && minerLrcBalance.Cmp(filledOrder.LrcFee) > 0 {
//					feeSelections = append(feeSelections, 1)
//					fee := new(big.Rat).Set(filledOrder.LegalFeeS)
//					fee.Sub(fee, filledOrder.LegalLrcFee)
//					legalFees = append(legalFees, fee)
//					lrcRewards = append(lrcRewards, filledOrder.LegalLrcFee)
//					legalFee.Add(legalFee, fee)
//
//					minerLrcBalance.Sub(minerLrcBalance, filledOrder.LrcFee)
//					//log.Debugf("Miner,lrcReward:%s  legalFee:%s", lrcReward.FloatString(10), filledOrder.LegalFee.FloatString(10))
//				} else {
//					feeSelections = append(feeSelections, 0)
//					legalFees = append(legalFees, filledOrder.LegalLrcFee)
//					lrcRewards = append(lrcRewards, new(big.Rat).SetInt(big.NewInt(int64(0))))
//					legalFee.Add(legalFee, filledOrder.LegalLrcFee)
//				}
//			}
//
//			if ringState.LegalFee.Sign() == 0 || ringState.LegalFee.Cmp(legalFee) < 0 {
//				ringState.LegalFee = legalFee
//				ringSubmitInfo.Miner = normalMinerAddress.Address
//				for idx, filledOrder := range ringState.Orders {
//					filledOrder.FeeSelection = feeSelections[idx]
//					filledOrder.LegalFee = legalFees[idx]
//					filledOrder.LrcReward = lrcRewards[idx]
//				}
//
//				if nil == ringSubmitInfo.ProtocolGasPrice || ringSubmitInfo.ProtocolGasPrice.Cmp(normalMinerAddress.GasPriceLimit) > 0 {
//					ringSubmitInfo.ProtocolGasPrice = normalMinerAddress.GasPriceLimit
//				}
//			}
//		}
//	}
//	//protocolCost := new(big.Int).Mul(ringSubmitInfo.ProtocolGas, ringSubmitInfo.ProtocolGasPrice)
//	//
//	//costEth := new(big.Rat).SetInt(protocolCost)
//	//costLegal, _ := submitter.marketCapProvider.LegalCurrencyValueOfEth(costEth)
//	//ringSubmitInfo.LegalCost = costLegal
//	//received := new(big.Rat).Sub(ringState.LegalFee, costLegal)
//	//ringSubmitInfo.Received = received
//
//	return nil
//}
