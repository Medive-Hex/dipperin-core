// Copyright 2019, Keychain Foundation Ltd.
// This file is part of the dipperin-core library.
//
// The dipperin-core library is free software: you can redistribute
// it and/or modify it under the terms of the GNU Lesser General Public License
// as published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// The dipperin-core library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"github.com/dipperin/dipperin-core/cmd/utils"
	"github.com/dipperin/dipperin-core/core/chain-config"
	"os"
	"testing"
	"time"
)

func Test_main(t *testing.T) {
	dataDir := "/tmp/dipperin_test"
	os.Args = []string{"xxx", "-data_dir", dataDir}
	defer os.RemoveAll(dataDir)
	utils.SetupGenesis(dataDir, chain_config.GetChainConfig())

	go main()
	time.Sleep(500 * time.Millisecond)
}
