package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mrAboalfazl/dnstt-manager/config"
	"github.com/mrAboalfazl/dnstt-manager/service"
	"github.com/mrAboalfazl/dnstt-manager/sshserver"
)

var sshSrv *sshserver.SSHServer

func SetSSHServer(s *sshserver.SSHServer) {
	sshSrv = s
}

func GetServerStatus(c *gin.Context) {
	dnsttStatus := service.GetDNSTTService().Status()
	userStats := service.GetStats()

	onlineUsers := 0
	activeConns := 0
	if sshSrv != nil {
		onlineUsers = sshSrv.GetOnlineUsersCount()
		activeConns = len(sshSrv.GetActiveConnections())
	}

	c.JSON(http.StatusOK, gin.H{
		"dnstt":        dnsttStatus,
		"users":        userStats,
		"online_users": onlineUsers,
		"active_connections": activeConns,
	})
}

func StartServer(c *gin.Context) {
	if err := service.GetDNSTTService().Start(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "dnstt-server started"})
}

func StopServer(c *gin.Context) {
	if err := service.GetDNSTTService().Stop(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "dnstt-server stopped"})
}

func RestartServer(c *gin.Context) {
	if err := service.GetDNSTTService().Restart(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "dnstt-server restarted"})
}

func GetServerConfig(c *gin.Context) {
	cfg := config.Get()
	c.JSON(http.StatusOK, gin.H{
		"dnstt_domain":  cfg.DNSTTDomain,
		"dnstt_port":    cfg.DNSTTPort,
		"ssh_port":      cfg.SSHPort,
		"http_port":     cfg.HTTPPort,
		"mtu":           cfg.MTU,
		"forward_addr":  cfg.ForwardAddr,
		"dns_resolver":  cfg.DNSResolver,
		"server_ip":     cfg.ServerIP,
		"dnstt_binary":  cfg.DNSTTBinary,
		"privkey_file":  cfg.PrivKeyFile,
		"pubkey_file":   cfg.PubKeyFile,
	})
}

func UpdateServerConfig(c *gin.Context) {
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Don't allow updating sensitive fields via API
	delete(updates, "api_key")
	delete(updates, "admin_user")
	delete(updates, "admin_pass")
	delete(updates, "db_path")

	config.Get().Update(updates)

	// Save to file
	configPath := config.Get().HostKeyDir + "/config.json"
	config.SaveToFile(configPath)

	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}

func GetActiveConnections(c *gin.Context) {
	if sshSrv == nil {
		c.JSON(http.StatusOK, gin.H{"connections": []interface{}{}})
		return
	}

	conns := sshSrv.GetActiveConnections()
	c.JSON(http.StatusOK, gin.H{
		"connections": conns,
		"total":       len(conns),
	})
}

func KickUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username required"})
		return
	}

	if sshSrv != nil {
		sshSrv.KickUser(username)
	}

	c.JSON(http.StatusOK, gin.H{"message": "user kicked", "username": username})
}
