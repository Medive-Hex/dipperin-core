
# 一键搭本地私网

## 网络角色

1 bootnode
4 verifiers
1 miner master

boots_env 用 local 则可以只需4验证者节点

1. 初始化 genesis.json 
    生成验证者的钱包
    设置验证者列表
1. 生成 bootnode ，并配置其conn到各节点
1. 启动 bootnode、verifiers、minermaster