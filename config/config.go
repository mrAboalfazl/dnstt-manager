package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	mu sync.RWMutex `json:"-"`

	// API
	APIKey   string `json:"api_key"`
	HTTPPort int    `json:"http_port"`

	// Admin
	AdminUser string `json:"admin_user"`
	AdminPass string `json:"admin_pass"`

	// SSH Server
	SSHPort    int    `json:"ssh_port"`
	HostKeyDir string `json:"host_key_dir"`

	// DNSTT
	DNSTTBinary  string `json:"dnstt_binary"`
	DNSTTDomain  string `json:"dnstt_domain"`
	DNSTTPort    int    `json:"dnstt_port"`
	PrivKeyFile  string `json:"privkey_file"`
	PubKeyFile   string `json:"pubkey_file"`
	MTU          int    `json:"mtu"`
	ForwardAddr  string `json:"forward_addr"`
	DNSResolver  string `json:"dns_resolver"`
	ServerIP     string `json:"server_ip"`

	// Database
	DBPath string `json:"db_path"`

	// Monitor
	MonitorInterval int `json:"monitor_interval_sec"`
}

var (
	cfg  *Config
	once sync.Once
)

func Get() *Config {
	once.Do(func() {
		cfg = defaultConfig()
	})
	return cfg
}

func defaultConfig() *Config {
	dataDir := getDataDir()
	return &Config{
		APIKey:          generateAPIKey(),
		HTTPPort:        8080,
		AdminUser:       "admin",
		AdminPass:       "admin",
		SSHPort:         2222,
		HostKeyDir:      dataDir,
		DNSTTBinary:     "/usr/local/bin/dnstt-server",
		DNSTTDomain:     "t.example.com",
		DNSTTPort:       5300,
		PrivKeyFile:     filepath.Join(dataDir, "dnstt-server.key"),
		PubKeyFile:      filepath.Join(dataDir, "dnstt-server.pub"),
		MTU:             1232,
		ForwardAddr:     "127.0.0.1:2222",
		DNSResolver:     "https://cloudflare-dns.com/dns-query",
		ServerIP:        "0.0.0.0",
		DBPath:          filepath.Join(dataDir, "dnstt-manager.db"),
		MonitorInterval: 30,
	}
}

func getDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	dir := filepath.Join(home, ".dnstt-manager")
	os.MkdirAll(dir, 0700)
	return dir
}

func generateAPIKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate API key: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func LoadFromFile(path string) error {
	c := Get()
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, c)
}

func SaveToFile(path string) error {
	c := Get()
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (c *Config) GetDomain() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.DNSTTDomain
}

func (c *Config) GetPubKey() (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, err := os.ReadFile(c.PubKeyFile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *Config) Update(updates map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, _ := json.Marshal(c)
	var m map[string]interface{}
	json.Unmarshal(data, &m)

	for k, v := range updates {
		m[k] = v
	}

	newData, _ := json.Marshal(m)
	json.Unmarshal(newData, c)
}
