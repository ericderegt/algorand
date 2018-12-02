package main

import (
	"log"

	// "github.com/nyu-distributed-systems-fa18/algorand/pb"
)

func runStep2(currentPeriod *PeriodState, lastPeriod *PeriodState, requiredVotes int64) string {
  var voteValue string
  votes := int64(0)

  if currentPeriod.period > 1 {
    for value, numVotes := range lastPeriod.nextVotes {
      // find max value
      if numVotes > votes {
        voteValue = value
        votes = numVotes
      }
    }
  }

  log.Printf("voteValue: %v, votes: %v", voteValue, votes)

  if currentPeriod.period == 1 || (voteValue == "_|_" && votes >= requiredVotes) {
    log.Printf("Period is 1 or vote value is _|_")
    log.Printf("ProposedValues: %#v", currentPeriod.proposedValues)
    leadersValue := selectLeader(currentPeriod.proposedValues)
    log.Printf("leadersValue: %v", leadersValue)
    return leadersValue
  } else if (voteValue != "_|_" && votes >= requiredVotes) {
    return voteValue
  }
  return ""
}

func runStep3(currentPeriod *PeriodState, requiredVotes int64) string {
  var voteValue string
  votes := int64(0)

  for value, numVotes := range currentPeriod.softVotes {
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

func runStep4(currentPeriod *PeriodState, lastPeriod *PeriodState, requiredVotes int64) string {
  var voteValue string
  votes := int64(0)

  if currentPeriod.period > 1 {
    for value, numVotes := range lastPeriod.nextVotes {
      // find max value
      if numVotes > votes {
        voteValue = value
        votes = numVotes
      }
    }
  }

  if currentPeriod.myCertVote != "" {
    voteValue = currentPeriod.myCertVote
  } else if currentPeriod.period >= 2 && voteValue == "_|_" && votes > requiredVotes {
    voteValue = "_|_"
  } else {
    // next vote starting value stpi??
    voteValue = currentPeriod.startingValue
  }
  return voteValue
}

func runStep5(currentPeriod *PeriodState, lastPeriod *PeriodState, requiredVotes int64) string {
  // if i sees 2t + 1 soft-votes for some value v 6= ⊥ for period p, then i next-votes v.
  var voteValue string
  votes := int64(0)

  for value, numVotes := range currentPeriod.softVotes {
    // find max value
    if numVotes > votes {
      voteValue = value
      votes = numVotes
    }
  }

  if voteValue != "_|_" && votes > requiredVotes {
    return voteValue
  } 

  // If p ≥ 2 AND i sees 2t+ 1 next-votes for ⊥ for period p−1 AND i has not certified in period p , then i next-votes _|_
  if currentPeriod.period > 1 {
    voteValue = ""
    votes = int64(0)

    for value, numVotes := range lastPeriod.nextVotes {
      if numVotes > votes {
        voteValue = value
        votes = numVotes
      }
    }

    if voteValue == "_|_" && votes > requiredVotes && currentPeriod.myCertVote == "" {
      return "_|_"
    }
  }

  return ""
}

// Returns value if consensus has chosen a block, otherwise empty string
func checkHaltingCondition(currentPeriod *PeriodState, requiredVotes int64) string {
  var voteValue string
  votes := int64(0)

  for value, numVotes := range currentPeriod.certVotes {
    // find max value
    if numVotes > votes {
      voteValue = value
      votes = numVotes
    }
  }

  if (votes >= requiredVotes) {
    return voteValue
  }
  return ""
}
