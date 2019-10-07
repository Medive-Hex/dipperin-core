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
)

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

		// 生成bootnode的key
		b0Dir := filepath.Join(wDir, "b0")
		nodeKey, _ := crypto.GenerateKey()
		if err := crypto.SaveECDSA(filepath.Join(b0Dir, "boot.key"), nodeKey); err != nil {
			panic(err)
		}

		m0DataDir := filepath.Join(wDir, "m0")




	}
	app.Run(os.Args)
}
