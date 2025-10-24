package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
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
		fmt.Printf("yundera-smtp-handler %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	log.Println("=================================================")
	log.Printf("Yundera SMTP Handler v%s", version)
	log.Printf("Git Commit: %s", commit)
	log.Printf("Build Date: %s", date)
	log.Println("=================================================")

	// Get configuration from environment
	smtpPort := os.Getenv("SMTP_PORT")
	if smtpPort == "" {
		smtpPort = "587"
	}

	orchestratorURL := os.Getenv("ORCHESTRATOR_URL")
	if orchestratorURL == "" {
		orchestratorURL = "https://nasselle.com/service/pcs"
		log.Printf("⚠ ORCHESTRATOR_URL not set, using default: %s", orchestratorURL)
	}

	userJWT := os.Getenv("USER_JWT")
	if userJWT == "" {
		log.Fatal("❌ ERROR: USER_JWT environment variable is required")
	}

	// Start SMTP server
	log.Printf("Starting SMTP server on port %s...", smtpPort)
	log.Printf("Orchestrator URL: %s", orchestratorURL)

	if err := StartSMTPServer(smtpPort, orchestratorURL, userJWT); err != nil {
		log.Fatalf("❌ Failed to start SMTP server: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down SMTP server...")
}
