package main

import (
	"flag"
	"fmt"
	"log"
	// rand "math/rand"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"

	"github.com/nyu-distributed-systems-fa18/algorand/pb"
)

func createGenesisBlock() *pb.Block {
	genesisBlock := new(pb.Block)
	genesisBlock.Id = 0
	genesisBlock.Timestamp = time.Now().String()
	genesisBlock.PrevHash = ""
	genesisBlock.Hash = calculateHash(genesisBlock)
	genesisBlock.Tx = nil
	return genesisBlock
}

func main() {
	// Argument parsing
	var seed int64
	var peers arrayPeers
	var clientPort int
	var algorandPort int
	flag.Int64Var(&seed, "seed", -1,
		"Seed for random number generator, values less than 0 result in use of time")
	flag.IntVar(&clientPort, "port", 3000,
		"Port on which server should listen to client requests")
	flag.IntVar(&algorandPort, "algorand", 3001,
		"Port on which server should listen to Algorand requests")
	flag.Var(&peers, "peer", "A peer for this process")
	flag.Parse()

	// Get hostname
	name, err := os.Hostname()
	if err != nil {
		// Without a host name we can't really get an ID, so die.
		log.Fatalf("Could not get hostname")
	}

	id := fmt.Sprintf("%s:%d", name, algorandPort)
	log.Printf("Starting peer with ID %s", id)

	// Convert port to a string form
	portString := fmt.Sprintf(":%d", clientPort)
	// Create socket that listens on the supplied port
	c, err := net.Listen("tcp", portString)
	if err != nil {
		// Note the use of Fatalf which will exit the program after reporting the error.
		log.Fatalf("Could not create listening socket %v", err)
	}
	// Create a new GRPC server
	s := grpc.NewServer()

	// Create service to handle BlockChain
	bcs := BCStore{C: make(chan InputChannelType), blockchain: []*pb.Block{}}

	// Init with GenesisBlock
	bcs.blockchain = append(bcs.blockchain, createGenesisBlock())

	// Spin up algorand server
	go serve(&bcs, &peers, id, algorandPort)

	pb.RegisterBCStoreServer(s, &bcs)
	log.Printf("Going to listen on port %v", clientPort)
	// Start serving, this will block this function and only return when done.
	if err := s.Serve(c); err != nil {
		log.Fatalf("Failed to serve %v", err)
	}
	log.Printf("Done listening")
}
