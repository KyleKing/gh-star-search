package main

import (
	"fmt"
	"log"
	"os"
)

// Config holds application configuration
type Config struct {
	Port     int    `json:"port"`
	Database string `json:"database"`
	Debug    bool   `json:"debug"`
}

// Server represents the main application server
type Server struct {
	config *Config
	logger *log.Logger
}

// NewServer creates a new server instance with the given configuration
func NewServer(config *Config) *Server {
	return &Server{
		config: config,
		logger: log.New(os.Stdout, "[SERVER] ", log.LstdFlags),
	}
}

// Start begins serving requests on the configured port
func (s *Server) Start() error {
	s.logger.Printf("Starting server on port %d", s.config.Port)

	if s.config.Debug {
		s.logger.Println("Debug mode enabled")
	}

	// Server startup logic would go here
	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	s.logger.Println("Shutting down server")
	// Cleanup logic would go here
	return nil
}

// handleRequest processes incoming requests
func (s *Server) handleRequest(req string) string {
	if s.config.Debug {
		s.logger.Printf("Processing request: %s", req)
	}

	// Request processing logic
	return fmt.Sprintf("Processed: %s", req)
}

// validateConfig ensures the configuration is valid
func validateConfig(config *Config) error {
	if config.Port <= 0 {
		return fmt.Errorf("invalid port: %d", config.Port)
	}

	if config.Database == "" {
		return fmt.Errorf("database connection string is required")
	}

	return nil
}

func main() {
	config := &Config{
		Port:     8080,
		Database: "localhost:5432",
		Debug:    true,
	}

	if err := validateConfig(config); err != nil {
		log.Fatal(err)
	}

	server := NewServer(config)

	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
