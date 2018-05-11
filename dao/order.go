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
	"errors"
	"fmt"
	"github.com/Loopring/relay-lib/crypto"
	"github.com/Loopring/relay-lib/log"
	util "github.com/Loopring/relay-lib/marketutil"
	"github.com/Loopring/relay-lib/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"time"
)

// order amountS 上限1e30

type Order struct {
	ID                    int     `gorm:"column:id;primary_key;"`
	Protocol              string  `gorm:"column:protocol;type:varchar(42)"`
	DelegateAddress       string  `gorm:"column:delegate_address;type:varchar(42)"`
	Owner                 string  `gorm:"column:owner;type:varchar(42)"`
	AuthAddress           string  `gorm:"column:auth_address;type:varchar(42)"`
	PrivateKey            string  `gorm:"column:priv_key;type:varchar(128)"`
	WalletAddress         string  `gorm:"column:wallet_address;type:varchar(42)"`
	OrderHash             string  `gorm:"column:order_hash;type:varchar(82)"`
	TokenS                string  `gorm:"column:token_s;type:varchar(42)"`
	TokenB                string  `gorm:"column:token_b;type:varchar(42)"`
	AmountS               string  `gorm:"column:amount_s;type:varchar(40)"`
	AmountB               string  `gorm:"column:amount_b;type:varchar(40)"`
	CreateTime            int64   `gorm:"column:create_time;type:bigint"`
	ValidSince            int64   `gorm:"column:valid_since;type:bigint"`
	ValidUntil            int64   `gorm:"column:valid_until;type:bigint"`
	LrcFee                string  `gorm:"column:lrc_fee;type:varchar(40)"`
	BuyNoMoreThanAmountB  bool    `gorm:"column:buy_nomore_than_amountb"`
	MarginSplitPercentage uint8   `gorm:"column:margin_split_percentage;type:tinyint(4)"`
	V                     uint8   `gorm:"column:v;type:tinyint(4)"`
	R                     string  `gorm:"column:r;type:varchar(66)"`
	S                     string  `gorm:"column:s;type:varchar(66)"`
	PowNonce              uint64  `gorm:"column:pow_nonce;type:bigint"`
	Price                 float64 `gorm:"column:price;type:decimal(28,16);"`
	UpdatedBlock          int64   `gorm:"column:updated_block;type:bigint"`
	DealtAmountS          string  `gorm:"column:dealt_amount_s;type:varchar(40)"`
	DealtAmountB          string  `gorm:"column:dealt_amount_b;type:varchar(40)"`
	CancelledAmountS      string  `gorm:"column:cancelled_amount_s;type:varchar(40)"`
	CancelledAmountB      string  `gorm:"column:cancelled_amount_b;type:varchar(40)"`
	SplitAmountS          string  `gorm:"column:split_amount_s;type:varchar(40)"`
	SplitAmountB          string  `gorm:"column:split_amount_b;type:varchar(40)"`
	Status                uint8   `gorm:"column:status;type:tinyint(4)"`
	MinerBlockMark        int64   `gorm:"column:miner_block_mark;type:bigint"`
	BroadcastTime         int     `gorm:"column:broadcast_time;type:bigint"`
	Market                string  `gorm:"column:market;type:varchar(40)"`
	Side                  string  `gorm:"column:side;type:varchar(40)"`
}

// convert types/orderState to dao/order
func (o *Order) ConvertDown(state *types.OrderState) error {
	src := state.RawOrder

	o.Price, _ = src.Price.Float64()
	o.AmountS = src.AmountS.String()
	o.AmountB = src.AmountB.String()
	o.DealtAmountS = state.DealtAmountS.String()
	o.DealtAmountB = state.DealtAmountB.String()
	o.SplitAmountS = state.SplitAmountS.String()
	o.SplitAmountB = state.SplitAmountB.String()
	o.CancelledAmountS = state.CancelledAmountS.String()
	o.CancelledAmountB = state.CancelledAmountB.String()
	o.LrcFee = src.LrcFee.String()

	o.Protocol = src.Protocol.Hex()
	o.DelegateAddress = src.DelegateAddress.Hex()
	o.Owner = src.Owner.Hex()

	auth, _ := src.AuthPrivateKey.MarshalText()
	o.PrivateKey = string(auth)
	o.AuthAddress = src.AuthAddr.Hex()
	o.WalletAddress = src.WalletAddress.Hex()

	o.OrderHash = src.Hash.Hex()
	o.TokenB = src.TokenB.Hex()
	o.TokenS = src.TokenS.Hex()
	o.CreateTime = time.Now().Unix()
	o.ValidSince = src.ValidSince.Int64()
	o.ValidUntil = src.ValidUntil.Int64()

	o.BuyNoMoreThanAmountB = src.BuyNoMoreThanAmountB
	o.MarginSplitPercentage = src.MarginSplitPercentage
	if state.UpdatedBlock != nil {
		o.UpdatedBlock = state.UpdatedBlock.Int64()
	}
	o.Status = uint8(state.Status)
	o.V = src.V
	o.S = src.S.Hex()
	o.R = src.R.Hex()
	o.PowNonce = src.PowNonce
	o.BroadcastTime = state.BroadcastTime
	o.Side = state.RawOrder.Side

	return nil
}

