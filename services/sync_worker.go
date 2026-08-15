package services

import (
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"adpanel/models"
)

// SyncAllAccounts syncs campaigns and insights for all active ad accounts
func SyncAllAccounts() {
	log.Println("Starting sync for all active ad accounts")
	start := time.Now()

	accounts, err := models.ListAllActiveAdAccounts()
	if err != nil {
		log.Printf("Sync: failed to list ad accounts: %v", err)
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // max 5 concurrent syncs

	for _, account := range accounts {
		wg.Add(1)
		sem <- struct{}{}
		go func(acc models.AdAccount) {
			defer wg.Done()
			defer func() { <-sem }()
			SyncAccount(&acc)
		}(account)
	}

	wg.Wait()
	log.Printf("Sync completed in %v for %d accounts", time.Since(start), len(accounts))
}

// SyncAccount syncs a single ad account
func SyncAccount(account *models.AdAccount) {
	startTime := time.Now()

	logEntry := &models.SyncLog{
		AdAccountID: account.ID,
		UserID:      account.UserID,
		SyncType:    "auto",
		Status:      "running",
	}
	logID, err := models.CreateSyncLog(logEntry)
	if err != nil {
		log.Printf("Sync: failed to create sync log: %v", err)
		return
	}

	defer func() {
		duration := time.Since(startTime).Milliseconds()
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("panic: %v", r)
			_ = models.UpdateSyncLog(logID, "failed", errMsg, duration)
		}
	}()

	// Get credential for this account
	cred, err := models.GetCredentialByIDAdmin(account.CredentialID)
	if err != nil {
		errMsg := fmt.Sprintf("get credential: %v", err)
		_ = models.UpdateSyncLog(logID, "failed", errMsg, time.Since(startTime).Milliseconds())
		return
	}

	accessToken, err := Decrypt(cred.AccessTokenEnc)
	if err != nil {
		errMsg := fmt.Sprintf("decrypt token: %v", err)
		_ = models.UpdateSyncLog(logID, "failed", errMsg, time.Since(startTime).Milliseconds())
		return
	}

	client := NewMetaClient(accessToken)

	// Validate token first
	if _, err := client.ValidateToken(); err != nil {
		errMsg := fmt.Sprintf("token invalid: %v", err)
		_ = models.UpdateSyncLog(logID, "failed", errMsg, time.Since(startTime).Milliseconds())
		_ = models.UpdateCredentialTokenStatus(cred.ID, "invalid")

		// Notify user via Telegram
		user, _ := models.GetUserByID(account.UserID)
		if user != nil {
			Bot.NotifyTokenError(user, cred.Label)
		}
		return
	}

	_ = models.UpdateCredentialTokenStatus(cred.ID, "valid")

	// Sync campaigns with retry on rate limit
	var syncErr error
	for attempt := 0; attempt < 3; attempt++ {
		syncErr = syncCampaigns(client, account)
		if syncErr == nil {
			break
		}

		if isRateLimitError(syncErr) {
			log.Printf("Rate limit hit for account %s, waiting 5 minutes...", account.MetaAccountID)
			time.Sleep(5 * time.Minute)
			continue
		}
		break
	}

	if syncErr != nil {
		errMsg := fmt.Sprintf("sync campaigns: %v", syncErr)
		_ = models.UpdateSyncLog(logID, "failed", errMsg, time.Since(startTime).Milliseconds())
		return
	}

	// Sync insights (last 7 days)
	for attempt := 0; attempt < 3; attempt++ {
		syncErr = SyncInsights(client, account, account.UserID, 7)
		if syncErr == nil {
			break
		}
		if isRateLimitError(syncErr) {
			log.Printf("Rate limit hit for insights %s, waiting 5 minutes...", account.MetaAccountID)
			time.Sleep(5 * time.Minute)
			continue
		}
		break
	}

	if syncErr != nil {
		errMsg := fmt.Sprintf("sync insights: %v", syncErr)
		_ = models.UpdateSyncLog(logID, "failed", errMsg, time.Since(startTime).Milliseconds())
		return
	}

	_ = models.UpdateSyncLog(logID, "success", "", time.Since(startTime).Milliseconds())
	log.Printf("Sync completed for account %s (user %d)", account.MetaAccountID, account.UserID)
}

func syncCampaigns(client *MetaClient, account *models.AdAccount) error {
	campaigns, err := client.FetchCampaigns(account.MetaAccountID)
	if err != nil {
		return err
	}

	for _, mc := range campaigns {
		budget, _ := strconv.ParseInt(mc.DailyBudget, 10, 64)

		campaign := &models.Campaign{
			AdAccountID:    account.ID,
			UserID:         account.UserID,
			MetaCampaignID: mc.ID,
			Name:           mc.Name,
			Status:         mc.Status,
			Objective:      mc.Objective,
			DailyBudget:    budget,
		}

		if err := models.UpsertCampaignFromMeta(campaign); err != nil {
			log.Printf("Upsert campaign %s: %v", mc.ID, err)
		}
	}

	return nil
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "error 17:") || contains(msg, "error 4:")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && searchString(s, substr))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// SyncAccountNow triggers an immediate sync for a specific account (used by "Sync Now" button)
func SyncAccountNow(account *models.AdAccount) error {
	startTime := time.Now()

	logEntry := &models.SyncLog{
		AdAccountID: account.ID,
		UserID:      account.UserID,
		SyncType:    "manual",
		Status:      "running",
	}
	logID, err := models.CreateSyncLog(logEntry)
	if err != nil {
		return err
	}

	cred, err := models.GetCredentialByIDAdmin(account.CredentialID)
	if err != nil {
		_ = models.UpdateSyncLog(logID, "failed", err.Error(), time.Since(startTime).Milliseconds())
		return err
	}

	accessToken, err := Decrypt(cred.AccessTokenEnc)
	if err != nil {
		_ = models.UpdateSyncLog(logID, "failed", err.Error(), time.Since(startTime).Milliseconds())
		return err
	}

	client := NewMetaClient(accessToken)

	if err := syncCampaigns(client, account); err != nil {
		_ = models.UpdateSyncLog(logID, "failed", err.Error(), time.Since(startTime).Milliseconds())
		return err
	}

	if err := SyncInsights(client, account, account.UserID, 7); err != nil {
		_ = models.UpdateSyncLog(logID, "failed", err.Error(), time.Since(startTime).Milliseconds())
		return err
	}

	_ = models.UpdateSyncLog(logID, "success", "", time.Since(startTime).Milliseconds())
	return nil
}
