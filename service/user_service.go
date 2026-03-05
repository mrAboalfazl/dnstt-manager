package service

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/mrAboalfazl/dnstt-manager/database"
	"github.com/mrAboalfazl/dnstt-manager/models"
	"gorm.io/gorm"
)

type CreateUserRequest struct {
	Username       string  `json:"username" binding:"required,min=3,max=32"`
	Password       string  `json:"password" binding:"required,min=4"`
	MaxConnections int     `json:"max_connections" binding:"required,min=1"`
	TrafficLimitGB float64 `json:"traffic_limit_gb"` // 0 = unlimited
	ExpiresAt      string  `json:"expires_at" binding:"required"`
}

type CreateTestUserRequest struct {
	MaxConnections int     `json:"max_connections"`
	TrafficLimitGB float64 `json:"traffic_limit_gb"`
	ExpiresAt      string  `json:"expires_at" binding:"required"`
}

type UpdateUserRequest struct {
	Password       *string  `json:"password"`
	MaxConnections *int     `json:"max_connections"`
	TrafficLimitGB *float64 `json:"traffic_limit_gb"`
	ExpiresAt      *string  `json:"expires_at"`
	Enabled        *bool    `json:"enabled"`
}

type UserListParams struct {
	Page    int
	PerPage int
	Status  string
	IsTest  *bool
	Search  string
}

