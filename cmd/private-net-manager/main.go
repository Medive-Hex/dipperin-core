package main

import (
	"gopkg.in/urfave/cli.v1"
	"os"
	"github.com/dipperin/dipperin-core/third-party/log"
	"github.com/dipperin/dipperin-core/common"
	"github.com/dipperin/dipperin-core/core/accounts/soft-wallet"
	"github.com/dipperin/dipperin-core/core/dipperin"
	"strings"
	"fmt"
	"path/filepath"
	"github.com/dipperin/dipperin-core/common/util"
	"github.com/dipperin/dipperin-core/third-party/crypto"
	"net"
	"strconv"
	"github.com/dipperin/dipperin-core/third-party/p2p/enode"
	"io/ioutil"
	"path"
)

type genesisCfgFile struct {
	Nonce uint64 `json:"nonce"`
	//Note       string           `json:"note"`
	Accounts   map[string]int64 `json:"accounts"`
	Timestamp  string           `json:"timestamp"`
	Difficulty string           `json:"difficulty" gencodec:"required"`
	Verifiers  []string         `json:"verifiers" gencodec:"required"`
	// todo add a foundation configuration
}

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "work_dir", Usage: "work dir of local net, put all config and data here"},
		cli.BoolFlag{Name: "f", Usage: "force create, will auto remove some old data"},
	}
	app.Action = func(c *cli.Context) {
		wDir := c.String("work_dir")
		walletPwd := "123"
		genesisConfPath := filepath.Join(util.HomeDir(), "softwares", "dipperin_deploy", "genesis.json")
		if common.FileExist(genesisConfPath) {
			if c.Bool("f") {
				if err := os.Remove(genesisConfPath); err != nil {
					log.Info("remove old genesis config failed", "err", err)
					return
				}
			} else {
				log.Info("genesis config already exist", "genesisConfPath", genesisConfPath)
				return
			}
		}
		if wDir == "" {
			log.Info("work_dir can't be empty")
			return
		}
		if common.FileExist(wDir) {
			log.Info("work_dir should not exist, i'll auto create it")
			return
		}
		if err := os.MkdirAll(wDir, 0755); err != nil {
			log.Info("create work_dir failed", "err", err)
			return
		}

		var vAddresses []string
		var vDataDirs []string
		for i := 0; i < 4; i++ {
			vDataDir := filepath.Join(wDir, fmt.Sprintf("v%v", i))
			if err := os.MkdirAll(vDataDir, 0755); err != nil {
				panic(err)
			}
			vDataDirs = append(vDataDirs, vDataDir)
			// 初始化验证者钱包
			// load wallet manager
			defaultWallet, wErr := soft_wallet.NewSoftWallet()
			if wErr != nil {
				panic("new soft wallet failed: " + wErr.Error())
			}
			nodeConfig := &dipperin.NodeConfig{
				DataDir: vDataDir,
				SoftWalletPassword: walletPwd,
			}
			var mnemonic string
			var err error
			exit, _ := soft_wallet.PathExists(nodeConfig.SoftWalletFile())
			if exit {
				err = defaultWallet.Open(nodeConfig.SoftWalletFile(), nodeConfig.SoftWalletName(), nodeConfig.SoftWalletPassword)
			} else {
				mnemonic, err = defaultWallet.Establish(nodeConfig.SoftWalletFile(), nodeConfig.SoftWalletName(), nodeConfig.SoftWalletPassword, nodeConfig.SoftWalletPassPhrase)
				mnemonic = strings.Replace(mnemonic, " ", ",", -1)
			}

			if err != nil {
				log.Info("init wallet failed", "err", err)
				os.Exit(1)
			}

			if defaultAccounts, err := defaultWallet.Accounts(); err != nil {
				log.Info("get default accounts failed: ", "err", err)
				os.Exit(1)
			} else {
				vAddresses = append(vAddresses, defaultAccounts[0].Address.Hex())
			}
		}

		// 写入 genesis.json
		ge := genesisCfgFile{
			Nonce: 11,
			Accounts: map[string]int64{},
			Timestamp: "1548554091989871000",
			Difficulty: "0x1e566611",
			Verifiers: vAddresses,
		}
		// 创建文件夹
		if err := os.MkdirAll(path.Dir(genesisConfPath), 0755); err != nil {
			panic(err)
		}
		if err := ioutil.WriteFile(genesisConfPath, util.StringifyJsonToBytes(ge), 0644); err != nil {
			panic(err)
		}

		// 生成bootnode的key
		b0Dir := filepath.Join(wDir, "b0")
		if err := os.MkdirAll(b0Dir, 0755); err != nil {
			panic(err)
		}
		nodeKey, _ := crypto.GenerateKey()
		b0keyfile := filepath.Join(b0Dir, "boot.key")
		if err := crypto.SaveECDSA(b0keyfile, nodeKey); err != nil {
			panic(err)
		}

		m0DataDir := filepath.Join(wDir, "m0")
		if err := os.MkdirAll(m0DataDir, 0755); err != nil {
			panic(err)
		}

		// 放入 bootnode key
		udpPort, _ := strconv.ParseInt((":30301")[1:], 10, 64)
		n := enode.NewV4(&nodeKey.PublicKey, net.ParseIP("127.0.0.1"), int(udpPort), int(udpPort))
		bnconfB := util.StringifyJsonToBytes([]string{n.String()})

		staticBFileName := "static_boot_nodes.json"
		if err := ioutil.WriteFile(filepath.Join(m0DataDir, staticBFileName), bnconfB, 0644); err != nil {
			panic(err)
		}
		for _, vd := range vDataDirs {
			if err := ioutil.WriteFile(filepath.Join(vd, staticBFileName), bnconfB, 0644); err != nil {
				panic(err)
			}
		}

		// 创建启动脚本，bootnode、verifiers、minermaster
		startsh := fmt.Sprintf(`
export boots_env=local

nohup bootnode --nodekey %v >> %v & 

nohup dipperin --node_type 2 --soft_wallet_pwd 123 --data_dir %v --http_port 10001 --ws_port 10002 --p2p_listener 20001 >> %v &
nohup dipperin --node_type 2 --soft_wallet_pwd 123 --data_dir %v --http_port 10003 --ws_port 10004 --p2p_listener 20002 >> %v &
nohup dipperin --node_type 2 --soft_wallet_pwd 123 --data_dir %v --http_port 10005 --ws_port 10006 --p2p_listener 20003 >> %v &
nohup dipperin --node_type 2 --soft_wallet_pwd 123 --data_dir %v --http_port 10007 --ws_port 10008 --p2p_listener 20004 >> %v &

nohup dipperin --node_type 1 --soft_wallet_pwd 123 --data_dir %v --is_start_mine 1 --http_port 10010 --ws_port 10011 --p2p_listener 20010 >> %v &
`, b0keyfile, filepath.Join(b0Dir, "out.log"),
			vDataDirs[0], filepath.Join(vDataDirs[0], "out.log"),
			vDataDirs[1], filepath.Join(vDataDirs[1], "out.log"),
			vDataDirs[2], filepath.Join(vDataDirs[2], "out.log"),
			vDataDirs[3], filepath.Join(vDataDirs[3], "out.log"),
				m0DataDir, filepath.Join(m0DataDir, "out.log"))

		if err := ioutil.WriteFile(filepath.Join(wDir, "start_nodes.sh"), []byte(startsh), 0744); err != nil {
			panic(err)
		}

		stopsh := `
ps aux|grep bootnode|awk '{print $2}'|xargs kill -9
ps aux|grep dipperin|awk '{print $2}'|xargs kill -9
`
		if err := ioutil.WriteFile(filepath.Join(wDir, "stop_nodes.sh"), []byte(stopsh), 0744); err != nil {
			panic(err)
		}
	}
	app.Run(os.Args)
}
