package main

import (
	"log"

	context "golang.org/x/net/context"

	"github.com/nyu-distributed-systems-fa18/algorand/pb"
)

// The struct for data to send over channel
type InputChannelType struct {
	command  pb.Command
	response chan pb.Result
}

// The struct for blockchain stores.
type BCStore struct {
	C     chan InputChannelType
  	blockchain []*pb.Block
}

func (bcs *BCStore) Get(ctx context.Context, in *pb.Empty) (*pb.Result, error) {
	// Create a channel
	c := make(chan pb.Result)
	// Create a request
	r := pb.Command{Operation: pb.Op_GET, Arg: &pb.Command_Empty{Empty: in}}
	// Send request over the channel
	bcs.C <- InputChannelType{command: r, response: c}
	log.Printf("Waiting for get response")
	result := <-c
	// The bit below works because Go maps return the 0 value for non existent keys, which is empty in this case.
	return &result, nil
}

func (bcs *BCStore) Send(ctx context.Context, in *pb.Transaction) (*pb.Result, error) {
	// Create a channel
	c := make(chan pb.Result)
	// Create a request
	r := pb.Command{Operation: pb.Op_SEND, Arg: &pb.Command_Tx{Tx: in}}
	// Send request over the channel
	bcs.C <- InputChannelType{command: r, response: c}
	log.Printf("Waiting for send response")
	result := <-c

	return &result, nil
}

func (bcs *BCStore) GetResponse(arg *pb.Empty) pb.Result {
	return pb.Result{Result: &pb.Result_Bc{Bc: &pb.Blockchain{Blocks: bcs.blockchain}}}
}

func (bcs *BCStore) SendResponse(arg *pb.Transaction) pb.Result {
	newBlock := generateBlock(nil, arg)
	bcs.blockchain = append(bcs.blockchain, newBlock)
  	return pb.Result{Result: &pb.Result_Bc{Bc: &pb.Blockchain{Blocks: bcs.blockchain}}}
}

func (bcs *BCStore) HandleCommand(op InputChannelType) {
	switch c := op.command; c.Operation {
	case pb.Op_GET:
		arg := c.GetEmpty()
		result := bcs.GetResponse(arg)
		op.response <- result
	case pb.Op_SEND:
		arg := c.GetTx()
		result := bcs.SendResponse(arg)
		op.response <- result
	default:
		// Sending a blank response to just free things up, but we don't know how to make progress here.
		op.response <- pb.Result{}
		log.Fatalf("Unrecognized operation %v", c)
	}
}
