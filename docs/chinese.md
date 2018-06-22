## 简介
Miner是Loopring中非常重要的一个角色，负责从订单池中发现并选择收益最大的环路提交到合约，即完成撮合提交部分。

当前实现的是一个叫做timingmatcher的撮合引擎，执行逻辑是：
* 根据支持的市场对，定时从订单池中获取订单
* 对订单进行匹配撮合并生成环路
* 估计环路的收益，并选取有足够收益的环路
* 将选取的环路按照收益大小依次提交到以太坊

## 编译部署
* [aws部署](https://loopring.github.io/relay-cluster/deploy/deploy_index_cn.html#%E6%9C%8D%E5%8A%A1)
* [Docker](https://loopring.github.io/miner/docker_chinese.html)
* 源码
    
    ```bash
    #本项目代码使用Go编写，请确保已经完成Go环境的配置。
    git clone https://github.com/Loopring/miner.git
    cd miner
    go build -o build/bin/miner cmd/lrc/*
    #miner依赖于relay-cluster、extractor、mysql、redis、kafka、zookeeper、eth节点等
    build/bin/miner --unlocks="address1,address2" --passwords="pwd1,pwd2" --config=miner.toml
    ```

## 支持
请访问官方网站获取联系方式，获得帮助: https://loopring.org