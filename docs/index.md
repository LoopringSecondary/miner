# Miner

## 简介
Miner是Loopring中非常重要的一个角色，他负责从订单池中发现并选择收益最大的环路提交到合约，即完成撮合部分。

从订单池中获取订单、匹配订单、估计收益、提交交易


## 如何编译

获取源代码

```
git clone 
```
请确保已经配置Go环境，<>

```
cd miner
go build -o build/bin/miner cmd/lrc/*
```


## 如何运行

该模块依赖于excactor以及relay-cluster，以及相应的服务如mysql、redis、kafka等，请先确保这些服务的正常。

然后根据配置文件样例设置对应的配置文件

```
build/bin/miner --unlocks="address1,address2" --passwords="pwd1,pwd2" --config=miner.toml
```


## 配置
可以通过 `--help` 查看启动参数
```
build/bin/miner --help
```

* --config, -c ： 指定配置文件
* --unlocks ： 需要解锁的账户
* --passwords ： `--unlocks`对应的密码，顺序需要相同

配置文件可以参考样例 `config/miner.toml`

重要参数说明：

| 参数 | 用途 | 默认值 |
|:--|:--:|--:|
| keystore | 保存需要keystrore文件的目录<br> `--unlocks`中指定的地址必须在此文件夹中| 无 | 
| miner.feeReceipt | 收益地址<br> 撮合所得的收益会发送到该地址<br> 注意：该地址不需要解锁| 无 |
| miner.subsidy | 补贴 | 无 |
| miner.walletSplit | 钱包分润后剩余的撮合收益的比例 | 0.8 |
| miner.maxGasLimit | 提交以太坊交易时，允许的最大gasprice | 无 |
| miner.minGasLimit | 最小的gasprice | 无 |
| miner.normal_miners.address | 提交撮合交易到合约的地址<br> 注意：该地址需要解锁，并且有Eth余额足够发送以太坊交易 | 无 |
| miner.TimingMatcher.mode | 获取订单的方式<br>可选值：motan 通过motan-rpc获取订单| motan |
| miner.TimingMatcher.round_orders_count| 每轮获取订单的数量<br>越大耗费资源越多，但是匹配量越大| 无 |
| miner.TimingMatcher.duration | 每轮的周期，单位ms | 无 |
| miner.TimingMatcher.reserved_submit_time | 只选取过期前n秒的订单 | 无 |
| miner.TimingMatcher.max_sumit_failed_count | 每个订单提交执行失败的次数，达到该次数后，订单不再进行撮合 | 无 |
| miner.TimingMatcher.max_cache_time | 撮合结果的最大缓存时间，单位：秒 | 无 |
| miner.TimingMatcher.lag_for_clean_submit_cache_blocks | 延迟清空缓存的块数，当交易执行成功后，缓存并不会立即清空，而是等待至该块号后清空| 无 |
| miner.TimingMatcher.delayed_number | 订单当轮没有被撮合的话，延迟n毫秒再次进行撮合  | 无 |
| market_cap.currency | 法币类型，可选值参考<https://coinmarketcap.com/api/#endpoint_ticker> | 无 |
| market_cap.dust_value | 灰尘金额，单位为 `market_cap.currency` | 无 |
| loopring_accessor.address | 使用的合约地址，撮合交易等会提交到该地址 | 无 |
| loopring_accessor.*Abi | `loopring_accessor.implAbi` `loopring_accessor.delegateAbi` `loopring_accessor.tokenRegistryAbi` 为对应的合约地址：`loopring_accessor.address`的abi字符串 | 不设置时，默认从`etherscan.io`中拉取对应的abi | 

