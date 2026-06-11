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

	// RELAY_ENDPOINT_URL is the base URL of the relay backend's email API.
	relayEndpointURL := os.Getenv("RELAY_ENDPOINT_URL")
	if relayEndpointURL == "" {
		log.Fatal("❌ ERROR: RELAY_ENDPOINT_URL environment variable is required")
	}
	// Strip /user suffix if present (some deployments seed the user-API URL)
	relayEndpointURL = strings.TrimSuffix(relayEndpointURL, "/user")

	// RELAY_CREDENTIAL is the opaque bearer credential the relay verifies — a JWT on
	// Yundera, a userid:signature derived from the provider string on nsl. The gateway
	// never parses it.
	relayCredential := os.Getenv("RELAY_CREDENTIAL")
	if relayCredential == "" {
		log.Fatal("❌ ERROR: RELAY_CREDENTIAL environment variable is required")
	}

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
