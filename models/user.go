package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	Username       string         `gorm:"uniqueIndex;not null;size:64" json:"username"`
	Password       string         `gorm:"not null" json:"-"`
	MaxConnections int            `gorm:"not null;default:1" json:"max_connections"`
	TrafficLimit   int64          `gorm:"not null;default:0" json:"traffic_limit"`
	TrafficUsed    int64          `gorm:"not null;default:0" json:"traffic_used"`
	ExpiresAt      time.Time      `gorm:"not null" json:"expires_at"`
	Enabled        bool           `gorm:"not null;default:true" json:"enabled"`
	IsTest         bool           `gorm:"not null;default:false" json:"is_test"`
	LastConnection time.Time      `json:"last_connection"`
}

func (u *User) SetPassword(plain string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hash)
	return nil
}

func (u *User) CheckPassword(plain string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(plain))
	return err == nil
}

func (u *User) IsExpired() bool {
	return time.Now().After(u.ExpiresAt)
}

func (u *User) IsTrafficExceeded() bool {
	if u.TrafficLimit <= 0 {
		return false
	}
	return u.TrafficUsed >= u.TrafficLimit
}

func (u *User) IsActive() bool {
	return u.Enabled && !u.IsExpired() && !u.IsTrafficExceeded()
}

func (u *User) Status() string {
	if !u.Enabled {
		return "disabled"
	}
	if u.IsExpired() {
		return "expired"
	}
	if u.IsTrafficExceeded() {
		return "traffic_exceeded"
	}
	return "active"
}

func (u *User) TrafficLimitGB() float64 {
	if u.TrafficLimit <= 0 {
		return 0
	}
	return float64(u.TrafficLimit) / (1024 * 1024 * 1024)
}

func (u *User) TrafficUsedGB() float64 {
	return float64(u.TrafficUsed) / (1024 * 1024 * 1024)
}

func (u *User) TrafficUsedMB() float64 {
	return float64(u.TrafficUsed) / (1024 * 1024)
}

type UserResponse struct {
	ID             uint      `json:"id"`
	Username       string    `json:"username"`
	MaxConnections int       `json:"max_connections"`
	TrafficLimit   int64     `json:"traffic_limit"`
	TrafficUsed    int64     `json:"traffic_used"`
	TrafficLimitGB float64   `json:"traffic_limit_gb"`
	TrafficUsedGB  float64   `json:"traffic_used_gb"`
	ExpiresAt      time.Time `json:"expires_at"`
	Enabled        bool      `json:"enabled"`
	IsTest         bool      `json:"is_test"`
	Status         string    `json:"status"`
	LastConnection time.Time `json:"last_connection"`
	CreatedAt      time.Time `json:"created_at"`
}

func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:             u.ID,
		Username:       u.Username,
		MaxConnections: u.MaxConnections,
		TrafficLimit:   u.TrafficLimit,
		TrafficUsed:    u.TrafficUsed,
		TrafficLimitGB: u.TrafficLimitGB(),
		TrafficUsedGB:  u.TrafficUsedGB(),
		ExpiresAt:      u.ExpiresAt,
		Enabled:        u.Enabled,
		IsTest:         u.IsTest,
		Status:         u.Status(),
		LastConnection: u.LastConnection,
		CreatedAt:      u.CreatedAt,
	}
}
