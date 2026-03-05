package service

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/mrAboalfazl/dnstt-manager/config"
)

type DNSTTService struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
}

var dnsttSvc = &DNSTTService{}

func GetDNSTTService() *DNSTTService {
	return dnsttSvc
}

func (d *DNSTTService) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return fmt.Errorf("dnstt-server is already running")
	}

	cfg := config.Get()

	if _, err := os.Stat(cfg.DNSTTBinary); os.IsNotExist(err) {
		return fmt.Errorf("dnstt-server binary not found at %s", cfg.DNSTTBinary)
	}
	if _, err := os.Stat(cfg.PrivKeyFile); os.IsNotExist(err) {
		return fmt.Errorf("private key not found at %s", cfg.PrivKeyFile)
	}

	d.cmd = exec.Command(
		cfg.DNSTTBinary,
		"-udp", fmt.Sprintf(":%d", cfg.DNSTTPort),
		"-privkey-file", cfg.PrivKeyFile,
		"-mtu", fmt.Sprintf("%d", cfg.MTU),
		cfg.DNSTTDomain,
		cfg.ForwardAddr,
	)

	d.cmd.Stdout = os.Stdout
	d.cmd.Stderr = os.Stderr

	if err := d.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dnstt-server: %v", err)
	}

	d.running = true

	go func() {
		d.cmd.Wait()
		d.mu.Lock()
		d.running = false
		d.mu.Unlock()
	}()

	return nil
}

func (d *DNSTTService) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running || d.cmd == nil || d.cmd.Process == nil {
		return fmt.Errorf("dnstt-server is not running")
	}

	if err := d.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to stop dnstt-server: %v", err)
	}

	d.running = false
	return nil
}

func (d *DNSTTService) Restart() error {
	d.mu.Lock()
	wasRunning := d.running
	d.mu.Unlock()

	if wasRunning {
		if err := d.Stop(); err != nil {
			return err
		}
	}
	return d.Start()
}

func (d *DNSTTService) IsRunning() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running
}

func (d *DNSTTService) Status() map[string]interface{} {
	cfg := config.Get()
	pubkey := ""
	if data, err := os.ReadFile(cfg.PubKeyFile); err == nil {
		pubkey = strings.TrimSpace(string(data))
	}

	return map[string]interface{}{
		"running":      d.IsRunning(),
		"domain":       cfg.DNSTTDomain,
		"port":         cfg.DNSTTPort,
		"mtu":          cfg.MTU,
		"forward_addr": cfg.ForwardAddr,
		"pubkey":       pubkey,
		"server_ip":    cfg.ServerIP,
	}
}

func GenerateKeys() error {
	cfg := config.Get()

	if _, err := os.Stat(cfg.DNSTTBinary); os.IsNotExist(err) {
		return fmt.Errorf("dnstt-server binary not found at %s", cfg.DNSTTBinary)
	}

	cmd := exec.Command(
		cfg.DNSTTBinary,
		"-gen-key",
		"-privkey-file", cfg.PrivKeyFile,
		"-pubkey-file", cfg.PubKeyFile,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("key generation failed: %v - %s", err, string(output))
	}

	return nil
}
