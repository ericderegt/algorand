package main

import (
	"fmt"
	"log"
	"net"
	"time"
	"strings"
	"sort"
	// "math/rand"
	"strconv"

	context "golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/nyu-distributed-systems-fa18/algorand/pb"
)

// Persistent and volatile state
type ServerState struct {
	privateKey   		int64
	publicKey 	 		int64
	round		 		int64
	lastCompletedRound 	int64
	tempBlock	 		pb.Block
	proposedBlock 		pb.Block
	seed 				string

	periodState			PeriodState
	lastPeriodState		PeriodState
}

type PeriodState struct {
	nextVotes	map[string]int64
	softVotes	map[string]int64
	certVotes	map[string]int64
	period		int64
}

type AppendBlockInput struct {
	arg	*pb.AppendBlockArgs
	response chan pb.AppendBlockRet
}

type AppendTransactionInput struct {
	arg *pb.AppendTransactionArgs
	response chan pb.AppendTransactionRet
}

type ProposeBlockInput struct {
	arg *pb.ProposeBlockArgs
	response chan pb.ProposeBlockRet
}

type VoteInput struct {
	arg *pb.VoteArgs
	response chan pb.VoteRet
}

type Algorand struct {
	AppendBlockChan chan AppendBlockInput
	AppendTransactionChan chan AppendTransactionInput
	ProposeBlockChan chan ProposeBlockInput
	VoteChan chan VoteInput
}

func (a *Algorand) AppendBlock(ctx context.Context, arg *pb.AppendBlockArgs) (*pb.AppendBlockRet, error) {
	c := make(chan pb.AppendBlockRet)
	a.AppendBlockChan <- AppendBlockInput{arg: arg, response: c}
	result := <-c
	return &result, nil
}

func (a *Algorand) AppendTransaction(ctx context.Context, arg *pb.AppendTransactionArgs) (*pb.AppendTransactionRet, error) {
	c := make(chan pb.AppendTransactionRet)
	a.AppendTransactionChan <- AppendTransactionInput{arg: arg, response: c}
	result := <-c
	return &result, nil
}

func (a *Algorand) Vote(ctx context.Context, arg *pb.VoteArgs) (*pb.VoteRet, error) {
	c := make(chan pb.VoteRet)
	a.VoteChan <- VoteInput{arg: arg, response: c}
	result := <-c
	return &result, nil
}

