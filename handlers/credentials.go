package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"adpanel/middleware"
	"adpanel/models"
	"adpanel/services"
)

func ListCredentials(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	creds, err := models.ListCredentialsByUser(userID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "credentials.html", gin.H{
			"error": "Gagal memuat kredensial",
		})
		return
	}

	c.HTML(http.StatusOK, "credentials.html", gin.H{
		"title":       "Manajemen Kredensial",
		"credentials": creds,
		"user":        middleware.GetCurrentUser(c),
	})
}

func CreateCredential(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	label := c.PostForm("label")
	appID := c.PostForm("app_id")
	appSecret := c.PostForm("app_secret")
	accessToken := c.PostForm("access_token")

	if label == "" || appID == "" || appSecret == "" || accessToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Semua field wajib diisi"})
		return
	}

	// Validate token
	client := services.NewMetaClient(accessToken)
	name, err := client.ValidateToken()
	tokenStatus := "valid"
	if err != nil {
		tokenStatus = "invalid"
	}

	appSecretEnc, err := services.Encrypt(appSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal enkripsi data"})
		return
	}

	accessTokenEnc, err := services.Encrypt(accessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal enkripsi data"})
		return
	}

	cred := &models.Credential{
		UserID:         userID,
		Label:          label,
		AppID:          appID,
		AppSecretEnc:   appSecretEnc,
		AccessTokenEnc: accessTokenEnc,
		TokenStatus:    tokenStatus,
	}

	id, err := models.CreateCredential(cred)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan kredensial"})
		return
	}

	_ = models.ChangeLog(userID, "credential", id, "create", "", label)

	response := gin.H{
		"id":           id,
		"token_status": tokenStatus,
		"message":      "Kredensial berhasil disimpan",
	}
	if name != "" {
		response["token_name"] = name
	}

	c.JSON(http.StatusOK, response)
}

func UpdateCredential(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	cred, err := models.GetCredentialByID(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kredensial tidak ditemukan"})
		return
	}

	label := c.PostForm("label")
	appID := c.PostForm("app_id")
	appSecret := c.PostForm("app_secret")
	accessToken := c.PostForm("access_token")

	if label != "" {
		cred.Label = label
	}
	if appID != "" {
		cred.AppID = appID
	}

	if appSecret != "" {
		enc, err := services.Encrypt(appSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal enkripsi"})
			return
		}
		cred.AppSecretEnc = enc
	}

	if accessToken != "" {
		client := services.NewMetaClient(accessToken)
		if _, err := client.ValidateToken(); err != nil {
			cred.TokenStatus = "invalid"
		} else {
			cred.TokenStatus = "valid"
		}

		enc, err := services.Encrypt(accessToken)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal enkripsi"})
			return
		}
		cred.AccessTokenEnc = enc
	}

	if err := models.UpdateCredential(cred); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update kredensial"})
		return
	}

	_ = models.ChangeLog(userID, "credential", id, "update", "", cred.Label)

	c.JSON(http.StatusOK, gin.H{
		"message":      "Kredensial berhasil diupdate",
		"token_status": cred.TokenStatus,
	})
}

func DeleteCredential(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := models.DeleteCredential(id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal hapus kredensial"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Kredensial berhasil dihapus"})
}

func FetchAdAccounts(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	cred, err := models.GetCredentialByID(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kredensial tidak ditemukan"})
		return
	}

	accessToken, err := services.Decrypt(cred.AccessTokenEnc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal dekripsi token"})
		return
	}

	client := services.NewMetaClient(accessToken)
	accounts, err := client.FetchAdAccounts()
	if err != nil {
		// Mark token as invalid if API fails
		_ = models.UpdateCredentialTokenStatus(id, "invalid")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal fetch ad accounts: " + err.Error()})
		return
	}

	_ = models.UpdateCredentialTokenStatus(id, "valid")

	c.JSON(http.StatusOK, gin.H{
		"accounts": accounts,
	})
}

func ValidateToken(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	cred, err := models.GetCredentialByID(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kredensial tidak ditemukan"})
		return
	}

	accessToken, err := services.Decrypt(cred.AccessTokenEnc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal dekripsi token"})
		return
	}

	client := services.NewMetaClient(accessToken)
	name, err := client.ValidateToken()

	status := "valid"
	errMsg := ""
	if err != nil {
		status = "invalid"
		errMsg = err.Error()
	}

	_ = models.UpdateCredentialTokenStatus(id, status)

	c.JSON(http.StatusOK, gin.H{
		"status":  status,
		"name":    name,
		"error":   errMsg,
	})
}
