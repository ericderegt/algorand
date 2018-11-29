package main

import (
	"log"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"time"
	"math/rand"
	"strings"

	"github.com/nyu-distributed-systems-fa18/algorand/pb"
)

func calculateHash(block *pb.Block) string {
	var transactions bytes.Buffer
	for _, tx := range block.Tx {
		transactions.WriteString(tx.V)
	}
	record := string(block.Id) + block.Timestamp + transactions.String() + block.PrevHash
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func signMessage(message []string) string {
	s := strings.Join(message[:],"")

	h := sha256.New()
	h.Write([]byte(s))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func generateBlock(oldBlock *pb.Block, tx *pb.Transaction) *pb.Block {
	newBlock := new(pb.Block)
	t := time.Now()
	transactions := []*pb.Transaction{}
	transactions = append(transactions, tx)

	newBlock.Id = oldBlock.Id + 1
	newBlock.Timestamp = t.String()
	newBlock.Tx = transactions //simple list of Transactions with one Transaction for now until we decide how to aggreate multiple into one block
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = calculateHash(newBlock) // set to the hash of all the bytes of this block

	return newBlock
}

func makeRange(min, max int64) []int64 {
    a := make([]int64, max-min+1)
    for i := range a {
        a[i] = min + int64(i)
    }
    return a
}

func shuffleSelection(arr []string, seed int64, k int64) []string {
	// create copy of arr that will be suffled
	shuffled := make([]string, len(arr))
	copy(shuffled, arr)

	// set up array to return as selected elements
	// and random number generator
	selection := make([]string, k)
	s := rand.NewSource(seed)
	rand := rand.New(s)

	// shuffle the array
	for i := len(shuffled)-1; i >= 0; i-- {
		random_idx := rand.Intn(i + 1)
		shuffled[i], shuffled[random_idx] = shuffled[random_idx], shuffled[i]
	 }

	// select the top k elements from shuffled as the selection
	for i := range selection {
		selection[i] = shuffled[i]
	}

	return selection
}

func sortition(privateKey int64, round int64, role string, userId string, userIds []string, k int64) (string, string, int64) {
	// sortition selects k committee members out of all users
	committee := shuffleSelection(userIds, round, k)

	// print committee to verify it is the same accross all servers
	log.Printf("Committee: %#v", committee)

	// Add up how many times we were selected
	votes := int64(0)
	for _, member := range committee {
		if member == userId {
			votes++
		}
	}

	return "hash", "proof", votes
}

func verifySort(userId string, userIds []string, round int64, k int64) bool {
	committee := shuffleSelection(userIds, round, k)

	// loop through committee and verify userId is in there
	for _, member := range committee {
		if member == userId {
			return true
		}
	}
	return false
}

func SIG(i string, message []string) *pb.SIGRet {
	signedMessage := signMessage(message)
	return &pb.SIGRet{UserId: i, Message: message, SignedMessage: signedMessage}
}
