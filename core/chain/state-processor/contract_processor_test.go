package state_processor

import (
	"encoding/json"
	"github.com/dipperin/dipperin-core/common"
	"github.com/dipperin/dipperin-core/core/model"
	"github.com/dipperin/dipperin-core/third-party/log"
	"github.com/ethereum/go-ethereum/ethdb"
	"testing"
)

func TestAccountStateDB_ProcessContract(t *testing.T) {
	log.InitLogger(log.LvlDebug)
	transactionJson := "{\"TxData\":{\"nonce\":\"0x5\",\"to\":\"0x00000000000000000000000000000000000000000018\",\"hashlock\":null,\"timelock\":\"0x0\",\"value\":\"0x2540be400\",\"fee\":\"0x424ad340\",\"extradata\":\"0x7b22636f6465223a224147467a625145414141414244514e674158384159414a2f6677426741414143485149445a573532426e427961573530637741414132567564676877636d6c7564484e666241414241775144416749414241554263414542415155444151414342685544667746426b49674543333841515a43494241742f41454747434173484e4155476257567462334a354167414c5831396f5a57467758324a68633255444151706658325268644746665a57356b4177494561573570644141444257686c624778764141514b52514d4341417343414173394151462f4977424245477369415351415159414945414167415545674f674150494146424432704241524142494141514143414251516f36414134674155454f616b45424541456741554551616951414377734e415142426741674c426d686c6247787641413d3d222c22616269223a2257776f674943416765776f67494341674943416749434a755957316c496a6f67496d6c75615851694c416f67494341674943416749434a70626e423164484d694f6942625853774b494341674943416749434169623356306348563063794936494674644c416f67494341674943416749434a6a6232357a644746756443493649434a6d5957787a5a534973436941674943416749434167496e5235634755694f6941695a6e5675593352706232346943694167494342394c416f674943416765776f67494341674943416749434a755957316c496a6f67496d686c624778764969774b494341674943416749434169615735776458527a496a6f6757776f6749434167494341674943416749434237436941674943416749434167494341674943416749434169626d46745a53493649434a755957316c4969774b494341674943416749434167494341674943416749434a306558426c496a6f67496e4e30636d6c755a79494b4943416749434167494341674943416766516f67494341674943416749463073436941674943416749434167496d39316448423164484d694f6942625853774b4943416749434167494341695932397563335268626e51694f69416964484a315a534973436941674943416749434167496e5235634755694f6941695a6e567559335270623234694369416749434239436c303d222c22496e707574223a6e756c6c7d\"},\"Wit\":{\"r\":\"0x911be4b1426a306aa67e96c69edec4706f9b970d776c73a451e33ab50b7eb60a\",\"s\":\"0x797761d0a881cd8a08ba746dc949bb583e65153575941b093d4510422133e908\",\"v\":\"0x38\",\"hashkey\":\"0x\"}}"
	var tx model.Transaction
	err := json.Unmarshal([]byte(transactionJson), &tx)
	if err != nil {
		log.Info("TestAccountStateDB_ProcessContract", "err", err)
	}
	log.Info("processContract", "tx", tx)

	block := model.Block{}



	db := ethdb.NewMemDatabase()
	sdb := NewStateStorageWithCache(db)
	processor, _ := NewAccountStateDB(common.Hash{}, sdb)
	processor.ProcessContract(&tx, &block, true)

}