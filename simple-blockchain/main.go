package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
	"sync"
	"fmt"
	"math/rand"

	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
)

type Message struct {
	Receiver string
	Payer string
	Amount int
}

type Block struct {
	Index     int
	Timestamp string
	Receiver  string
	Payer     string
	Amount    int
	Hash      string
	PrevHash  string
	Validator string
}

// Global functions
var Blockchain []Block
var tempBlocks []Block
var candidateBlocks = make(chan Block)
var announcements = make(chan string)

var mutex = &sync.Mutex{}

var validators = make(map[string]int)

// Blockchain Code
func calculateHash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func calculateBlockHash(block Block) string {
	record := string(block.Index) + block.Timestamp + block.Receiver + block.Payer + string(block.Amount) + block.PrevHash
	return calculateHash(record)
}

func generateBlock(oldBlock Block, Receiver string, Payer string, Amount int, address string) (Block, error) {

	var newBlock Block

	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.Receiver = Receiver
	newBlock.Payer = Payer
	newBlock.Amount = Amount
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = calculateBlockHash(newBlock)
	newBlock.Validator = address

	return newBlock, nil
}

func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index || oldBlock.Hash != newBlock.PrevHash || calculateBlockHash(newBlock) != newBlock.Hash {
		return false
	}
	return true
}

func replaceChain(newBlocks []Block) {
	if len(newBlocks) > len(Blockchain) {
		Blockchain = newBlocks
	}
}

// Server code
func handleGetBlockchain(w http.ResponseWriter, r *http.Request) {
	bytes, err := json.MarshalIndent(Blockchain, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, string(bytes))
}

func respondWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
	response, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("HTTP 500: Internal Server Error"))
		return
	}
	w.WriteHeader(code)
	w.Write(response)
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	go func() {
		for {
			msg := <-announcements
			io.WriteString(conn, msg)
		}
	}()
	var address string //validator

	// users input their init stake
	io.WriteString(conn, "Enter token balance:")
	scanBalance := bufio.NewScanner(conn)
	for scanBalance.Scan() {
		balance, err := strconv.Atoi(scanBalance.Text())
		if err != nil {
			log.Printf("%v not a number: %v", scanBalance.Text(), err)
			return
		}
		t := time.Now()
		address = calculateHash(t.String())
		validators[address] = balance
		fmt.Println(validators)
		break
	}

	io.WriteString(conn, "Enter a new amount:")

	scanner := bufio.NewScanner(conn)

	// take in Amount from stdin and add it to blockchain after conducting necessary validation
	go func() {
		for scanner.Scan() {
			amount, err := strconv.Atoi(scanner.Text())
			if err != nil {
				log.Printf("%v not a number: %v", scanner.Text(), err)
				delete(validators, address) // bad input revokes your status as validator
				continue
			}
			mutex.Lock()
			oldLastIndex := Blockchain[len(Blockchain) - 1]
			mutex.Unlock()

			newBlock, err := generateBlock(oldLastIndex, "Nick", "Eric", amount, address)
			if err != nil {
				log.Println(err)
				continue
			}
			if isBlockValid(newBlock, oldLastIndex) {
				candidateBlocks <- newBlock
			}
			io.WriteString(conn, "\nEnter a new amount:")
		}
	}()

	// re-broadcast the blockchain every 10 seconds
	for {
		time.Sleep(10 * time.Second)

		// put a lock on Blockchain datastructure when adding to it
		mutex.Lock()
		output, err := json.Marshal(Blockchain)
		mutex.Unlock()

		if err != nil {
			log.Fatal(err)
		}

		io.WriteString(conn, string(output) + "\n")
	}
}

// randomly picks from validators, weighted by stake
func pickWinner() {
	time.Sleep(10 * time.Second)

	mutex.Lock()
	temp := tempBlocks 
	mutex.Unlock()

	lotteryPool := []string{}
	if len(temp) > 0 {
		//modified proof of stake algorithm
	OUTER:
		for _, block := range temp {
			// if already in lottery pool, skip
			for _, node := range lotteryPool {
				if block.Validator == node {
					continue OUTER
				}
			}

			mutex.Lock()
			setValidators := validators
			mutex.Unlock()

			k, ok := setValidators[block.Validator]
			if ok {
				for i := 0; i < k; i++ {
					lotteryPool = append(lotteryPool, block.Validator)
				}
			}
		}

		// randomly pick from the lottery pool
		s := rand.NewSource(time.Now().Unix())
		r := rand.New(s)
		winner := lotteryPool[r.Intn(len(lotteryPool))]

		// add winner's block to blockchain and broadcast
		for _, block := range temp {
			if block.Validator == winner {
				mutex.Lock()
				Blockchain = append(Blockchain, block)
				mutex.Unlock()

				for _ = range validators {
					announcements <- "\nWinning Validator: " + winner + "\n"
				}
				break
			}
		}
	}

	mutex.Lock()
	tempBlocks = []Block{}
	mutex.Unlock()
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	// set up Genesis block
	t := time.Now()
	genesisBlock := Block{}
	genesisBlock = Block{0, t.String(), "", "", 0, calculateBlockHash(genesisBlock), "", ""}
	spew.Dump(genesisBlock)
	Blockchain = append(Blockchain, genesisBlock)

	httpPort := os.Getenv("PORT")

	// start TCP and serve TCP server
	server, err := net.Listen("tcp", ":"+httpPort)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("HTTP Server Listening on port :", httpPort)
	defer server.Close()

	go func() {
		for candidate := range candidateBlocks {
			mutex.Lock()
			tempBlocks = append(tempBlocks, candidate)
			mutex.Unlock()
		}
	}()

	go func() {
		for {
			pickWinner()
		}
	}()

	// create a new connection each time we receive a request
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}
}