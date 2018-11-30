package main

import (
	// "log"

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

  if currentPeriod.period == 1 || (voteValue == "_|_" && votes >= requiredVotes) {
    leadersValue := selectLeader(currentPeriod.proposedValues)
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

  if (voteValue == "_|_" && votes >= requiredVotes) {
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
