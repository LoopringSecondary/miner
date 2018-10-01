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

package config

import (
	"errors"
	"os"
	"reflect"

	"github.com/Loopring/relay-cluster/accountmanager"
	"github.com/Loopring/relay-cluster/ordermanager"
	"github.com/Loopring/relay-lib/cache/redis"
	"github.com/Loopring/relay-lib/cloudwatch"
	"github.com/Loopring/relay-lib/dao"
	"github.com/Loopring/relay-lib/eth/accessor"
	"github.com/Loopring/relay-lib/eth/loopringaccessor"
	"github.com/Loopring/relay-lib/kafka"
	"github.com/Loopring/relay-lib/marketcap"
	"github.com/Loopring/relay-lib/marketutil"
	"github.com/Loopring/relay-lib/motan"
	"github.com/Loopring/relay-lib/sns"
	"github.com/Loopring/relay-lib/zklock"
	"github.com/naoina/toml"
	"go.uber.org/zap"
)

func LoadConfig(file string) *GlobalConfig {
	if "" == file {
		dir, _ := os.Getwd()
		file = dir + "/config/miner.toml"
	}

	io, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	defer io.Close()

	c := &GlobalConfig{}
	if err := toml.NewDecoder(io).Decode(c); err != nil {
		panic(err)
	}

	//if c.Common.Develop {
	//	basedir := strings.TrimSuffix(os.Getenv("GOPATH"), "/") + "/src/github.com/Loopring/relay/"
	//	c.Keystore.Keydir = basedir + c.Keystore.Keydir
	//
	//	for idx, path := range c.Log.ZapOpts.OutputPaths {
	//		if !strings.HasPrefix(path, "std") {
	//			c.Log.ZapOpts.OutputPaths[idx] = basedir + path
	//		}
	//	}
	//}

	// extractor.IsDevNet default false

	return c
}

type GlobalConfig struct {
	Title            string `required:"true"`
	Mysql            *dao.MysqlOptions
	Redis            redis.RedisOptions
	Accessor         accessor.AccessorOptions
	LoopringAccessor loopringaccessor.LoopringProtocolOptions
	Miner            MinerOptions
	Log              zap.Config
	Keystore         KeyStoreOptions
	MarketCap        marketcap.MarketCapOptions
	MarketUtil       marketutil.MarketOptions
	ZkLock           zklock.ZkLockConfig
	DataSource       DataSource
	Kafka            kafka.KafkaOptions
	Sns              sns.SnsConfig
	CloudWatch       cloudwatch.CloudWatchConfig
}

type DataSource struct {
	Type           string
	OrderManager   ordermanager.OrderManagerOptions
	AccountManager accountmanager.AccountManagerOptions
	MotanClient    motan.MotanClientOptions
}

type KeyStoreOptions struct {
	Keydir  string
	ScryptN int
	ScryptP int
}

type TimingMatcher struct {
	Mode                         string
	RoundOrdersCount             int
	Duration                     int64
	ReservedSubmitTime           int64
	MaxSumitFailedCount          int64
	DelayedNumber                int64
	MaxCacheTime                 int64
	LagForCleanSubmitCacheBlocks int64
}

type PercentMinerAddress struct {
	Address    string
	FeePercent float64 //the gasprice will be calculated by (FeePercent/100)*(legalFee/eth-price)/gaslimit
	StartFee   float64 //If received reaches StartReceived, it will use feepercent to ensure eth confirm this tx quickly.
}

type NormalMinerAddress struct {
	Address         string
	MaxPendingTtl   int   //if a tx is still pending after MaxPendingTtl blocks, the nonce used by it will be used again.
	MaxPendingCount int64 //this addr will be used to send tx again until the count of pending txs belows MaxPendingCount.
	GasPriceLimit   int64 //the max gas price
}

type MinerOptions struct {
	RingMaxLength         int `` //recommended value:4
	Subsidy               float64
	WalletSplit           float64
	NormalMiners          []NormalMinerAddress  //
	PercentMiners         []PercentMinerAddress //
	TimingMatcher         *TimingMatcher
	RateRatioCVSThreshold int64
	MinGasLimit           int64
	MaxGasLimit           int64
	FeeReceipt            string
	GasPriceRate	float64
}

func Validator(cv reflect.Value) (bool, error) {
	for i := 0; i < cv.NumField(); i++ {
		cvt := cv.Type().Field(i)

		if cv.Field(i).Type().Kind() == reflect.Struct {
			if res, err := Validator(cv.Field(i)); nil != err {
				return res, err
			}
		} else {
			if "true" == cvt.Tag.Get("required") {
				if !isSet(cv.Field(i)) {
					return false, errors.New("The field " + cvt.Name + " in config must be setted")
				}
			}
		}
	}

	return true, nil
}

func isSet(v reflect.Value) bool {
	switch v.Type().Kind() {
	case reflect.Invalid:
		return false
	case reflect.String:
		return v.String() != ""
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() != 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() != 0
	case reflect.Map:
		return len(v.MapKeys()) != 0
	}
	return true
}
