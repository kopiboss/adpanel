package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	if err != nil || accountID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ad account ID"})
		return
	}

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

	ext := strings.ToLower(filepath.Ext(header.Filename))
	creativeType := ""
	maxSize := int64(0)

	switch ext {
	case ".jpg", ".jpeg", ".png":
		creativeType = "image"
		maxSize = 30 * 1024 * 1024
	case ".mp4":
		creativeType = "video"
		maxSize = 1024 * 1024 * 1024
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak didukung. Gunakan JPG, PNG, atau MP4"})
		return
	}

	if header.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("File terlalu besar. Maksimal %dMB", maxSize/1024/1024),
		})
		return
	}

	// Save to temp file
	tempFile := filepath.Join(os.TempDir(),
		fmt.Sprintf("adpanel_%d_%d%s", userID, time.Now().UnixNano(), ext))

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

	if creativeType == "image" {
		go services.ProcessImageUpload(creative, accessToken, account.MetaAccountID, tempFile)
	} else {
		go services.ProcessVideoUpload(creative, accessToken, account.MetaAccountID, tempFile)
	}

	// Return immediately after record created - frontend polls for status
	c.JSON(http.StatusOK, gin.H{
		"creative_id": creativeID,
		"name":        header.Filename,
		"type":        creativeType,
		"message":     "Upload dimulai",
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

	ext := strings.ToLower(filepath.Ext(header.Filename))
	creativeType := ""
	switch ext {
	case ".jpg", ".jpeg", ".png":
		creativeType = "image"
	case ".mp4":
		creativeType = "video"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tidak didukung"})
		return
	}

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
				r.Status = "failed"; resultCh <- r; return
			}
			r.AccountName = account.Name
			cred, err := models.GetCredentialByIDAdmin(account.CredentialID)
			if err != nil {
				r.Status = "failed"; resultCh <- r; return
			}
			accessToken, err := services.Decrypt(cred.AccessTokenEnc)
			if err != nil {
				r.Status = "failed"; resultCh <- r; return
			}
			creative := &models.Creative{
				AdAccountID: accountID, UserID: userID,
				Name: header.Filename, Type: creativeType,
				FileSize: header.Size, UploadStatus: "pending",
			}
			creativeID, err := models.CreateCreative(creative)
			if err != nil {
				r.Status = "failed"; resultCh <- r; return
			}
			creative.ID = creativeID
			r.CreativeID = creativeID

			accountTempFile := fmt.Sprintf("%s.%d", tempFile, accountID)
			if err := copyFile(tempFile, accountTempFile); err != nil {
				r.Status = "failed"; resultCh <- r; return
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
		results = append(results, <-resultCh)
	}
	go func() { time.Sleep(30 * time.Second); os.Remove(tempFile) }()

	c.JSON(http.StatusOK, gin.H{"results": results, "message": "Upload dimulai"})
}

// GetCreativeStatus - polling endpoint untuk status upload creative
func GetCreativeStatus(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	// Coba dengan user_id dulu
	creative, err := models.GetCreativeByID(id, userID)
	if err != nil {
		// Fallback tanpa user_id
		creative, err = models.GetCreativeByIDOnly(id)
		if err != nil {
			// Record belum ada di DB (race condition antara INSERT dan polling)
			// Return 200 dengan status pending supaya frontend terus poll
			c.JSON(http.StatusOK, gin.H{
				"id":            id,
				"upload_status": "pending",
			})
			return
		}
		// Security: pastikan milik user ini
		if creative.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
			return
		}
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
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
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

// RefreshCreativeThumbnail - fetch ulang thumbnail dari Meta untuk video yang sudah done
func RefreshCreativeThumbnail(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	creative, err := models.GetCreativeByID(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Creative tidak ditemukan"})
		return
	}

	if creative.MetaVideoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Creative ini tidak memiliki video ID"})
		return
	}

	account, err := models.GetAdAccountByID(creative.AdAccountID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ad account tidak ditemukan"})
		return
	}

	cred, err := models.GetCredentialByIDAdmin(account.CredentialID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal load kredensial"})
		return
	}

	accessToken, err := services.Decrypt(cred.AccessTokenEnc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal dekripsi token"})
		return
	}

	thumbnail, err := services.FetchVideoThumbnail(accessToken, creative.MetaVideoID)
	if err != nil || thumbnail == "" {
		c.JSON(http.StatusOK, gin.H{"message": "Thumbnail belum tersedia, coba lagi nanti", "thumbnail_url": ""})
		return
	}

	_ = models.UpdateCreativeAfterUpload(creative.ID, "", creative.MetaVideoID, thumbnail, "done")
	c.JSON(http.StatusOK, gin.H{"thumbnail_url": thumbnail, "message": "Thumbnail berhasil diperbarui"})
}

// ProxyThumbnail - proxy thumbnail dari Meta CDN agar bisa ditampilkan di browser
// tanpa CORS/hotlink block
func ProxyThumbnail(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	creative, err := models.GetCreativeByID(id, userID)
	if err != nil {
		creative, err = models.GetCreativeByIDOnly(id)
		if err != nil || creative.UserID != userID {
			c.Status(http.StatusNotFound)
			return
		}
	}

	if creative.ThumbnailURL == "" {
		c.Status(http.StatusNotFound)
		return
	}

	// Fetch dari Meta CDN
	req, err := http.NewRequest(http.MethodGet, creative.ThumbnailURL, nil)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	// Set headers agar fbcdn mau serve
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AdPanel/1.0)")
	req.Header.Set("Referer", "https://www.facebook.com/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		c.Status(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Teruskan content-type dan body
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	// Cache 1 jam
	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("Content-Type", contentType)
	c.Status(http.StatusOK)
	io.Copy(c.Writer, resp.Body)
}
