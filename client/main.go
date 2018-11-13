package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"encoding/json"

	context "golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/nyu-distributed-systems-fa18/algorand/pb"
)

func usage() {
	fmt.Printf("Usage %s <message>\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	// Take endpoint as input
	flag.Usage = usage
	flag.Parse()
	// If there is no endpoint fail
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	endpoint := flag.Args()[0]
	log.Printf("Connecting to %v", endpoint)
	// Connect to the server. We use WithInsecure since we do not configure https in this class.
	conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
	//Ensure connection did not fail.
	if err != nil {
		log.Fatalf("Failed to dial GRPC server %v", err)
	}
	log.Printf("Connected")

	// Create Algorand client
	bcc := pb.NewBCStoreClient(conn)

	// Send Transaction
	transReq := &pb.Transaction{V: "Eric and Nick are good at blockchain"}
	res, err := bcc.Send(context.Background(), transReq)
	if err != nil {
		log.Fatalf("Send error")
	}
	bytes, err := json.MarshalIndent(res.GetBc(), "", "    ")
	if err != nil {
		log.Println(err)
	}
	log.Printf(string(bytes))
}
