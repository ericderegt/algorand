package main

import (
	"log"

	// "github.com/nyu-distributed-systems-fa18/algorand/pb"
)

func runStep2(periodStates map[int64]*PeriodState, requiredVotes int64, currentPeriod int64) string {
  lastPeriod := currentPeriod - 1
  var voteValue string
  votes := int64(0)

  if currentPeriod > 1 {
    for value, numVotes := range periodStates[lastPeriod].nextVotes {
      // find max value
      if numVotes > votes {
        voteValue = value
        votes = numVotes
      }
    }
  }

  log.Printf("voteValue: %v, votes: %v", voteValue, votes)

  if currentPeriod == 1 || (voteValue == "_|_" && votes >= requiredVotes) {
    log.Printf("Period is 1 or vote value is _|_")
    log.Printf("ProposedValues: %#v", periodStates[currentPeriod].proposedValues)
    leadersValue := selectLeader(periodStates[currentPeriod].proposedValues)
    log.Printf("leadersValue: %v", leadersValue)
    return leadersValue
  } else if (voteValue != "_|_" && votes >= requiredVotes) {
    return voteValue
  }
  return ""
}

func runStep3(periodStates map[int64]*PeriodState, requiredVotes int64, currentPeriod int64) string {
  var voteValue string
  votes := int64(0)

  for value, numVotes := range periodStates[currentPeriod].softVotes {
    // find max value
    if numVotes > votes {
      voteValue = value
      votes = numVotes
    }
  }

  log.Printf("voteValue: %v, votes: %v", voteValue, votes)

  if (voteValue != "_|_" && votes >= requiredVotes) {
    return voteValue
  }
  return ""
}

func runStep4(periodStates map[int64]*PeriodState, requiredVotes int64, currentPeriod int64) string {
  lastPeriod := currentPeriod - 1
  var voteValue string
  votes := int64(0)

  if currentPeriod > 1 {
    for value, numVotes := range periodStates[lastPeriod].nextVotes {
      // find max value
      if numVotes > votes {
        voteValue = value
        votes = numVotes
      }
    }
  }

  if periodStates[currentPeriod].myCertVote != "" {
    voteValue = periodStates[currentPeriod].myCertVote
  } else if currentPeriod >= 2 && voteValue == "_|_" && votes >= requiredVotes {
    voteValue = "_|_"
  } else {
    // next vote starting value stpi??
    voteValue = periodStates[currentPeriod].startingValue
  }
  return voteValue
}

func runStep5(periodStates map[int64]*PeriodState, requiredVotes int64, currentPeriod int64) string {
  lastPeriod := currentPeriod - 1

  log.Printf("SoftVotes: %#v", periodStates[currentPeriod].softVotes)

  // if i sees 2t + 1 soft-votes for some value v != ⊥ for period p, then i next-votes v.
  var voteValue string
  votes := int64(0)

  for value, numVotes := range periodStates[currentPeriod].softVotes {
    // find max value
    if numVotes > votes {
      voteValue = value
      votes = numVotes
    }
  }

  if voteValue != "_|_" && votes >= requiredVotes {
    return voteValue
  } 

  // If p ≥ 2 AND i sees 2t+ 1 next-votes for ⊥ for period p−1 AND i has not certified in period p , then i next-votes _|_
  if currentPeriod > 1 {
    voteValue = ""
    votes = int64(0)

    for value, numVotes := range periodStates[lastPeriod].nextVotes {
      if numVotes > votes {
        voteValue = value
        votes = numVotes
      }
    }

    if voteValue == "_|_" && votes >= requiredVotes && periodStates[currentPeriod].myCertVote == "" {
      return "_|_"
    }
  }

  return ""
}

// Returns value if consensus has chosen a block, otherwise empty string
func checkHaltingCondition(periodStates map[int64]*PeriodState, requiredVotes int64) string {

  // check for required cert votes in any period
  for currentPeriod,_ := range periodStates {
    var voteValue string
    votes := int64(0)

    for value, numVotes := range periodStates[currentPeriod].certVotes {
      // find max value
      if numVotes > votes {
        voteValue = value
        votes = numVotes
      }
    }

    if (votes >= requiredVotes) {
      return voteValue
    }
  }
  return ""
}
