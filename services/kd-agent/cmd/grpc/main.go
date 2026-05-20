// Package main is the entry point for the kd-agent gRPC client process.
// It reads configuration from environment variables, establishes an mTLS
// connection to kd-gateway, and maintains a bidirectional stream with
// exponential-backoff reconnects.
package main

import (
	"log"
	"os"

	"github.com/kubediscovery/kd-agent/internal/core/stream"
)

func main() {
	callerID := os.Getenv("AGENT_ID")
	if callerID == "" {
		log.Fatal("AGENT_ID must be set and non-empty")
	}

	log.Printf("kd-agent starting: caller_id=%s retrier=%+v", callerID, stream.DefaultRetrier)
	// Full UberFX wiring and gRPC client setup are introduced in task 5.9.
	// This placeholder ensures the binary compiles and the AGENT_ID guard is
	// exercised.
}
