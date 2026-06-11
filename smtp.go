package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

const (
	MaxEmailSize = 10 * 1024 * 1024 // 10MB
)

// EmailAttachment represents an inline or file attachment
type EmailAttachment struct {
	Filename  string `json:"filename"`
	Content   string `json:"content"`               // base64 encoded
	MimeType  string `json:"type"`                  // e.g., "image/png", "image/svg+xml"
	ContentID string `json:"content_id,omitempty"` // for inline images (cid: references)
}

// EmailRequest represents the request to the relay email API
type EmailRequest struct {
	To          string            `json:"to"`
	Subject     string            `json:"subject"`
	Text        string            `json:"text"`
	HTML        string            `json:"html,omitempty"`
	AppName     string            `json:"appName"`
	Attachments []EmailAttachment `json:"attachments,omitempty"`
}

// SMTPBackend implements SMTP server backend
type SMTPBackend struct {
	relayCredential        string
	relayEndpointURL string
}

// SMTPSession represents an SMTP session
type SMTPSession struct {
	backend  *SMTPBackend
	appName  string
	from     string
	to       []string
	authUser string
}

// NewSMTPBackend creates a new SMTP backend
func NewSMTPBackend(relayEndpointURL, relayCredential string) *SMTPBackend {
	return &SMTPBackend{
		relayEndpointURL: relayEndpointURL,
		relayCredential:        relayCredential,
	}
}

// NewSession creates a new SMTP session
func (b *SMTPBackend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &SMTPSession{
		backend: b,
	}, nil
}

// AuthMechanisms returns supported authentication mechanisms
func (s *SMTPSession) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

// Auth creates a SASL server for authentication
// NOTE: Authentication is relaxed since this runs in a private Docker network
// Apps can connect without credentials since network isolation provides security
func (s *SMTPSession) Auth(mech string) (sasl.Server, error) {
	return sasl.NewPlainServer(func(identity, username, password string) error {
		// Accept any credentials - network isolation is the security boundary
		log.Printf("SMTP connection from app: %s", username)
		s.authUser = username
		s.appName = sanitizeAppName(username)
		return nil
	}), nil
}

// Mail sets the sender
func (s *SMTPSession) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

// Rcpt adds a recipient
func (s *SMTPSession) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	return nil
}

// Data handles the email data and forwards to the relay API
func (s *SMTPSession) Data(r io.Reader) error {
	// Read email data
	data, err := io.ReadAll(io.LimitReader(r, MaxEmailSize))
	if err != nil {
		return err
	}

	// Parse email (now includes attachments)
	subject, text, html, attachments := parseEmail(string(data))

	// Get first recipient
	if len(s.to) == 0 {
		return fmt.Errorf("no recipients specified")
	}
	recipientEmail := s.to[0]

	// Determine app name from sender address, fall back to auth username
	var appName string
	if s.from != "" {
		parts := strings.Split(s.from, "@")
		if len(parts) > 0 {
			appName = sanitizeAppName(parts[0])
		}
	}
	if appName == "" {
		appName = s.appName
	}
	if appName == "" {
		appName = "app"
	}

	log.Printf("Processing email from app '%s' to %s (attachments: %d)", appName, recipientEmail, len(attachments))

	// Forward to relay email API
	err = s.forwardToAPI(recipientEmail, subject, text, html, appName, attachments)
	if err != nil {
		log.Printf("Failed to forward email to API: %v", err)
		return err
	}

	log.Printf("Email forwarded successfully from %s", appName)
	return nil
}

// Reset resets the session
func (s *SMTPSession) Reset() {
	s.from = ""
	s.to = nil
}

// Logout ends the session
func (s *SMTPSession) Logout() error {
	return nil
}

