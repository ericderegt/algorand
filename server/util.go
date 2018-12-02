package main

import (
	"log"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
	"math/rand"
	"strings"
	"strconv"

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

func prepareBlock(block *pb.Block, blockchain []*pb.Block) pb.Block {
	if len(blockchain) > 0 {
		lastBlock := blockchain[len(blockchain)-1]
		block.Id = lastBlock.Id + 1
		block.PrevHash = lastBlock.Hash
	} else {
		block.Id = 1
		block.PrevHash = ""
	}
	block.Timestamp = time.Now().String()
	block.Hash = calculateHash(block)

	return *block
}

func makeRange(min, max int64) []int64 {
    a := make([]int64, max-min+1)
    for i := range a {
        a[i] = min + int64(i)
    }
    return a
}

func initStake(userIds []string, min, max int) map[string]int {
	idToStake := make(map[string]int)

	for _, id := range userIds {
		seed,_ := strconv.ParseInt(id, 10, 64)
		s := rand.NewSource(seed)
		rand := rand.New(s)
		stake := min + rand.Intn(max - min)

		idToStake[id] = stake
	}
	return idToStake
}

func generateCandidatesByStake(userIds []string, idToStake map[string]int) []string {
	// add up the total stake and create new array with that many slots
	totalStake := 0
	for _, v := range idToStake {
		totalStake += v
	}
	candidates := make([]string, totalStake)

	// add user to candidates as many times as they have stake
	i := 0
	for _,member := range userIds {
		for j := 0; j < idToStake[member]; j++ {
			candidates[i] = member
			i++
		}
	}

	return candidates
}

func committeeSelection(arr []string, seed int64, k int64) []string {
	// set up array to return as selected elements
	// and random number generator
	selection := make([]string, k)
	s := rand.NewSource(seed)
	rand := rand.New(s)

	i := int64(0)
	for i = 0; i < k; i++ {
		random_idx := rand.Intn(len(arr))
		selection[i] = arr[random_idx]
	}

	return selection
}

func sortition(privateKey int64, round int64, role string, userId string, candidates []string, k int64) (string, string, int64) {
	// sortition selects k committee members out of all users
	committee := committeeSelection(candidates, round, k)

	// print committee to verify it is the same accross all servers
	log.Printf("Round %v Committee: %#v", round, committee)

	// Add up how many times we were selected
	votes := int64(0)
	for _, member := range committee {
		if member == userId {
			votes++
		}
	}

	return "hash", "proof", votes
}

func verifySort(userId string, candidates []string, round int64, k int64) bool {
	committee := committeeSelection(candidates, round, k)

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

func selectLeader(proposedValues map[string]string) string {
	minCredential := ""
	value := ""

	for k, v := range proposedValues {
		if minCredential == "" {
			minCredential = k
			value = v
		} else if k < minCredential {
			minCredential = k
			value = v
		}
	}

	return value
}

func PrettyPrint(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
			return string(b)
	}
	return ""
}
