package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/mrAboalfazl/dnstt-manager/api"
	"github.com/mrAboalfazl/dnstt-manager/config"
	"github.com/mrAboalfazl/dnstt-manager/database"
	"github.com/mrAboalfazl/dnstt-manager/service"
	"github.com/mrAboalfazl/dnstt-manager/sshserver"
	"github.com/mrAboalfazl/dnstt-manager/web"
)

func main() {
	configFile := flag.String("config", "", "Path to config file")
	showConfig := flag.Bool("show-config", false, "Print current config and exit")
	flag.Parse()

	// Load config
	cfg := config.Get()

	if *configFile != "" {
		if err := config.LoadFromFile(*configFile); err != nil {
			log.Printf("Warning: could not load config file: %v", err)
		}
	} else {
		// Try default config location
		defaultConfig := filepath.Join(cfg.HostKeyDir, "config.json")
		if _, err := os.Stat(defaultConfig); err == nil {
			config.LoadFromFile(defaultConfig)
		}
	}

	if *showConfig {
		fmt.Printf("API Key: %s\n", cfg.APIKey)
		fmt.Printf("HTTP Port: %d\n", cfg.HTTPPort)
		fmt.Printf("SSH Port: %d\n", cfg.SSHPort)
		fmt.Printf("DNSTT Domain: %s\n", cfg.DNSTTDomain)
		fmt.Printf("DNSTT Port: %d\n", cfg.DNSTTPort)
		fmt.Printf("DB Path: %s\n", cfg.DBPath)
		fmt.Printf("Admin User: %s\n", cfg.AdminUser)
		return
	}

	// Initialize database
	if err := database.Init(cfg.DBPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	log.Println("Database initialized")

	// Start monitor
	service.StartMonitor(cfg.MonitorInterval)

	// Start SSH server
	sshSrv := sshserver.New(cfg.SSHPort, cfg.HostKeyDir)
	go func() {
		if err := sshSrv.Start(); err != nil {
			log.Printf("SSH server error: %v", err)
		}
	}()

	// Share SSH server reference with handlers
	api.SetSSHServer(sshSrv)
	web.SetSSHServer(sshSrv)

	// Setup HTTP server
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// Setup API routes
	api.SetupRouter(r)

	// Setup web dashboard routes
	web.SetupRoutes(r)

	// Print startup info
	log.Println("===========================================")
	log.Println("  DNSTT Manager Started")
	log.Println("===========================================")
	log.Printf("  HTTP:  http://0.0.0.0:%d", cfg.HTTPPort)
	log.Printf("  SSH:   0.0.0.0:%d", cfg.SSHPort)
	log.Printf("  API Key: %s", cfg.APIKey)
	log.Printf("  Admin: %s / %s", cfg.AdminUser, cfg.AdminPass)
	log.Println("===========================================")

	// Save config for first run
	configPath := filepath.Join(cfg.HostKeyDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config.SaveToFile(configPath)
		log.Printf("Config saved to %s", configPath)
	}

	// Handle shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down...")
		sshSrv.Stop()
		service.GetDNSTTService().Stop()
		database.Close()
		os.Exit(0)
	}()

	// Start HTTP server
	if err := r.Run(fmt.Sprintf(":%d", cfg.HTTPPort)); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
