# Loopring miner Docker

The loopring development team provides the loopring/miner image. The latest version is v1.5.0

## Run
get the latest docker image
``` 
docker pull loopring/miner
```
create log&config dir
```bash
mkdir your_log_path your_config_path your_keystore_path
```
config miner.toml，[reference](https://github.com/Loopring/relay-cluster/wiki/%E9%83%A8%E7%BD%B2miner#%E9%83%A8%E7%BD%B2%E9%85%8D%E7%BD%AE%E6%96%87%E4%BB%B6)

before deployment, perform telnet tests according to the ports related to the configuration files mysql, redis, kafka, and zk to ensure that these dependencies can be accessed normally.

mount the log,config,keystore dir, unlock your miner account and run
```bash
docker run --name extractor -idt -v your_log_path:/opt/loopring/extractor/logs -v your_config_path:/opt/loopring/extractor/config loopring/extractor:latest --config=/opt/loopring/extractor/config/extractor.toml /bin/bash
```

## History version

| version         | desc         |
|--------------|------------|
| v1.5.0| the first release version|