func (a *Algorand) ProposeBlock(ctx context.Context, arg *pb.ProposeBlockArgs) (*pb.ProposeBlockRet, error) {
	c := make(chan pb.ProposeBlockRet)
	a.ProposeBlockChan <- ProposeBlockInput{arg: arg, response: c}
	result := <-c
	return &result, nil
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

func restartTimer(timer *time.Timer) {
	stopped := timer.Stop()

	if !stopped {
		for len(timer.C) > 0 {
			<-timer.C
		}

	}
	timer.Reset(5000 * time.Millisecond)
}

func initPeriodState(p int64) PeriodState {
	newPeriodState := PeriodState{
		nextVotes: make(map[string]int64),
		softVotes: make(map[string]int64),
		certVotes: make(map[string]int64),
		period: p,
	}

	return newPeriodState
}

// The main service loop.
func serve(bcs *BCStore, peers *arrayPeers, id string, port int) {

	algorand := Algorand{
		AppendBlockChan: make(chan AppendBlockInput),
		AppendTransactionChan: make(chan AppendTransactionInput),
		ProposeBlockChan: make(chan ProposeBlockInput),
	}
	// Start in a Go routine so it doesn't affect us.
	go RunAlgorandServer(&algorand, port)

	state := ServerState{
		privateKey: 0,
		publicKey: 0,
		round: 0,
		lastCompletedRound: 0,
		seed: "thisshouldbeahash", // R in the paper
	}

	peerClients := make(map[string]pb.AlgorandClient)
	peerCount := int64(0)
	userIds := make([]string, len(*peers) + 1)

	for i, peer := range *peers {
		client, err := connectToPeer(peer)
		if err != nil {
			log.Fatalf("Failed to connect to GRPC server %v", err)
		}

		peerClients[peer] = client

		split := strings.Split(peer, ":")
		userIds[i] = split[1]

		peerCount++
		log.Printf("Connected to %v", peer)
	}

	// add my Id to pool of userIds
	split := strings.Split(id, ":")
	userId := split[1]
	userIds[len(userIds) - 1] = split[1]

	// sort userIds so all Algorand servers have the same list of userIds
	sort.Strings(userIds)
	log.Printf("UserIds: %#v", userIds)

	type AppendBlockResponse struct {
		ret *pb.AppendBlockRet
		err error
		peer string
	}

	type AppendTransactionResponse struct {
		ret *pb.AppendTransactionRet
		err error
		peer string
	}

	type ProposeBlockResponse struct {
		ret *pb.ProposeBlockRet
		err error
		peer string
	}

	appendBlockResponseChan := make(chan AppendBlockResponse)
	appendTransactionResponseChan := make(chan AppendTransactionResponse)
	proposeBlockResponseChan := make(chan ProposeBlockResponse)

	// Set timer to check for new rounds
	timer := time.NewTimer(5000 * time.Millisecond)

	// set hardcode k to be 2 for now, so 2 members will always be selected to committee
	k := int64(2)

	// Run forever handling inputs from various channels
	for {
		select{
		case <-timer.C:
			log.Printf("Timer went off")

			if state.lastCompletedRound == state.round {
				state.round++

				// each server needs exact same seed per round so they all see the same selection
				_, _, votes := sortition(state.privateKey, state.round, "proposer", userId, userIds, k)

				// start at period 1
				period := int64(1)

				// initialize periodState
				state.lastPeriodState = state.periodState
				state.periodState = initPeriodState(period)

				// we capture our tempBlock at the time agreement starts. We will reconcile this block after agreement ends
				state.proposedBlock = state.tempBlock

				v := calculateHash(&state.proposedBlock)

				sigParams := []string{state.seed, strconv.FormatInt(period, 10)}

				sig := SIG(userId, sigParams)

				// Value proposal step
				for votes > 0 {
					// broadcast proposal
					for p, c := range peerClients {
						go func(c pb.AlgorandClient, p string, v string, sig *pb.SIGRet) {
							log.Printf("Sent proposal to peer %v", p)
							ret, err := c.ProposeBlock(context.Background(), &pb.ProposeBlockArgs{Block: v, Credential: sig})
							proposeBlockResponseChan <- ProposeBlockResponse{ret: ret, err: err, peer: p}
						}(c, p, v, sig)
					}
					// if period == 1 || (period > 1 && emptyNextVote) {
					// 	// propose own value
					// }
					votes--
				}
				state.lastCompletedRound++
			}

			restartTimer(timer)
		case op := <-bcs.C:
			// Received a command from client
			// TODO: Add Transaction to our local block, broadcast to every user
			log.Printf("Transaction request: %#v, Round: %v", op.command.Arg, state.round)

			if op.command.Operation == pb.Op_SEND {
				state.tempBlock.Tx = append(state.tempBlock.Tx, op.command.GetTx())

				// TODO - broadcast, and figure out when to reponse to client?

				// broadcast
				for p, c := range peerClients {
					transaction := op.command.GetTx()

					go func(c pb.AlgorandClient, p string, transaction *pb.Transaction) {
						log.Printf("Sent transaction to peer %v", p)
						ret, err := c.AppendTransaction(context.Background(), &pb.AppendTransactionArgs{Peer: p, Tx: transaction})
						appendTransactionResponseChan <- AppendTransactionResponse{ret: ret, err: err, peer: p}
					}(c, p, transaction)
				}

			} else {
				bcs.HandleCommand(op)
			}

			// Check if add new Transaction, or simply get the curent Blockchain
			// if op.command.Operation == pb.Op_GET {
			// 	log.Printf("Request to view the blockchain")
			// 	bcs.HandleCommand(op)
			// } else {
			// 	log.Printf("Request to add new Block")

			// 	// for now, we simply append to our blockchain and broadcast the new blockchain to all known peers
			// 	bcs.HandleCommand(op)
			// 	for p, c := range peerClients {
			// 		go func(c pb.AlgorandClient, blockchain []*pb.Block, p string) {
			// 			ret, err := c.AppendBlock(context.Background(), &pb.AppendBlockArgs{Blockchain: blockchain, Peer: id})
			// 			appendBlockResponseChan <- AppendResponse{ret: ret, err: err, peer: p}
			// 		}(c, bcs.blockchain, p)
			// 	}
			// 	log.Printf("Period: %v, Blockchain: %#v", state.period, bcs.blockchain)
			// }
		case ab := <-algorand.AppendBlockChan:
			// we got an AppendBlock request
			log.Printf("AppendBlock from %v", ab.arg.Peer)

			// for now, just check if blockchain is longer than ours
			// if yes, overwrite ours and return true
			// if no, return false
			if len(ab.arg.Blockchain) > len(bcs.blockchain) {
				bcs.blockchain = ab.arg.Blockchain
				ab.response <- pb.AppendBlockRet{Success: true}
			} else {
				ab.response <- pb.AppendBlockRet{Success: false}
			}
		case abr := <-appendBlockResponseChan:
			// we got a response to our AppendBlock request
			log.Printf("AppendBlockResponse: %#v", abr)

		case at := <-algorand.AppendTransactionChan:
			// we got an AppendTransaction request
			log.Printf("AppendTransaction from %v", at.arg.Peer)

			state.tempBlock.Tx = append(state.tempBlock.Tx, at.arg.Tx)
			log.Printf("Temp block: %#v", len(state.tempBlock.Tx))

			at.response <- pb.AppendTransactionRet{Success: true}

		case atr := <- appendTransactionResponseChan:
			// we got a response to our AppendTransaction request
			log.Printf("AppendTransactionResponse: %#v", atr)

		case pb := <-algorand.ProposeBlockChan:
			proposerId := pb.arg.Credential.UserId
			log.Printf("ProposeBlock from %v", proposerId)

			verified := verifySort(proposerId, userIds, state.round, k)
			if verified {
				log.Printf("VERIFIED that %v is on the committee for round %v", proposerId, state.round)
			} else {
				log.Printf("DENIED that %v is on the committee for round %v", proposerId, state.round)
			}

		case pbr := <-proposeBlockResponseChan:
			log.Printf("ProposeBlockResponse: %#v", pbr)
		}
	}
	log.Printf("Strange to arrive here")
}