type UserListResult struct {
	Users      []models.UserResponse `json:"users"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	PerPage    int                   `json:"per_page"`
	TotalPages int                   `json:"total_pages"`
}

func CreateUser(req CreateUserRequest) (*models.User, error) {
	expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		expiresAt, err = time.Parse("2006-01-02", req.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("invalid expires_at format, use RFC3339 or YYYY-MM-DD")
		}
		expiresAt = expiresAt.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	}

	var existing models.User
	if err := database.DB.Where("username = ?", req.Username).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("username '%s' already exists", req.Username)
	}

	user := &models.User{
		Username:       req.Username,
		MaxConnections: req.MaxConnections,
		TrafficLimit:   int64(req.TrafficLimitGB * 1024 * 1024 * 1024),
		ExpiresAt:      expiresAt,
		Enabled:        true,
		IsTest:         false,
	}

	if err := user.SetPassword(req.Password); err != nil {
		return nil, err
	}

	if err := database.DB.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func CreateTestUser(req CreateTestUserRequest) (*models.User, string, error) {
	expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		expiresAt, err = time.Parse("2006-01-02", req.ExpiresAt)
		if err != nil {
			return nil, "", fmt.Errorf("invalid expires_at format")
		}
		expiresAt = expiresAt.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	}

	username := fmt.Sprintf("test_%s", randomString(6))
	password := randomString(10)

	maxConn := req.MaxConnections
	if maxConn <= 0 {
		maxConn = 1
	}

	user := &models.User{
		Username:       username,
		MaxConnections: maxConn,
		TrafficLimit:   int64(req.TrafficLimitGB * 1024 * 1024 * 1024),
		ExpiresAt:      expiresAt,
		Enabled:        true,
		IsTest:         true,
	}

	if err := user.SetPassword(password); err != nil {
		return nil, "", err
	}

	if err := database.DB.Create(user).Error; err != nil {
		return nil, "", err
	}
	return user, password, nil
}

func GetUser(id uint) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func ListUsers(params UserListParams) (*UserListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PerPage <= 0 {
		params.PerPage = 20
	}
	if params.PerPage > 100 {
		params.PerPage = 100
	}

	query := database.DB.Model(&models.User{})

	if params.IsTest != nil {
		query = query.Where("is_test = ?", *params.IsTest)
	}
	if params.Search != "" {
		query = query.Where("username LIKE ?", "%"+params.Search+"%")
	}

	// Status is a computed field, so we translate it to DB conditions
	now := time.Now()
	if params.Status == "active" {
		query = query.Where("enabled = ? AND expires_at > ? AND (traffic_limit = 0 OR traffic_used < traffic_limit)", true, now)
	} else if params.Status == "disabled" {
		query = query.Where("enabled = ?", false)
	} else if params.Status == "expired" {
		query = query.Where("enabled = ? AND expires_at <= ?", true, now)
	} else if params.Status == "traffic_exceeded" {
		query = query.Where("enabled = ? AND traffic_limit > 0 AND traffic_used >= traffic_limit", true)
	}

	var total int64
	query.Count(&total)

	var users []models.User
	offset := (params.Page - 1) * params.PerPage
	if err := query.Order("id DESC").Offset(offset).Limit(params.PerPage).Find(&users).Error; err != nil {
		return nil, err
	}

	responses := make([]models.UserResponse, len(users))
	for i, u := range users {
		responses[i] = u.ToResponse()
	}

	totalPages := int(total) / params.PerPage
	if int(total)%params.PerPage > 0 {
		totalPages++
	}

	return &UserListResult{
		Users:      responses,
		Total:      total,
		Page:       params.Page,
		PerPage:    params.PerPage,
		TotalPages: totalPages,
	}, nil
}

func UpdateUser(id uint, req UpdateUserRequest) (*models.User, error) {
	user, err := GetUser(id)
	if err != nil {
		return nil, err
	}

	if req.Password != nil && *req.Password != "" {
		if err := user.SetPassword(*req.Password); err != nil {
			return nil, err
		}
	}
	if req.MaxConnections != nil {
		user.MaxConnections = *req.MaxConnections
	}
	if req.TrafficLimitGB != nil {
		user.TrafficLimit = int64(*req.TrafficLimitGB * 1024 * 1024 * 1024)
	}
	if req.ExpiresAt != nil {
		expiresAt, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			expiresAt, err = time.Parse("2006-01-02", *req.ExpiresAt)
			if err != nil {
				return nil, fmt.Errorf("invalid expires_at format")
			}
			expiresAt = expiresAt.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
		user.ExpiresAt = expiresAt
	}
	if req.Enabled != nil {
		user.Enabled = *req.Enabled
	}

	if err := database.DB.Save(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func DeleteUser(id uint) error {
	result := database.DB.Unscoped().Delete(&models.User{}, id)
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return result.Error
}

func EnableUser(id uint) error {
	return database.DB.Model(&models.User{}).Where("id = ?", id).Update("enabled", true).Error
}

func DisableUser(id uint) error {
	return database.DB.Model(&models.User{}).Where("id = ?", id).Update("enabled", false).Error
}

func ResetTraffic(id uint) error {
	return database.DB.Model(&models.User{}).Where("id = ?", id).Update("traffic_used", 0).Error
}

func AddTraffic(username string, bytes int64) error {
	return database.DB.Model(&models.User{}).Where("username = ?", username).
		Update("traffic_used", gorm.Expr("traffic_used + ?", bytes)).Error
}

func UpdateLastConnection(username string) error {
	return database.DB.Model(&models.User{}).Where("username = ?", username).
		Update("last_connection", time.Now()).Error
}

func GetStats() map[string]interface{} {
	var totalUsers, activeUsers, testUsers, disabledUsers int64

	database.DB.Model(&models.User{}).Count(&totalUsers)
	database.DB.Model(&models.User{}).Where("enabled = ? AND expires_at > ? AND (traffic_limit = 0 OR traffic_used < traffic_limit)", true, time.Now()).Count(&activeUsers)
	database.DB.Model(&models.User{}).Where("is_test = ?", true).Count(&testUsers)
	database.DB.Model(&models.User{}).Where("enabled = ?", false).Count(&disabledUsers)

	return map[string]interface{}{
		"total_users":    totalUsers,
		"active_users":   activeUsers,
		"test_users":     testUsers,
		"disabled_users": disabledUsers,
	}
}

func randomString(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
