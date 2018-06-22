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

package timing_matcher_test

import (
	"encoding/json"
	"github.com/Loopring/miner/miner/timing_matcher"
	"github.com/Loopring/relay-lib/log"
	"go.uber.org/zap"
	"testing"
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
	    "levelEncoder": "lowercase",
	    "encodeTime": "iso8601"
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

	//cache.NewCache(redis.RedisOptions{Host: "127.0.0.1", Port: "6379"})
	//
	//options := marketutil.MarketOptions{}
	//options.TokenFile = "/Users/yuhongyu/Desktop/service/go/src/github.com/Loopring/relay/config/tokens.json"
	//marketutil.Initialize(&options)
	//
	//zkconfig := zklock.ZkLockConfig{}
	//zkconfig.ZkServers = "127.0.0.1:2181"
	//zkconfig.ConnectTimeOut = 10000
	//zklock.Initialize(zkconfig)
}

func TestSingleNode(t *testing.T) {
	singleNode := timing_matcher.SingleNode{}
	singleNode
}

func TestClusterNode(t *testing.T) {

}
