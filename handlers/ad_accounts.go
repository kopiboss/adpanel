package handlers

import (
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
		c.HTML(http.StatusInternalServerError, "ad_accounts.html", gin.H{
			"error": "Gagal memuat ad accounts",
		})
		return
	}

	creds, _ := models.ListCredentialsByUser(userID)

	c.HTML(http.StatusOK, "ad_accounts.html", gin.H{
		"title":       "Ad Accounts",
		"accounts":    accounts,
		"credentials": creds,
		"user":        middleware.GetCurrentUser(c),
	})
}

func SaveAdAccounts(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	credIDStr := c.PostForm("credential_id")
	credID, err := strconv.ParseUint(credIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credential ID"})
		return
	}

	// Verify credential belongs to user
	cred, err := models.GetCredentialByID(credID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kredensial tidak ditemukan"})
		return
	}

	type AccountInput struct {
		MetaAccountID string `json:"meta_account_id"`
		Name          string `json:"name"`
		Currency      string `json:"currency"`
		Timezone      string `json:"timezone"`
		AccountStatus int    `json:"account_status"`
	}

	var accounts []AccountInput
	if err := c.ShouldBindJSON(&accounts); err != nil {
		// Try form data fallback
		metaIDs := c.PostFormArray("meta_account_ids[]")
		names := c.PostFormArray("names[]")
		currencies := c.PostFormArray("currencies[]")
		timezones := c.PostFormArray("timezones[]")

		for i, metaID := range metaIDs {
			acc := AccountInput{MetaAccountID: metaID, AccountStatus: 1}
			if i < len(names) {
				acc.Name = names[i]
			}
			if i < len(currencies) {
				acc.Currency = currencies[i]
			}
			if i < len(timezones) {
				acc.Timezone = timezones[i]
			}
			accounts = append(accounts, acc)
		}
	}

	_ = cred

	saved := 0
	for _, acc := range accounts {
		adAccount := &models.AdAccount{
			CredentialID:  credID,
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
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
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

	c.JSON(http.StatusOK, gin.H{
		"is_active": newState,
		"message":   "Status berhasil diubah",
	})
}

func DeleteAdAccount(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
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
