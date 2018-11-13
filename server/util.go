package main

import (
	"time"
	"bytes"
	"crypto/sha256"
	"encoding/hex"

	"github.com/nyu-distributed-systems-fa18/algorand/pb"
)

func calculateHash(block *pb.Block) string {
	var transactions bytes.Buffer
	for _,tx := range block.Tx {
		transactions.WriteString(tx.V)
	}
	record := string(block.Id) + block.Timestamp + transactions.String() + block.PrevHash
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func generateBlock(oldBlock *pb.Block, tx *pb.Transaction) *pb.Block {
	newBlock := new(pb.Block)
	t := time.Now()
	transactions := []*pb.Transaction{}
	transactions = append(transactions, tx)

	newBlock.Id = 0 //hook upt to last block id + 1 once we get GenesisBlock
	newBlock.Timestamp = t.String()
	newBlock.Tx = transactions //simple list of Transactions with one Transaction for now until we decide how to aggreate multiple into one block
	newBlock.PrevHash = "" // set to last block hash
	newBlock.Hash = calculateHash(newBlock) // set to the hash of all the bytes of this block

	return newBlock
}
