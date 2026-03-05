package web

import (
	"embed"
	"html/template"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mrAboalfazl/dnstt-manager/config"
	"github.com/mrAboalfazl/dnstt-manager/service"
	"github.com/mrAboalfazl/dnstt-manager/sshserver"
)

//go:embed templates/*
var templateFS embed.FS

var sshSrv *sshserver.SSHServer

func SetSSHServer(s *sshserver.SSHServer) {
	sshSrv = s
}

func SetupRoutes(r *gin.Engine) {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}
	tmpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"))
	r.SetHTMLTemplate(tmpl)

	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/dashboard")
	})

	r.GET("/login", loginPage)
	r.POST("/login", loginSubmit)
	r.GET("/logout", logout)

	admin := r.Group("/")
	admin.Use(webAuth())
	{
		admin.GET("/dashboard", dashboardPage)
		admin.GET("/users", usersPage)
		admin.GET("/users/new", userFormPage)
		admin.GET("/users/:id/edit", userEditPage)
		admin.GET("/settings", settingsPage)
	}
}

func webAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		session, err := c.Cookie("session")
		if err != nil || session != config.Get().APIKey {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func loginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"error": c.Query("error"),
	})
}

func loginSubmit(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	cfg := config.Get()
	if username == cfg.AdminUser && password == cfg.AdminPass {
		c.SetCookie("session", cfg.APIKey, 86400, "/", "", false, true)
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	c.Redirect(http.StatusFound, "/login?error=Invalid+credentials")
}

func logout(c *gin.Context) {
	c.SetCookie("session", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

func dashboardPage(c *gin.Context) {
	stats := service.GetStats()
	dnsttStatus := service.GetDNSTTService().Status()

	onlineUsers := 0
	if sshSrv != nil {
		onlineUsers = sshSrv.GetOnlineUsersCount()
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"stats":       stats,
		"dnstt":       dnsttStatus,
		"onlineUsers": onlineUsers,
		"apiKey":      config.Get().APIKey,
	})
}

func usersPage(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	search := c.Query("search")
	status := c.Query("status")

	params := service.UserListParams{
		Page:    page,
		PerPage: 20,
		Search:  search,
		Status:  status,
	}

	result, _ := service.ListUsers(params)

	c.HTML(http.StatusOK, "users.html", gin.H{
		"result": result,
		"search": search,
		"status": status,
		"apiKey": config.Get().APIKey,
	})
}

func userFormPage(c *gin.Context) {
	c.HTML(http.StatusOK, "user_form.html", gin.H{
		"user":   nil,
		"isEdit": false,
		"apiKey": config.Get().APIKey,
	})
}

func userEditPage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.Redirect(http.StatusFound, "/users")
		return
	}

	user, err := service.GetUser(uint(id))
	if err != nil {
		c.Redirect(http.StatusFound, "/users")
		return
	}

	c.HTML(http.StatusOK, "user_form.html", gin.H{
		"user":   user.ToResponse(),
		"isEdit": true,
		"apiKey": config.Get().APIKey,
	})
}

func settingsPage(c *gin.Context) {
	cfg := config.Get()
	dnsttStatus := service.GetDNSTTService().Status()

	c.HTML(http.StatusOK, "settings.html", gin.H{
		"config": cfg,
		"dnstt":  dnsttStatus,
		"apiKey": cfg.APIKey,
	})
}
