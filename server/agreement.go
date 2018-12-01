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
  return ""
}

func runStep5(currentPeriod *PeriodState, lastPeriod *PeriodState, requiredVotes int64) string {
  return ""
}
