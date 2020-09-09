package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	ErrTxMetadataNotFound = errors.New("tx metadata not found")
	ErrTxNotFound         = errors.New("tx not found")
)

type Dump struct {
	TxHash            trinary.Trytes
	TrunkHash         trinary.Trytes
	BranchHash        trinary.Trytes
	BundleHash        trinary.Trytes
	confirmationIndex milestone.Index
	IsSolid           bool
	IsConfirmed       bool
	IsConflicting     bool
	IsHead            bool
	IsTail            bool
	IsValue           bool
	Trytes            trinary.Trytes
}

func main() {
	config.NodeConfig.Set(profile.CfgUseProfile, "auto")

	cfgDbPath := flag.String("dbPath", "", "directory that contains the tangle.db")
	cfgOutputFile := flag.String("output", "output.txt", "output file to store the dump")
	flag.Parse()
	dbPath := *cfgDbPath
	outputFile := *cfgOutputFile

	if dbPath == "" {
		log.Fatal("dbPath is required")
	}

	f, err := os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf(fmt.Errorf("cannot open file: %w", err).Error())
	}
	defer f.Close()

	totalCount, successCount := 0, 0
	tangle.ConfigureDatabases(dbPath)
	log.Println(tangle.GetTransactionStorageSize())
	tangle.ForEachTransactionMetadataHash(func(txHash hornet.Hash) bool {
		totalCount++
		cachedTx := tangle.GetCachedTransactionOrNil(txHash)
		if cachedTx == nil {
			log.Println(fmt.Errorf("tx %s not found: %w", txHash.Trytes(), ErrTxNotFound).Error())
			return true
		}
		defer cachedTx.Release()
		trytes, err := transaction.TransactionToTrytes(cachedTx.GetTransaction().Tx)
		if err != nil {
			log.Println(fmt.Errorf("cannot convert transaction to trytes: %w", err).Error())
			return true
		}
		cachedTxMetadata := tangle.GetCachedTxMetadataOrNil(txHash)
		if cachedTxMetadata == nil {
			log.Println(fmt.Errorf("tx metadata %s not found: %w", txHash.Trytes(), ErrTxMetadataNotFound).Error())
			return true
		}
		defer cachedTxMetadata.Release()
		txMetadata := cachedTxMetadata.GetMetadata()
		isConfirmed, confirmationIndex := txMetadata.GetConfirmed()
		dump := Dump{
			TxHash:            txMetadata.GetTxHash().Trytes(),
			TrunkHash:         txMetadata.GetTrunkHash().Trytes(),
			BranchHash:        txMetadata.GetBranchHash().Trytes(),
			BundleHash:        txMetadata.GetBranchHash().Trytes(),
			confirmationIndex: confirmationIndex,
			IsSolid:           txMetadata.IsSolid(),
			IsConfirmed:       isConfirmed,
			IsConflicting:     txMetadata.IsConflicting(),
			IsHead:            txMetadata.IsHead(),
			IsTail:            txMetadata.IsTail(),
			IsValue:           txMetadata.IsValue(),
			Trytes:            trytes,
		}

		if writeToFile(f, dump) {
			successCount++
		}
		return true
	}, true)
	log.Println("Total txs: ", totalCount)
	log.Println("Success: ", successCount)
}

func writeToFile(f *os.File, dump Dump) bool {
	bytes, err := json.MarshalIndent(dump, "", "    ")
	if err != nil {
		log.Println(fmt.Errorf("err marshalling dump: %w", err).Error())
		return false
	}
	jsonStr := string(bytes)

	if _, err := f.WriteString(fmt.Sprintf("%s\n", jsonStr)); err != nil {
		log.Println(fmt.Errorf("err writing to file: %w", err).Error())
		return false
	}
	log.Println(dump.TxHash, " done...")
	return true
}
