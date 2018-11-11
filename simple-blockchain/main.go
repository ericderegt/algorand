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
}

// Global functions
var Blockchain []Block
var bcServer chan[]Block // handles incoming concurrent Blocks
var mutex = &sync.Mutex{}

// Blockchain Code
func calculateHash(block Block) string {
	record := string(block.Index) + block.Timestamp + block.Receiver + block.Payer + string(block.Amount) + block.PrevHash
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func generateBlock(oldBlock Block, Receiver string, Payer string, Amount int) (Block, error) {

	var newBlock Block

	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.Receiver = Receiver
	newBlock.Payer = Payer
	newBlock.Amount = Amount
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = calculateHash(newBlock)

	return newBlock, nil
}

func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index || oldBlock.Hash != newBlock.PrevHash || calculateHash(newBlock) != newBlock.Hash {
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

func handleWriteBlock(w http.ResponseWriter, r *http.Request) {
	var m Message

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&m); err != nil {
		respondWithJSON(w, r, http.StatusBadRequest, r.Body)
		return
	}
	defer r.Body.Close()

	newBlock, err := generateBlock(Blockchain[len(Blockchain)-1], m.Receiver, m.Payer, m.Amount)
	if err != nil {
		respondWithJSON(w, r, http.StatusInternalServerError, m)
		return
	}
	if isBlockValid(newBlock, Blockchain[len(Blockchain)-1]) {
		newBlockchain := append(Blockchain, newBlock)
		replaceChain(newBlockchain)
		spew.Dump(Blockchain)
	}

	respondWithJSON(w, r, http.StatusCreated, newBlock)
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	io.WriteString(conn, "Enter a new amount:")

	scanner := bufio.NewScanner(conn)

	// take in Amount from stdin and add it to blockchain after conducting necessary validation
	go func() {
		for scanner.Scan() {
			amount, err := strconv.Atoi(scanner.Text())
			if err != nil {
				log.Printf("%v not a number: %v", scanner.Text(), err)
				continue
			}
			newBlock, err := generateBlock(Blockchain[len(Blockchain)-1], "Nick", "Eric", amount)
			if err != nil {
				log.Println(err)
				continue
			}
			if isBlockValid(newBlock, Blockchain[len(Blockchain)-1]) {
				newBlockchain := append(Blockchain, newBlock)
				replaceChain(newBlockchain)
			}

			bcServer <- Blockchain
			io.WriteString(conn, "\nEnter a new amount:")
		}
	}()

	// re-broadcast the blockchain every 10 seconds
	go func() {
		for {
			time.Sleep(10 * time.Second)

			// put a lock on Blockchain datastructure when adding to it
			mutex.Lock()
			output, err := json.Marshal(Blockchain)
			if err != nil {
				log.Fatal(err)
			}
			mutex.Unlock()

			io.WriteString(conn, string(output))
		}
	}()

	// pretty print the Blockchain
	for _ = range bcServer {
		spew.Dump(Blockchain)
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	bcServer = make(chan []Block)

	// set up Genesis block
	t := time.Now()
	genesisBlock := Block{0, t.String(), "", "", 0, "", "",}
	spew.Dump(genesisBlock)
	Blockchain = append(Blockchain, genesisBlock)

	// start TCP and serve it
	server, err := net.Listen("tcp", ":"+os.Getenv("ADDR"))
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()

	// create a new connection each time we receive a request
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}
}