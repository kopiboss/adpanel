package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"adpanel/middleware"
	"adpanel/models"
)

func ListAdAccounts(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	accounts, err := models.ListAdAccountsByUser(userID)
	if err != nil {
		accounts = []models.AdAccount{}
	}
	creds, _ := models.ListCredentialsByUser(userID)
	c.HTML(http.StatusOK, "ad_accounts.html", gin.H{
		"title":       "Ad Accounts",
		"accounts":    accounts,
		"credentials": creds,
		"user":        middleware.GetCurrentUser(c),
	})
}

type saveAdAccountsPayload struct {
	CredentialID uint64             `json:"credential_id"`
	Accounts     []adAccountInput   `json:"accounts"`
}

type adAccountInput struct {
	MetaAccountID string `json:"meta_account_id"`
	Name          string `json:"name"`
	Currency      string `json:"currency"`
	Timezone      string `json:"timezone"`
	AccountStatus int    `json:"account_status"`
}

func SaveAdAccounts(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal baca request"})
		return
	}

	var payload saveAdAccountsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format JSON tidak valid"})
		return
	}

	if payload.CredentialID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "credential_id wajib diisi"})
		return
	}

	// Verify credential belongs to user
	cred, err := models.GetCredentialByID(payload.CredentialID, userID)
	if err != nil || cred == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kredensial tidak ditemukan"})
		return
	}

	if len(payload.Accounts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pilih minimal 1 ad account"})
		return
	}

	saved := 0
	for _, acc := range payload.Accounts {
		adAccount := &models.AdAccount{
			CredentialID:  payload.CredentialID,
			UserID:        userID,
			MetaAccountID: acc.MetaAccountID,
			Name:          acc.Name,
			Currency:      acc.Currency,
			Timezone:      acc.Timezone,
			AccountStatus: acc.AccountStatus,
			IsActive:      true,
		}
		if _, err := models.CreateAdAccount(adAccount); err == nil {
			saved++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": strconv.Itoa(saved) + " ad account berhasil disimpan",
		"saved":   saved,
	})
}

func ToggleAdAccount(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	account, err := models.GetAdAccountByID(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ad account tidak ditemukan"})
		return
	}
	newState := !account.IsActive
	if err := models.UpdateAdAccountActive(id, userID, newState); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"is_active": newState, "message": "Status berhasil diubah"})
}

func DeleteAdAccount(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	if err := models.DeleteAdAccount(id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal hapus ad account"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Ad account berhasil dihapus"})
}
