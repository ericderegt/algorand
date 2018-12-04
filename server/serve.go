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
	readyForNextRound 	bool
	tempBlock	 		*pb.Block
	proposedBlock 		*pb.Block
	seed 				string
	periodStates		map[int64]*PeriodState
	period int64
	step int64
}

type PeriodState struct {
	proposedValues	map[string]string
	valueToBlock	map[string]*pb.Block

	nextVotes		map[string]int64
	softVotes		map[string]int64
	certVotes		map[string]int64

	haveNextVoted	map[string]bool
	haveSoftVoted	map[string]bool
	haveCertVoted	map[string]bool

	myCertVote  	string
	mySoftVote		string
	myNextVote		string
	startingValue	string
	period			int64
	round			int64
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

type RequestBlockChainInput struct {
	arg *pb.RequestBlockChainArgs
	response chan pb.RequestBlockChainRet
}

type Algorand struct {
	AppendBlockChan chan AppendBlockInput
	AppendTransactionChan chan AppendTransactionInput
	ProposeBlockChan chan ProposeBlockInput
	VoteChan chan VoteInput
	RequestBlockChainChan chan RequestBlockChainInput
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

func (a *Algorand) RequestBlockChain(ctx context.Context, arg *pb.RequestBlockChainArgs) (*pb.RequestBlockChainRet, error) {
	c := make(chan pb.RequestBlockChainRet)
	a.RequestBlockChainChan <- RequestBlockChainInput{arg: arg, response: c}
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

func restartTimer(timer *time.Timer, ms int64) {
	stopped := timer.Stop()

	if !stopped {
		for len(timer.C) > 0 {
			<-timer.C
		}

	}
	timer.Reset(time.Duration(ms) * time.Millisecond)
}

func initPeriodState(p int64) *PeriodState {
	newPeriodState := PeriodState{
		proposedValues: make(map[string]string),
		valueToBlock:   make(map[string]*pb.Block),

		nextVotes: 		make(map[string]int64),
		softVotes: 		make(map[string]int64),
		certVotes: 		make(map[string]int64),

		haveNextVoted:	make(map[string]bool),
		haveSoftVoted:	make(map[string]bool),
		haveCertVoted:	make(map[string]bool),

		myCertVote: 	"",
		mySoftVote: 	"",
		myNextVote:		"",
		startingValue: 	"",
		period: 		p,
	}

	return &newPeriodState
}

func handleHalt(bcs *BCStore, state *ServerState, newBlock *pb.Block) {
    log.Printf("AGREEMENT!")
    bcs.blockchain = append(bcs.blockchain, newBlock)
    log.Printf("Chain: %v", PrettyPrint(bcs.blockchain))

    // Handle Halting Condition
    state.readyForNextRound = true
    state.round++

    state.period = int64(1)
	state.step = int64(1)
	state.periodStates = map[int64]*PeriodState{}

	// set up 10 periodStates in case others are that far ahead, we can capture all their stuff
	for p := int64(1); p <= 2; p++ {
        state.periodStates[p] = initPeriodState(p)
    	state.periodStates[p].startingValue = "_|_"
    }
}

// The main service loop.
func serve(bcs *BCStore, peers *arrayPeers, id string, port int) {

	log.Printf("peers: %#v", peers)

	algorand := Algorand{
		AppendBlockChan: make(chan AppendBlockInput),
		AppendTransactionChan: make(chan AppendTransactionInput),
		ProposeBlockChan: make(chan ProposeBlockInput),
		VoteChan: make(chan VoteInput),
		RequestBlockChainChan: make(chan RequestBlockChainInput),
	}
	// Start in a Go routine so it doesn't affect us.
	go RunAlgorandServer(&algorand, port)

	state := ServerState{
		privateKey: 0,
		publicKey: 0,
		round: 1,
		readyForNextRound: true,
		seed: "thisshouldbeahash", // R in the paper
	}
	state.tempBlock = new(pb.Block)

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

	if peerCount < 3 {
		// log.Fatalf("Need at least 4 nodes to achieve Byzantine fault tolerance")
	}

	// 2t+1 required votes for Byzantine fault tolerance
	t := peerCount / 3
	requiredVotes := 2*t + 1

	// REMOVE
	requiredVotes = 3

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

	type VoteResponse struct {
		ret *pb.VoteRet
		err error
		peer string
	}

	type RequestBlockChainResponse struct {
		ret *pb.RequestBlockChainRet
		err error
		peer string
	}

	appendBlockResponseChan := make(chan AppendBlockResponse)
	appendTransactionResponseChan := make(chan AppendTransactionResponse)
	proposeBlockResponseChan := make(chan ProposeBlockResponse)
	voteResponseChan := make(chan VoteResponse)
	requestBlockChainResponseChan := make(chan RequestBlockChainResponse)

	// Set timer to check for new rounds
	roundTimer := time.NewTimer(5000 * time.Millisecond)
	agreementTimer := time.NewTimer(10000 * time.Millisecond)

	// set hardcode k to be 2 for now, so 2 members will always be selected to committee
	k := int64(2)

	// intialize everyone's stake between 1 to 10 tokens
	idToStake := initStake(userIds, 1, 10)

	// generate candidates using every user's stake which will be used for sortition
	candidates := generateCandidatesByStake(userIds, idToStake)


	//Prepare periodState
	state.period = int64(1)
	state.step = int64(1)
	state.periodStates = map[int64]*PeriodState{}
	
	// set up 10 periodStates in case others are that far ahead, we can capture all their stuff
	for p := int64(1); p <= 2; p++ {
        state.periodStates[p] = initPeriodState(p)
    	state.periodStates[p].startingValue = "_|_"
    }

	// Run forever handling inputs from various channels
	for {
		select{
		case <-roundTimer.C:
			// propose block if last round complete or very first round
			if state.readyForNextRound {
				log.Printf("Starting round %v, period %v", state.round, state.period)
				state.readyForNextRound = false

				// we don't want step two to happen too quick before users can collect proposedBlocks
				restartTimer(agreementTimer, 5000)

				// we capture our tempBlock at the time agreement starts. We will reconcile this block after agreement ends
				state.proposedBlock = prepareBlock(state.tempBlock, bcs.blockchain)
				b := state.proposedBlock
				v := calculateHash(state.proposedBlock)

				// each server needs exact same seed per round so they all see the same selection
				_, _, votes := sortition(state.privateKey, state.round, "proposer", userId, candidates, k)

				sigParams := []string{strconv.FormatInt(state.round, 10), strconv.FormatInt(state.period, 10)}

				sig := SIG(userId, sigParams)

				// Value proposal step
				for votes > 0 {
					// add your own proposal to proposedBlock map
					proposerCredential := []string{userId, sig.SignedMessage}
					proposerHash := signMessage(proposerCredential)
					state.periodStates[state.period].proposedValues[proposerHash] = v
					state.periodStates[state.period].valueToBlock[v] = b

					// broadcast proposal
					for p, c := range peerClients {
						go func(c pb.AlgorandClient, p string, b *pb.Block, v string, sig *pb.SIGRet, round int64) {
							log.Printf("Sent proposal to peer %v", p)
							ret, err := c.ProposeBlock(context.Background(), &pb.ProposeBlockArgs{Block: b, Credential: sig, Value: v, Round: round, Peer: userId})
							proposeBlockResponseChan <- ProposeBlockResponse{ret: ret, err: err, peer: p}
						}(c, p, b, v, sig, state.round)
					}
					votes--
				}
			}

			restartTimer(roundTimer, 5000)

		case <-agreementTimer.C:
			// if we are currently in agreement protocol
			if !state.readyForNextRound && state.step < 5 {
				state.step++
			}

			if state.step == 2 {
				log.Printf("PERIOD %v, STEP 2", state.period)
				softVoteV := runStep2(state.periodStates, requiredVotes, state.period)
				log.Printf("soft vote is %v", softVoteV)

				if softVoteV != "" {
					// add my own vote for this value
					state.periodStates[state.period].softVotes[softVoteV]++
					state.periodStates[state.period].mySoftVote = softVoteV;

					// broadcast my decision to vote for this value
					message := []string{softVoteV, "soft", strconv.FormatInt(state.period, 10)}
					softVoteSIG := SIG(userId, message)

					for p, c := range peerClients {
						go func(c pb.AlgorandClient, p string, softVoteSIG *pb.SIGRet, round int64) {
							log.Printf("Sent soft vote to peer %v", p)
							ret, err := c.Vote(context.Background(), &pb.VoteArgs{Message: softVoteSIG, Round: round, Period: state.period, Peer: userId})
							voteResponseChan <- VoteResponse{ret: ret, err: err, peer: p}
						}(c, p, softVoteSIG, state.round)
					}
				}
			} else if state.step == 3 {
				log.Printf("PERIOD %v, STEP 3", state.period)
				certVoteV := runStep3(state.periodStates, requiredVotes, state.period)
				log.Printf("cert vote is %v", certVoteV)

				if certVoteV != "" {
					// add my own vote for this value
					state.periodStates[state.period].certVotes[certVoteV]++
					state.periodStates[state.period].myCertVote = certVoteV;

					message := []string{certVoteV, "cert", strconv.FormatInt(state.period, 10)}
					certVoteSIG := SIG(userId, message)

					for p, c := range peerClients {
						go func(c pb.AlgorandClient, p string, certVoteSIG *pb.SIGRet, round int64) {
							log.Printf("Sent cert vote to peer %v", p)
							ret, err := c.Vote(context.Background(), &pb.VoteArgs{Message: certVoteSIG, Round: round, Period: state.period, Peer: userId})
							voteResponseChan <- VoteResponse{ret: ret, err: err, peer: p}
						}(c, p, certVoteSIG, state.round)
					}

					// check if our own vote helped us reach requiredVotes
					haltValue := checkHaltingCondition(state.periodStates, requiredVotes)
					if haltValue != "" {
						handleHalt(bcs, &state, state.periodStates[state.period].valueToBlock[haltValue])
					}
				}
			} else if state.step == 4 {
				log.Printf("PERIOD %v, STEP 4", state.period)
				nextVoteV := runStep4(state.periodStates, requiredVotes, state.period)
				log.Printf("next vote is %v", nextVoteV)

				// add my own vote for this value
				state.periodStates[state.period].nextVotes[nextVoteV]++
				state.periodStates[state.period].myNextVote = nextVoteV;

				message := []string{nextVoteV, "next", strconv.FormatInt(state.period, 10)}
				nextVoteSIG := SIG(userId, message)

				for p, c := range peerClients {
					go func(c pb.AlgorandClient, p string, nextVoteSIG *pb.SIGRet, round int64) {
						log.Printf("Sent next vote to peer %v", p)
						ret, err := c.Vote(context.Background(), &pb.VoteArgs{Message: nextVoteSIG, Round: round, Period: state.period, Peer: userId})
						voteResponseChan <- VoteResponse{ret: ret, err: err, peer: p}
					}(c, p, nextVoteSIG, state.round)
				}
			} else if state.step == 5 {
				log.Printf("PERIOD %v, STEP 5", state.period)
				nextVoteV := runStep5(state.periodStates, requiredVotes, state.period)
				log.Printf("next vote is %v", nextVoteV)

				if nextVoteV != "" {
					// add my own vote for this value
					state.periodStates[state.period].nextVotes[nextVoteV]++
					state.periodStates[state.period].myNextVote = nextVoteV;

					message := []string{nextVoteV, "next", strconv.FormatInt(state.period, 10)}
					nextVoteSIG := SIG(userId, message)

					for p, c := range peerClients {
						go func(c pb.AlgorandClient, p string, nextVoteSIG *pb.SIGRet, round int64) {
							log.Printf("Sent next vote to peer %v", p)
							ret, err := c.Vote(context.Background(), &pb.VoteArgs{Message: nextVoteSIG, Round: round, Period: state.period, Peer: userId})
							voteResponseChan <- VoteResponse{ret: ret, err: err, peer: p}
						}(c, p, nextVoteSIG, state.round)
					}

					// finish period
					state.period ++
					state.step = 1
					state.periodStates[state.period].startingValue = calculateHash(state.proposedBlock)
					
					// allow step1 to happen again
					state.readyForNextRound = true
				}
			}

			// Handle resetting the agreementTimer
			// we want a shorter timout for continously checking step5 again and again
			if state.step == 5 {
				restartTimer(agreementTimer, 2000)
			} else {
				restartTimer(agreementTimer, 5000)
			}

		case op := <-bcs.C:
			// Received a command from client
			// TODO: Add Transaction to our local block, broadcast to every user
			log.Printf("Transaction request: %#v, Round: %v", op.command.Arg, state.round)

			if op.command.Operation == pb.Op_SEND {
				state.tempBlock.Tx = append(state.tempBlock.Tx, op.command.GetTx())

				// broadcast
				for p, c := range peerClients {
					transaction := op.command.GetTx()

					go func(c pb.AlgorandClient, p string, transaction *pb.Transaction) {
						log.Printf("Sent transaction to peer %v", p)
						ret, err := c.AppendTransaction(context.Background(), &pb.AppendTransactionArgs{Peer: p, Tx: transaction})
						appendTransactionResponseChan <- AppendTransactionResponse{ret: ret, err: err, peer: p}
					}(c, p, transaction)
				}

			}
			bcs.HandleCommand(op)

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

		case pbc := <-algorand.ProposeBlockChan:
			if pbc.arg.Round > state.round {
				for p, c := range peerClients {
					go func(c pb.AlgorandClient, p string) {
						ret, err := c.RequestBlockChain(context.Background(), &pb.RequestBlockChainArgs{Peer: p})
						requestBlockChainResponseChan <- RequestBlockChainResponse{ret: ret, err: err, peer: p}
					}(c, p)
				}
				pbc.response <- pb.ProposeBlockRet{Success: false}
				break
			}
			
			proposerId := pbc.arg.Credential.UserId
			log.Printf("ProposeBlock from %v", proposerId)

			verified := verifySort(proposerId, candidates, state.round, k, state.period)
			if verified {
				log.Printf("VERIFIED that %v is on the committee for round %v", proposerId, state.round)

				proposerCredential := []string{pbc.arg.Credential.UserId, pbc.arg.Credential.SignedMessage}

				proposerHash := signMessage(proposerCredential)
				// add verified block to list of blocks I've seen this period
				state.periodStates[state.period].proposedValues[proposerHash] = pbc.arg.Value
				state.periodStates[state.period].valueToBlock[pbc.arg.Value] = pbc.arg.Block
				pbc.response <- pb.ProposeBlockRet{Success: true}
			} else {
				// rejected proposed block
				log.Printf("DENIED that %v is on the committee for round %v", proposerId, state.round)
				pbc.response <- pb.ProposeBlockRet{Success: true}
			}

		case pbr := <-proposeBlockResponseChan:
			if pbr.err != nil || !pbr.ret.Success{

				p := pbr.peer
				c := peerClients[p]

				for period,_ := range state.periodStates {
					// calculate your signature for this round and period
					sigParams := []string{strconv.FormatInt(state.round, 10), strconv.FormatInt(period, 10)}
					sig := SIG(userId, sigParams)

					// get your proposal back out of the map
					proposerCredential := []string{userId, sig.SignedMessage}
					proposerHash := signMessage(proposerCredential)

					// check if you proposed a block for this round and period
					if v, ok := state.periodStates[period].proposedValues[proposerHash]; ok {
						b := state.periodStates[period].valueToBlock[v]

						go func(c pb.AlgorandClient, p string, b *pb.Block, v string, sig *pb.SIGRet, round int64) {
							ret, err := c.ProposeBlock(context.Background(), &pb.ProposeBlockArgs{Block: b, Credential: sig, Value: v, Round: round, Peer: userId})
							proposeBlockResponseChan <- ProposeBlockResponse{ret: ret, err: err, peer: p}
						}(c, p, b, v, sig, state.round)
					}
				}
			}

		case vc := <-algorand.VoteChan:
			if vc.arg.Round > state.round {
				for p, c := range peerClients {
					go func(c pb.AlgorandClient, p string) {
						ret, err := c.RequestBlockChain(context.Background(), &pb.RequestBlockChainArgs{Peer: userId})
						requestBlockChainResponseChan <- RequestBlockChainResponse{ret: ret, err: err, peer: p}
					}(c, p)
				}
				vc.response <- pb.VoteRet{Success: false}
				break
			}

			voterId := vc.arg.Message.UserId
			voteValue := vc.arg.Message.Message[0]
			voteType := vc.arg.Message.Message[1]
			votePeriod, _ := strconv.ParseInt(vc.arg.Message.Message[2], 10, 64)

			if voteType == "soft" {
				_, hasVoted := state.periodStates[votePeriod].haveSoftVoted[voterId]
				if !hasVoted {
					state.periodStates[votePeriod].softVotes[voteValue]++
					state.periodStates[votePeriod].haveSoftVoted[voterId] = true
					vc.response <- pb.VoteRet{Success: true}

					log.Printf("NewSoftVotes: %#v", state.periodStates[votePeriod].softVotes)
				} else {
					vc.response <- pb.VoteRet{Success: true}
				}
			} else if voteType == "cert" {
				_, hasVoted := state.periodStates[votePeriod].haveCertVoted[voterId]
				if !hasVoted {
					state.periodStates[votePeriod].certVotes[voteValue]++
					state.periodStates[votePeriod].haveCertVoted[voterId] = true
					vc.response <- pb.VoteRet{Success: true}

					log.Printf("NewCertVotes: %#v", state.periodStates[votePeriod].certVotes)

					// we need to check for halting condition anytime we see a new cert vote
					haltValue := checkHaltingCondition(state.periodStates, requiredVotes)
					if haltValue != "" {
						handleHalt(bcs, &state, state.periodStates[state.period].valueToBlock[haltValue])
					}
				} else {
					vc.response <- pb.VoteRet{Success: true}
				}
			} else if voteType == "next" {
				_, hasVoted := state.periodStates[votePeriod].haveNextVoted[voterId]
				if !hasVoted {
					state.periodStates[votePeriod].nextVotes[voteValue]++
					state.periodStates[votePeriod].haveNextVoted[voterId] = true
					vc.response <- pb.VoteRet{Success: true}

					log.Printf("NewNextVotes: %#v", state.periodStates[votePeriod].nextVotes)
				} else {
					vc.response <- pb.VoteRet{Success: true}
				}
			} else {
				log.Printf("Strange to arrive here")
				vc.response <- pb.VoteRet{Success: false}
			}
		case vr := <-voteResponseChan:
			if vr.err != nil || !vr.ret.Success {
				// retry sending every vote cast for this round in all periods
				p := vr.peer
				c := peerClients[p]

				for period,_ := range state.periodStates {

					// Send nextVote if needed
					nextVoteV := state.periodStates[period].myNextVote
					if nextVoteV != "" {
						message := []string{nextVoteV, "next", strconv.FormatInt(period, 10)}
						nextVoteSIG := SIG(userId, message)

						go func(c pb.AlgorandClient, p string, nextVoteSIG *pb.SIGRet, round int64) {
							ret, err := c.Vote(context.Background(), &pb.VoteArgs{Message: nextVoteSIG, Round: round, Period: period, Peer: userId})
							voteResponseChan <- VoteResponse{ret: ret, err: err, peer: p}
						}(c, p, nextVoteSIG, state.round)
					}
					
					// Send softVote if needed
					softVoteV := state.periodStates[period].mySoftVote
					if softVoteV != "" {
						message := []string{softVoteV, "soft", strconv.FormatInt(period, 10)}
						softVoteSIG := SIG(userId, message)

						go func(c pb.AlgorandClient, p string, softVoteSIG *pb.SIGRet, round int64) {
							ret, err := c.Vote(context.Background(), &pb.VoteArgs{Message: softVoteSIG, Round: round, Period: period, Peer: userId})
							voteResponseChan <- VoteResponse{ret: ret, err: err, peer: p}
						}(c, p, softVoteSIG, state.round)
					}

					// Send certVote if needed
					certVoteV := state.periodStates[period].myCertVote
					if certVoteV != "" {
						message := []string{certVoteV, "soft", strconv.FormatInt(period, 10)}
						certVoteSIG := SIG(userId, message)

						go func(c pb.AlgorandClient, p string, certVoteSIG *pb.SIGRet, round int64) {
							ret, err := c.Vote(context.Background(), &pb.VoteArgs{Message: certVoteSIG, Round: round, Period: period, Peer: userId})
							voteResponseChan <- VoteResponse{ret: ret, err: err, peer: p}
						}(c, p, certVoteSIG, state.round)
					}
				}
			}

		case bcc := <-algorand.RequestBlockChainChan:

			bcc.response <- pb.RequestBlockChainRet{Peer: userId, Blockchain: bcs.blockchain}

			// if someone asked to be caudght up, send proposed values
			for period,_ := range state.periodStates {
				// calculate your signature for this round and period
				sigParams := []string{strconv.FormatInt(state.round, 10), strconv.FormatInt(period, 10)}
				sig := SIG(userId, sigParams)

				// get your proposal back out of the map
				proposerCredential := []string{userId, sig.SignedMessage}
				proposerHash := signMessage(proposerCredential)

				// check if you proposed a block for this round and period
				if v, ok := state.periodStates[period].proposedValues[proposerHash]; ok {
					b := state.periodStates[period].valueToBlock[v]

					for p,c := range peerClients {
						go func(c pb.AlgorandClient, p string, b *pb.Block, v string, sig *pb.SIGRet, round int64) {
							ret, err := c.ProposeBlock(context.Background(), &pb.ProposeBlockArgs{Block: b, Credential: sig, Value: v, Round: round, Peer: userId})
							proposeBlockResponseChan <- ProposeBlockResponse{ret: ret, err: err, peer: p}
						}(c, p, b, v, sig, state.round)
					}
				}
			}

		case bcr := <-requestBlockChainResponseChan:
			if bcr.err == nil {
				candidateBlockchain := bcr.ret.Blockchain

				if len(candidateBlockchain) > len(bcs.blockchain) {
					// verify every block in this blockchain
					verified := true

					if verified {
						log.Printf("Verified new Blockchain from peer: %v", bcr.ret.Peer)
						bcs.blockchain = candidateBlockchain

						// Prepare to reenter into Agreement
						state.readyForNextRound = true
						state.round = int64(len(bcs.blockchain))
					}
					
					log.Printf("NewChain: %v", PrettyPrint(bcs.blockchain))
				}
			}
		}
	}
	log.Printf("Strange to arrive here")
}
