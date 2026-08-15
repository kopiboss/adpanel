package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"adpanel/middleware"
	"adpanel/models"
	"adpanel/services"
)

func ListCreatives(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	accounts, _ := models.ListActiveAdAccountsByUser(userID)

	accountID := uint64(0)
	if accIDStr := c.Query("account_id"); accIDStr != "" {
		accountID, _ = strconv.ParseUint(accIDStr, 10, 64)
	}

	var creatives []models.Creative
	if accountID > 0 {
		creatives, _ = models.ListCreativesByAdAccount(accountID, userID)
	} else {
		creatives, _ = models.ListCreativesByUser(userID)
	}

	c.HTML(http.StatusOK, "creatives.html", gin.H{
		"title":      "Creative Library",
		"creatives":  creatives,
		"accounts":   accounts,
		"account_id": accountID,
		"user":       middleware.GetCurrentUser(c),
	})
}

func UploadCreative(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	accountIDStr := c.PostForm("ad_account_id")
	accountID, err := strconv.ParseUint(accountIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ad account ID"})
		return
	}

	// Verify account belongs to user
	account, err := models.GetAdAccountByID(accountID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ad account tidak ditemukan"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membaca file"})
		return
	}
	defer file.Close()

	// Determine file type
	ext := filepath.Ext(header.Filename)
	creativeType := ""
	maxSize := int64(0)

	switch ext {
	case ".jpg", ".jpeg", ".png":
		creativeType = "image"
		maxSize = 30 * 1024 * 1024 // 30MB
	case ".mp4":
		creativeType = "video"
		maxSize = 1024 * 1024 * 1024 // 1GB
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tipe file tidak didukung. Gunakan JPG, PNG, atau MP4"})
		return
	}

	if header.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("File terlalu besar. Max: %dMB", maxSize/1024/1024),
		})
		return
	}

	// Save to temp file
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("adpanel_%d_%d%s", userID, time.Now().UnixNano(), ext))

	f, err := os.Create(tempFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file sementara"})
		return
	}

	if _, err := f.ReadFrom(file); err != nil {
		f.Close()
		os.Remove(tempFile)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menulis file"})
		return
	}
	f.Close()

	// Create creative record
	creative := &models.Creative{
		AdAccountID:  accountID,
		UserID:       userID,
		Name:         header.Filename,
		Type:         creativeType,
		FileSize:     header.Size,
		UploadStatus: "pending",
	}

	creativeID, err := models.CreateCreative(creative)
	if err != nil {
		os.Remove(tempFile)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat record creative"})
		return
	}
	creative.ID = creativeID

	// Get access token
	cred, err := models.GetCredentialByIDAdmin(account.CredentialID)
	if err != nil {
		os.Remove(tempFile)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal load kredensial"})
		return
	}

	accessToken, err := services.Decrypt(cred.AccessTokenEnc)
	if err != nil {
		os.Remove(tempFile)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal dekripsi token"})
		return
	}

	// Launch upload in background
	if creativeType == "image" {
		go services.ProcessImageUpload(creative, accessToken, account.MetaAccountID, tempFile)
	} else {
		go services.ProcessVideoUpload(creative, accessToken, account.MetaAccountID, tempFile)
	}

	c.JSON(http.StatusOK, gin.H{
		"creative_id": creativeID,
		"name":        header.Filename,
		"type":        creativeType,
		"status":      "pending",
		"message":     "Upload dimulai, memantau status...",
	})
}

func UploadToMultipleAccounts(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membaca file"})
		return
	}
	defer file.Close()

	accountIDsStr := c.PostFormArray("ad_account_ids[]")
	if len(accountIDsStr) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pilih minimal 1 ad account"})
		return
	}

	ext := filepath.Ext(header.Filename)
	creativeType := ""
	switch ext {
	case ".jpg", ".jpeg", ".png":
		creativeType = "image"
	case ".mp4":
		creativeType = "video"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tipe file tidak didukung"})
		return
	}

	// Save temp file once
	tempFile := filepath.Join(os.TempDir(),
		fmt.Sprintf("adpanel_%d_%d%s", userID, time.Now().UnixNano(), ext))
	f, err := os.Create(tempFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file"})
		return
	}
	_, _ = f.ReadFrom(file)
	f.Close()

	type uploadResult struct {
		AccountID   uint64 `json:"account_id"`
		AccountName string `json:"account_name"`
		CreativeID  uint64 `json:"creative_id"`
		Status      string `json:"status"`
	}

	results := make([]uploadResult, 0)
	resultCh := make(chan uploadResult, len(accountIDsStr))

	for _, accIDStr := range accountIDsStr {
		accID, err := strconv.ParseUint(accIDStr, 10, 64)
		if err != nil {
			continue
		}

		go func(accountID uint64) {
			r := uploadResult{AccountID: accountID}

			account, err := models.GetAdAccountByID(accountID, userID)
			if err != nil {
				r.Status = "failed"
				resultCh <- r
				return
			}
			r.AccountName = account.Name

			cred, err := models.GetCredentialByIDAdmin(account.CredentialID)
			if err != nil {
				r.Status = "failed"
				resultCh <- r
				return
			}

			accessToken, err := services.Decrypt(cred.AccessTokenEnc)
			if err != nil {
				r.Status = "failed"
				resultCh <- r
				return
			}

			creative := &models.Creative{
				AdAccountID:  accountID,
				UserID:       userID,
				Name:         header.Filename,
				Type:         creativeType,
				FileSize:     header.Size,
				UploadStatus: "pending",
			}

			creativeID, err := models.CreateCreative(creative)
			if err != nil {
				r.Status = "failed"
				resultCh <- r
				return
			}
			creative.ID = creativeID
			r.CreativeID = creativeID

			// Copy temp file for this account
			accountTempFile := fmt.Sprintf("%s.%d", tempFile, accountID)
			if err := copyFile(tempFile, accountTempFile); err != nil {
				r.Status = "failed"
				resultCh <- r
				return
			}

			r.Status = "uploading"
			resultCh <- r

			if creativeType == "image" {
				go services.ProcessImageUpload(creative, accessToken, account.MetaAccountID, accountTempFile)
			} else {
				go services.ProcessVideoUpload(creative, accessToken, account.MetaAccountID, accountTempFile)
			}
		}(accID)
	}

	for range accountIDsStr {
		r := <-resultCh
		results = append(results, r)
	}

	// Remove original temp file after all goroutines have copied it
	go func() {
		time.Sleep(30 * time.Second)
		os.Remove(tempFile)
	}()

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"message": "Upload dimulai untuk semua akun yang dipilih",
	})
}

func GetCreativeStatus(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	creative, err := models.GetCreativeByID(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Creative tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":              creative.ID,
		"upload_status":   creative.UploadStatus,
		"meta_image_hash": creative.MetaImageHash,
		"meta_video_id":   creative.MetaVideoID,
		"thumbnail_url":   creative.ThumbnailURL,
		"error_message":   creative.ErrorMessage,
	})
}

func DeleteCreativeHandler(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := models.DeleteCreative(id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal hapus creative"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Creative berhasil dihapus"})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.ReadFrom(in)
	return err
}