// forwardToAPI sends the email to the relay email API via HTTP
func (s *SMTPSession) forwardToAPI(recipientEmail, subject, text, html, appName string, attachments []EmailAttachment) error {
	// Create email request
	emailReq := EmailRequest{
		To:          recipientEmail,
		Subject:     subject,
		Text:        text,
		HTML:        html,
		AppName:     appName,
		Attachments: attachments,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(emailReq)
	if err != nil {
		return fmt.Errorf("failed to marshal email request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/email/send", s.backend.relayEndpointURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.backend.relayCredential))

	// Send request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned error status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Email successfully forwarded to API for recipient %s", recipientEmail)
	return nil
}

// sanitizeAppName sanitizes the app name
func sanitizeAppName(name string) string {
	// Only allow alphanumeric and hyphens, lowercase, max 20 chars
	reg := regexp.MustCompile("[^a-z0-9-]")
	name = strings.ToLower(name)
	name = reg.ReplaceAllString(name, "")
	if len(name) > 20 {
		name = name[:20]
	}
	if name == "" {
		name = "app"
	}
	return name
}

// parseEmail extracts subject, text, HTML, and attachments from email data using go-message library
func parseEmail(data string) (subject, text, html string, attachments []EmailAttachment) {
	reader := strings.NewReader(data)
	entity, err := message.Read(reader)
	if err != nil {
		log.Printf("Failed to parse email: %v", err)
		return "No Subject", data, "", nil
	}

	header := entity.Header
	if subjectHeader := header.Get("Subject"); subjectHeader != "" {
		subject = subjectHeader
	} else {
		subject = "No Subject"
	}

	text, html, attachments = extractBodyParts(entity)

	if text == "" && html != "" {
		text = html
	}

	return subject, strings.TrimSpace(text), strings.TrimSpace(html), attachments
}

// extractBodyParts recursively extracts text, HTML, and attachments from a MIME message
func extractBodyParts(entity *message.Entity) (text, html string, attachments []EmailAttachment) {
	mediaType, params, err := entity.Header.ContentType()
	if err != nil {
		body, _ := io.ReadAll(entity.Body)
		return string(body), "", nil
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		mr := entity.MultipartReader()
		if mr == nil {
			return "", "", nil
		}

		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("Error reading multipart: %v", err)
				break
			}

			partMediaType, _, _ := part.Header.ContentType()
			contentID := part.Header.Get("Content-Id")
			contentDisposition := part.Header.Get("Content-Disposition")

			// Extract inline images as attachments (preserve Content-ID for SendGrid)
			if strings.HasPrefix(partMediaType, "image/") && contentID != "" {
				contentID = strings.Trim(contentID, "<>")
				body, err := io.ReadAll(part.Body)
				if err == nil {
					// Extract filename from Content-Disposition or use Content-ID
					filename := contentID
					if contentDisposition != "" {
						// Try to extract filename from Content-Disposition
						filenameRegex := regexp.MustCompile(`filename="?([^";\s]+)"?`)
						if matches := filenameRegex.FindStringSubmatch(contentDisposition); len(matches) > 1 {
							filename = matches[1]
						}
					}

					attachments = append(attachments, EmailAttachment{
						Filename:  filename,
						Content:   base64.StdEncoding.EncodeToString(body),
						MimeType:  partMediaType,
						ContentID: contentID,
					})
					log.Printf("Extracted inline image attachment: %s (Content-ID: %s)", filename, contentID)
				}
			} else if contentDisposition == "" || !strings.HasPrefix(contentDisposition, "attachment") {
				// Recursively extract text/html from nested parts
				partText, partHTML, partAttachments := extractBodyParts(part)
				if partText != "" && text == "" {
					text = partText
				}
				if partHTML != "" {
					html = partHTML
				}
				attachments = append(attachments, partAttachments...)
			}
		}
	} else if mediaType == "text/plain" {
		body, err := io.ReadAll(entity.Body)
		if err == nil {
			text = string(body)
		}
	} else if mediaType == "text/html" {
		body, err := io.ReadAll(entity.Body)
		if err == nil {
			html = string(body)
		}
	} else {
		charset := params["charset"]
		if charset == "" {
			charset = "utf-8"
		}
		body, err := io.ReadAll(entity.Body)
		if err == nil {
			text = string(body)
		}
	}

	return text, html, attachments
}

// StartSMTPServer starts the SMTP server
func StartSMTPServer(port, relayEndpointURL, relayCredential string) error {
	if port == "" {
		return errors.New("SMTP port is required")
	}

	if relayEndpointURL == "" {
		return errors.New("relay endpoint URL is required")
	}

	if relayCredential == "" {
		return errors.New("relay credential is required")
	}

	backend := NewSMTPBackend(relayEndpointURL, relayCredential)

	server := smtp.NewServer(backend)
	server.Addr = ":" + port
	server.Domain = "smtp.local"
	server.AllowInsecureAuth = true // OK within private Docker network
	server.MaxMessageBytes = MaxEmailSize
	server.MaxRecipients = 50
	server.ReadTimeout = 30 * time.Second
	server.WriteTimeout = 30 * time.Second

	// Check if port is already in use
	testLn, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return fmt.Errorf("port %s is already in use or cannot bind: %w", port, err)
	}
	testLn.Close()

	// Listen on all interfaces (Docker network)
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	log.Printf("✓ SMTP Server started on port %s", port)
	log.Printf("✓ Forwarding emails to relay backend at %s", relayEndpointURL)
	log.Printf("✓ Ready to accept SMTP connections from apps")

	// Start serving in a goroutine
	go func() {
		if err := server.Serve(ln); err != nil {
			log.Printf("SMTP server error: %v", err)
		}
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	log.Println("✓ SMTP server is running")
	return nil
}
