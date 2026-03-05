package sshserver

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/mrAboalfazl/dnstt-manager/service"
)

type SSHServer struct {
	port        int
	hostKeyPath string
	server      *ssh.Server
	connections sync.Map // map[string]*int32 (username -> active count)
	connDetails sync.Map // map[string]*[]ConnInfo
	mu          sync.Mutex
}

type ConnInfo struct {
	Username    string    `json:"username"`
	RemoteAddr  string    `json:"remote_addr"`
	ConnectedAt time.Time `json:"connected_at"`
}

func New(port int, hostKeyPath string) *SSHServer {
	return &SSHServer{
		port:        port,
		hostKeyPath: hostKeyPath,
	}
}

func (s *SSHServer) Start() error {
	s.server = &ssh.Server{
		Addr: fmt.Sprintf(":%d", s.port),
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return s.authenticate(ctx, password)
		},
		LocalPortForwardingCallback: func(ctx ssh.Context, dhost string, dport uint32) bool {
			log.Printf("[SSH] Port forward request from %s to %s:%d", ctx.User(), dhost, dport)
			return true
		},
		Handler: func(sess ssh.Session) {
			username := sess.User()
			log.Printf("[SSH] Session opened for user: %s from %s", username, sess.RemoteAddr())
			<-sess.Context().Done()
			log.Printf("[SSH] Session closed for user: %s", username)
		},
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"session":      ssh.DefaultSessionHandler,
			"direct-tcpip": ssh.DirectTCPIPHandler,
		},
	}

	// Generate or load host key
	pemData, err := ensureHostKey(filepath.Join(s.hostKeyPath, "ssh_host_key"))
	if err != nil {
		return fmt.Errorf("failed to setup host key: %v", err)
	}

	s.server.SetOption(ssh.HostKeyPEM(pemData))

	log.Printf("[SSH] Starting SSH server on port %d", s.port)
	return s.server.ListenAndServe()
}

func (s *SSHServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

func (s *SSHServer) authenticate(ctx ssh.Context, password string) bool {
	username := ctx.User()

	user, err := service.GetUserByUsername(username)
	if err != nil {
		log.Printf("[SSH] Auth failed - user not found: %s", username)
		return false
	}

	if !user.CheckPassword(password) {
		log.Printf("[SSH] Auth failed - wrong password: %s", username)
		return false
	}

	if !user.IsActive() {
		log.Printf("[SSH] Auth failed - user not active (status: %s): %s", user.Status(), username)
		return false
	}

	count := s.getConnectionCount(username)
	if count >= int32(user.MaxConnections) {
		log.Printf("[SSH] Auth failed - connection limit reached (%d/%d): %s", count, user.MaxConnections, username)
		return false
	}

	s.incrementConnection(username)
	service.UpdateLastConnection(username)

	s.addConnInfo(username, ConnInfo{
		Username:    username,
		RemoteAddr:  ctx.RemoteAddr().String(),
		ConnectedAt: time.Now(),
	})

	go func() {
		<-ctx.Done()
		s.decrementConnection(username)
		s.removeConnInfo(username, ctx.RemoteAddr().String())
		log.Printf("[SSH] Connection closed for user: %s (active: %d)", username, s.getConnectionCount(username))
	}()

	log.Printf("[SSH] Auth success: %s (active: %d/%d)", username, s.getConnectionCount(username), user.MaxConnections)
	return true
}

func (s *SSHServer) getConnectionCount(username string) int32 {
	if val, ok := s.connections.Load(username); ok {
		return atomic.LoadInt32(val.(*int32))
	}
	return 0
}

func (s *SSHServer) incrementConnection(username string) {
	val, _ := s.connections.LoadOrStore(username, new(int32))
	atomic.AddInt32(val.(*int32), 1)
}

func (s *SSHServer) decrementConnection(username string) {
	if val, ok := s.connections.Load(username); ok {
		newVal := atomic.AddInt32(val.(*int32), -1)
		if newVal <= 0 {
			s.connections.Delete(username)
		}
	}
}

func (s *SSHServer) addConnInfo(username string, info ConnInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	val, _ := s.connDetails.LoadOrStore(username, &[]ConnInfo{})
	list := val.(*[]ConnInfo)
	*list = append(*list, info)
}

func (s *SSHServer) removeConnInfo(username string, remoteAddr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if val, ok := s.connDetails.Load(username); ok {
		list := val.(*[]ConnInfo)
		newList := make([]ConnInfo, 0, len(*list))
		for _, c := range *list {
			if c.RemoteAddr != remoteAddr {
				newList = append(newList, c)
			}
		}
		if len(newList) == 0 {
			s.connDetails.Delete(username)
		} else {
			*list = newList
		}
	}
}

func (s *SSHServer) GetActiveConnections() []ConnInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	var all []ConnInfo
	s.connDetails.Range(func(key, value interface{}) bool {
		list := value.(*[]ConnInfo)
		all = append(all, *list...)
		return true
	})
	return all
}

func (s *SSHServer) GetOnlineUsersCount() int {
	count := 0
	s.connections.Range(func(key, value interface{}) bool {
		if atomic.LoadInt32(value.(*int32)) > 0 {
			count++
		}
		return true
	})
	return count
}

func (s *SSHServer) KickUser(username string) {
	s.connections.Delete(username)
	s.mu.Lock()
	s.connDetails.Delete(username)
	s.mu.Unlock()
	log.Printf("[SSH] Kicked user: %s", username)
}

func ensureHostKey(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}

	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0700)

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}

	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	})

	if err := os.WriteFile(path, pemData, 0600); err != nil {
		return nil, err
	}

	return pemData, nil
}
