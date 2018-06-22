
## Introduction
The Miner has a very important role in Loopring. It is responsible for discovering and selecting the loop with the highest return from the order pool and submitting it to the contract.

The current implementation is a blending engine called timingmatcher. The execution logic is:
* Get orders from the order pool on a regular basis according to the supported market pair
* Match orders and merge loops
* Estimate loop revenue and select loops with sufficient revenue
* The selected loop is submitted to Ethereum in the order of amount of profit

## Compile and deploy
* [Aws deployment](https://loopring.github.io/relay-cluster/deploy/deploy_index.html#%E6%9C%8D%E5%8A%A1)
* [Docker]((https://loopring.github.io/miner/docker.html))
* Source code
    
    ```
    #This project is written in the coding language Go. Make sure you have already configured Go.
    git clone https://github.com/Loopring/miner.git
    cd miner
    go build -o build/bin/miner cmd/lrc/*
    #The miner depends on therelay-cluster、extractor、mysql、redis、kafka、zookeeper、eth nodes, etc.
    build/bin/miner --unlocks="address1,address2" --passwords="pwd1,pwd2" --config=miner.toml
    ```
    
    
## Documents in Other Languages
- [中文文档（Chinese）](chinese.md)

## Extra Info and Help
Please visit the official website for contact information and help: https://loopring.org



