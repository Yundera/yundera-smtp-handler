package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	versionFlag := flag.Bool("version", false, "show version")
	flag.BoolVar(versionFlag, "v", false, "show version")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("mail-gateway %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	log.Println("=================================================")
	log.Printf("Mail Gateway v%s", version)
	log.Printf("Git Commit: %s", commit)
	log.Printf("Build Date: %s", date)
	log.Println("=================================================")

	// Get configuration from environment
	smtpPort := os.Getenv("SMTP_PORT")
	if smtpPort == "" {
		smtpPort = "587"
	}

	// RELAY_CREDENTIAL is the credential the relay verifies. Two forms are accepted:
	//   - explicit (Yundera): an opaque bearer token (a JWT); pair it with RELAY_ENDPOINT_URL.
	//   - provider mode (nsl): the PCS provider string "backend_url,userid,signature".
	relayCredential := os.Getenv("RELAY_CREDENTIAL")
	if relayCredential == "" {
		log.Fatal("❌ ERROR: RELAY_CREDENTIAL environment variable is required")
	}

	// RELAY_ENDPOINT_URL is the base URL of the relay backend's email API. Optional in
	// provider mode (derived from the credential), required otherwise.
	relayEndpointURL := os.Getenv("RELAY_ENDPOINT_URL")

	// Provider mode: a 3-field "backend_url,userid,signature" credential. (A JWT has no
	// commas, so it never matches.) Derive the endpoint and the bearer the backend expects.
	if parts := strings.Split(relayCredential, ","); len(parts) == 3 {
		backendURL, userID, signature := parts[0], parts[1], parts[2]
		if relayEndpointURL == "" {
			// The mesh-router backend API lives under /router/api (same prefix agent/tunnel use).
			relayEndpointURL = backendURL + "/router/api"
		}
		relayCredential = userID + ":" + signature
	}

	if relayEndpointURL == "" {
		log.Fatal("❌ ERROR: RELAY_ENDPOINT_URL environment variable is required")
	}
	// Strip /user suffix if present (some deployments seed the user-API URL)
	relayEndpointURL = strings.TrimSuffix(relayEndpointURL, "/user")

	// Start SMTP server
	log.Printf("Starting SMTP server on port %s...", smtpPort)
	log.Printf("Relay endpoint URL: %s", relayEndpointURL)

	if err := StartSMTPServer(smtpPort, relayEndpointURL, relayCredential); err != nil {
		log.Fatalf("❌ Failed to start SMTP server: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down SMTP server...")
}
