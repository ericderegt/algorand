package main

import (
	"fmt"
	"log"
	// "math"
	"net"
	"time"

	context "golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/nyu-distributed-systems-fa18/algorand/pb"
)

// Persistent and volatile state
type ServerState struct {
	privateKey   int64
	publicKey 	 int64
	period		 int64
}

type AppendBlockInput struct {
	arg	*pb.AppendBlockArgs
	response chan pb.AppendBlockRet
}

type Algorand struct {
	AppendChan chan AppendBlockInput
}

func (a *Algorand) AppendBlock(ctx context.Context, arg *pb.AppendBlockArgs) (*pb.AppendBlockRet, error) {
	c := make(chan pb.AppendBlockRet)
	a.AppendChan <- AppendBlockInput{arg: arg, response: c}
	result := <-c
	return &result, nil
}

func (a *Algorand) SIG(ctx context.Context, arg *pb.SIGArgs) (*pb.SIGRet, error) {
	return nil, nil
}

// Launch a GRPC service for this peer.
func RunAlgorandServer(algorand *Algorand, port int) {
	// Convert port to a string form
	portString := fmt.Sprintf(":%d", port)
	// Create socket that listens on the supplied port
	c, err := net.Listen("tcp", portString)
	if err != nil {
		// Note the use of Fatalf which will exit the program after reporting the error.
		log.Fatalf("Could not create listening socket %v", err)
	}
	// Create a new GRPC server
	s := grpc.NewServer()

	pb.RegisterAlgorandServer(s, algorand)
	log.Printf("Going to listen on port %v", port)

	// Start serving, this will block this function and only return when done.
	if err := s.Serve(c); err != nil {
		log.Fatalf("Failed to serve %v", err)
	}
}

func connectToPeer(peer string) (pb.AlgorandClient, error) {
	backoffConfig := grpc.DefaultBackoffConfig
	// Choose an aggressive backoff strategy here.
	backoffConfig.MaxDelay = 500 * time.Millisecond
	conn, err := grpc.Dial(peer, grpc.WithInsecure(), grpc.WithBackoffConfig(backoffConfig))
	// Ensure connection did not fail, which should not happen since this happens in the background
	if err != nil {
		return pb.NewAlgorandClient(nil), err
	}
	return pb.NewAlgorandClient(conn), nil
}

// The main service loop.
func serve(bcs *BCStore, peers *arrayPeers, id string, port int) {
	algorand := Algorand{AppendChan: make(chan AppendBlockInput)}
	// Start in a Go routine so it doesn't affect us.
	go RunAlgorandServer(&algorand, port)

	state := ServerState{
		privateKey: 0,
		publicKey: 0,
		period: 1,
	}

	peerClients := make(map[string]pb.AlgorandClient)
	peerCount := int64(0)

	for _, peer := range *peers {
		client, err := connectToPeer(peer)
		if err != nil {
			log.Fatalf("Failed to connect to GRPC server %v", err)
		}

		peerClients[peer] = client
		peerCount++
		log.Printf("Connected to %v", peer)
	}
	log.Printf("My ServerState is: %#v", state)

	type AppendResponse struct {
		ret *pb.AppendBlockRet
		err error
		peer string
	}

	appendResponseChan := make(chan AppendResponse)

	// Run forever handling inputs from various channels
	for {
		select{
		case op := <-bcs.C:
			bcs.HandleCommand(op)
		case ab := <-algorand.AppendChan:
			// we got an AppendBlock request
			log.Printf("AppendBlock: %#v", ab)
		case ar := <-appendResponseChan:
			// we got a response to our AppendBlock request
			log.Printf("AppendBlockResponse: %#v", ar)
		}
	}
	log.Printf("Strange to arrive here")
}