// convert dao/order to types/orderState
func (o *Order) ConvertUp(state *types.OrderState) error {
	state.RawOrder.AmountS, _ = new(big.Int).SetString(o.AmountS, 0)
	state.RawOrder.AmountB, _ = new(big.Int).SetString(o.AmountB, 0)
	state.DealtAmountS, _ = new(big.Int).SetString(o.DealtAmountS, 0)
	state.DealtAmountB, _ = new(big.Int).SetString(o.DealtAmountB, 0)
	state.SplitAmountS, _ = new(big.Int).SetString(o.SplitAmountS, 0)
	state.SplitAmountB, _ = new(big.Int).SetString(o.SplitAmountB, 0)
	state.CancelledAmountS, _ = new(big.Int).SetString(o.CancelledAmountS, 0)
	state.CancelledAmountB, _ = new(big.Int).SetString(o.CancelledAmountB, 0)
	state.RawOrder.LrcFee, _ = new(big.Int).SetString(o.LrcFee, 0)

	state.RawOrder.Price = new(big.Rat).SetFloat64(o.Price)
	state.RawOrder.Protocol = common.HexToAddress(o.Protocol)
	state.RawOrder.DelegateAddress = common.HexToAddress(o.DelegateAddress)
	state.RawOrder.TokenS = common.HexToAddress(o.TokenS)
	state.RawOrder.TokenB = common.HexToAddress(o.TokenB)
	state.RawOrder.ValidSince = big.NewInt(o.ValidSince)
	state.RawOrder.ValidUntil = big.NewInt(o.ValidUntil)

	if len(o.AuthAddress) > 0 {
		state.RawOrder.AuthAddr = common.HexToAddress(o.AuthAddress)
	}
	state.RawOrder.AuthPrivateKey, _ = crypto.NewPrivateKeyCrypto(false, o.PrivateKey)
	state.RawOrder.WalletAddress = common.HexToAddress(o.WalletAddress)

	state.RawOrder.BuyNoMoreThanAmountB = o.BuyNoMoreThanAmountB
	state.RawOrder.MarginSplitPercentage = o.MarginSplitPercentage
	state.RawOrder.V = o.V
	state.RawOrder.S = types.HexToBytes32(o.S)
	state.RawOrder.R = types.HexToBytes32(o.R)
	state.RawOrder.PowNonce = o.PowNonce
	state.RawOrder.Owner = common.HexToAddress(o.Owner)
	state.RawOrder.Hash = common.HexToHash(o.OrderHash)

	if state.RawOrder.Hash != state.RawOrder.GenerateHash() {
		log.Debug("different order hash found......")
		log.Debug(state.RawOrder.Hash.Hex())
		log.Debug(state.RawOrder.GenerateHash().Hex())
		return fmt.Errorf("dao order convert down generate hash error")
	}

	state.UpdatedBlock = big.NewInt(o.UpdatedBlock)
	state.Status = types.OrderStatus(o.Status)
	state.BroadcastTime = o.BroadcastTime
	state.RawOrder.Market = o.Market
	state.RawOrder.CreateTime = o.CreateTime
	state.RawOrder.Side = o.Side

	state.RawOrder.Side = util.GetSide(o.TokenS, o.TokenB)
	return nil
}

func (s *RdsServiceImpl) MarkMinerOrders(filterOrderhashs []string, blockNumber int64) error {
	if len(filterOrderhashs) == 0 {
		return nil
	}

	err := s.db.Model(&Order{}).
		Where("order_hash in (?)", filterOrderhashs).
		Update("miner_block_mark", blockNumber).Error

	return err
}

func (s *RdsServiceImpl) GetOrdersForMiner(protocol, tokenS, tokenB string, length int, filterStatus []types.OrderStatus, reservedTime, startBlockNumber, endBlockNumber int64) ([]*Order, error) {
	var (
		list []*Order
		err  error
	)

	if len(filterStatus) < 1 {
		return list, errors.New("should filter cutoff and finished orders")
	}

	nowtime := time.Now().Unix()
	sinceTime := nowtime
	untilTime := nowtime + reservedTime
	err = s.db.Where("delegate_address = ? and token_s = ? and token_b = ?", protocol, tokenS, tokenB).
		Where("valid_since < ?", sinceTime).
		Where("valid_until >= ? ", untilTime).
		Where("status not in (?) ", filterStatus).
		Where("miner_block_mark between ? and ?", startBlockNumber, endBlockNumber).
		Order("price desc").
		Limit(length).
		Find(&list).
		Error

	return list, err
}

func (s *RdsServiceImpl) GetOrderByHash(orderhash common.Hash) (*Order, error) {
	order := &Order{}
	err := s.db.Where("order_hash = ?", orderhash.Hex()).First(order).Error
	return order, err
}

func (s *RdsServiceImpl) GetOrdersByHash(orderhashs []string) (map[string]Order, error) {
	var (
		list []Order
		err  error
	)

	ret := make(map[string]Order)
	if err = s.db.Where("order_hash in (?)", orderhashs).Find(&list).Error; err != nil {
		return ret, err
	}

	for _, v := range list {
		ret[v.OrderHash] = v
	}

	return ret, err
}
