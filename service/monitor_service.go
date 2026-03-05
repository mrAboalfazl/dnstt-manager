package service

import (
	"log"
	"time"

	"github.com/mrAboalfazl/dnstt-manager/database"
	"github.com/mrAboalfazl/dnstt-manager/models"
)

func StartMonitor(intervalSec int) {
	if intervalSec <= 0 {
		intervalSec = 30
	}

	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)

	go func() {
		for range ticker.C {
			disableExpiredAndOverQuota()
		}
	}()

	log.Printf("[Monitor] Started, checking every %d seconds", intervalSec)
}

func disableExpiredAndOverQuota() {
	now := time.Now()

	// Disable expired users
	result := database.DB.Model(&models.User{}).
		Where("enabled = ? AND expires_at < ?", true, now).
		Update("enabled", false)

	if result.RowsAffected > 0 {
		log.Printf("[Monitor] Disabled %d expired users", result.RowsAffected)
	}

	// Disable users who exceeded traffic limit
	result = database.DB.Model(&models.User{}).
		Where("enabled = ? AND traffic_limit > 0 AND traffic_used >= traffic_limit", true).
		Update("enabled", false)

	if result.RowsAffected > 0 {
		log.Printf("[Monitor] Disabled %d users who exceeded traffic limit", result.RowsAffected)
	}
}
