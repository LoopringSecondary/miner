# Loopring miner Docker 中文文档

loopring开发团队提供loopring/miner镜像,最新版本是v1.5.0。<br>

## 部署
* 获取docker镜像
```bash
docker pull loopring/miner
```
* 创建log&config目录
```bash
mkdir your_log_path your_config_path your_keystore_path
```
* 配置miner.toml文件，[参考](https://github.com/Loopring/relay-cluster/wiki/%E9%83%A8%E7%BD%B2miner#%E9%83%A8%E7%BD%B2%E9%85%8D%E7%BD%AE%E6%96%87%E4%BB%B6)
* telnet测试mysql,redis,zk,kafka,ethereum,motan rpc相关端口能否连接

* 运行
运行时需要挂载logs&config&keystore目录, 解锁miner账户并指定config文件
```bash
docker run --name miner -idt -v your_keystore_path:/opt/loopring/miner/keystore -v your_log_path:/opt/loopring/miner/logs -v your_config_path:/opt/loopring/miner/config loopring/miner:latest --unlock your_miner_account --password your_miner_password --config=/opt/loopring/miner/config/miner.toml /bin/bash
```

## 历史版本

| 版本号         | 描述         |
|--------------|------------|
| v1.5.0| release初始版本|

