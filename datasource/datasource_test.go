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

package datasource_test

import (
	"testing"
	"github.com/Loopring/miner/datasource"
	"github.com/Loopring/miner/config"
	"github.com/Loopring/relay-lib/motan"
	"github.com/Loopring/relay-lib/marketcap"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"
	"encoding/json"
	"github.com/Loopring/relay-lib/log"
)


func init() {
	logConfig := `{
	  "level": "debug",
	  "development": false,
	  "encoding": "json",
	  "outputPaths": ["stdout"],
	  "errorOutputPaths": ["stderr"],
	  "encoderConfig": {
	    "messageKey": "message",
	    "levelKey": "level",
	    "levelEncoder": "lowercase"
	  }
	}`
	rawJSON := []byte(logConfig)

	var (
		cfg zap.Config
		err error
	)
	if err = json.Unmarshal(rawJSON, &cfg); err != nil {
		panic(err)
	}
	log.Initialize(cfg)
	options := config.DataSource{}
	options.Type = "MOTAN"
	options.MotanClient = motan.MotanClientOptions{}
	options.MotanClient.ClientId = "miner-client"
	options.MotanClient.ConfFile = "../config/motan_client.yaml"
	datasource.Initialize(options, nil, &marketcap.CapProvider_CoinMarketCap{})
}

func TestGetBalanceAndAllowance(t *testing.T) {
	if balance,allowance, err := datasource.GetBalanceAndAllowance(common.HexToAddress("0x750aD4351bB728ceC7d639A9511F9D6488f1E259"), common.HexToAddress("0xe1C541BA900cbf212Bc830a5aaF88aB499931751"), common.HexToAddress("0xa0aF16eDD397d9e826295df9E564b10D57E3C457")); nil != err {
		t.Fatalf("err:%s", err.Error())
	} else {
		t.Logf("balance:%s, allowance:%s", balance.String(), allowance.String())
	}


}

func TestMinerOrders(t *testing.T) {
	delegateAddress := common.HexToAddress("0xa0aF16eDD397d9e826295df9E564b10D57E3C457")
	tokenS := common.HexToAddress("0xe1C541BA900cbf212Bc830a5aaF88aB499931751")
	tokenB := common.HexToAddress("0x639687b7f8501f174356D3aCb1972f749021CCD0")
	length := 10
	reservedTime := int64(1000)
	startNumber := int64(0)
	endNumber := int64(1000000000000)
	orders := datasource.MinerOrders(delegateAddress, tokenS, tokenB, length, reservedTime, startNumber, endNumber)
	for _,o := range orders {
		t.Logf("###, hash:%s", o.RawOrder.Hash.Hex())
	}
}
