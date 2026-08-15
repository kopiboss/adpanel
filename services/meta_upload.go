package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"adpanel/models"
)

const (
	metaVideoUploadBase = "https://graph-video.facebook.com"
	chunkSize           = 4 * 1024 * 1024 // 4MB per chunk
)

// UploadImage uploads an image to Meta and returns the image hash
func UploadImage(accessToken, adAccountID, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("access_token", accessToken)

	part, err := writer.CreateFormFile("filename", filePath)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err = io.Copy(part, file); err != nil {
		return "", fmt.Errorf("copy file: %w", err)
	}
	writer.Close()

	reqURL := fmt.Sprintf("%s/%s/act_%s/adimages", metaAPIBase, metaAPIVersion, adAccountID)
	req, err := http.NewRequest(http.MethodPost, reqURL, &buf)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Images map[string]struct {
			Hash string `json:"hash"`
		} `json:"images"`
		Error struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if result.Error.Code != 0 {
		return "", fmt.Errorf("meta error %d: %s", result.Error.Code, result.Error.Message)
	}
	for _, img := range result.Images {
		return img.Hash, nil
	}
	return "", fmt.Errorf("no image hash in response: %s", string(body))
}

// UploadVideoResumable uploads a video to Meta using the 3-phase resumable upload API.
// Ref: https://developers.facebook.com/docs/marketing-api/reference/ad-video/
func UploadVideoResumable(accessToken, adAccountID, filePath string, creative *models.Creative) error {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	fileSize := fileInfo.Size()
	fileName := fileInfo.Name()

	// ── Phase 1: START ───────────────────────────────────────────────────────
	// Endpoint untuk start & finish pakai graph.facebook.com
	startURL := fmt.Sprintf("%s/%s/act_%s/advideos",
		metaAPIBase, metaAPIVersion, adAccountID)

	startData := url.Values{
		"access_token": {accessToken},
		"upload_phase": {"start"},
		"file_size":    {strconv.FormatInt(fileSize, 10)},
		"file_name":    {fileName},
	}

	startResp, err := http.PostForm(startURL, startData)
	if err != nil {
		return fmt.Errorf("start upload request: %w", err)
	}
	defer startResp.Body.Close()

	var startResult struct {
		VideoID         string `json:"video_id"`
		UploadSessionID string `json:"upload_session_id"`
		StartOffset     string `json:"start_offset"`
		EndOffset       string `json:"end_offset"`
		UploadDomain    string `json:"upload_domain"`
		Error           struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}

	startBody, _ := io.ReadAll(startResp.Body)
	if err := json.Unmarshal(startBody, &startResult); err != nil {
		return fmt.Errorf("parse start response: %w — body: %s", err, string(startBody))
	}
	if startResult.Error.Code != 0 {
		return fmt.Errorf("start error %d: %s", startResult.Error.Code, startResult.Error.Message)
	}

	videoID := startResult.VideoID
	// upload_session_id bisa ada di field VideoID atau UploadSessionID
	uploadSessionID := startResult.UploadSessionID
	if uploadSessionID == "" {
		uploadSessionID = videoID
	}
	if uploadSessionID == "" {
		return fmt.Errorf("no upload_session_id or video_id in start response: %s", string(startBody))
	}

	// Tentukan base URL untuk chunk transfer
	// Meta merekomendasikan graph-video.facebook.com untuk chunk transfer
	transferBase := metaVideoUploadBase
	if startResult.UploadDomain != "" {
		transferBase = "https://" + startResult.UploadDomain
	}
	transferURL := fmt.Sprintf("%s/%s/act_%s/advideos",
		transferBase, metaAPIVersion, adAccountID)

	// Parse offsets
	startOffset, _ := strconv.ParseInt(startResult.StartOffset, 10, 64)
	endOffset, _ := strconv.ParseInt(startResult.EndOffset, 10, 64)

	// ── Phase 2: TRANSFER CHUNKS ─────────────────────────────────────────────
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	for startOffset < fileSize {
		// Hitung ukuran chunk
		size := endOffset - startOffset
		if size <= 0 {
			size = chunkSize
		}
		if startOffset+size > fileSize {
			size = fileSize - startOffset
		}

		// Seek ke posisi yang benar
		if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
			return fmt.Errorf("seek to %d: %w", startOffset, err)
		}

		// Baca chunk
		chunk := make([]byte, size)
		n, err := io.ReadFull(file, chunk)
		if err != nil && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("read chunk at %d: %w", startOffset, err)
		}
		chunk = chunk[:n]

		// Build multipart body
		var chunkBuf bytes.Buffer
		mw := multipart.NewWriter(&chunkBuf)
		_ = mw.WriteField("access_token", accessToken)
		_ = mw.WriteField("upload_phase", "transfer")
		_ = mw.WriteField("upload_session_id", uploadSessionID)
		_ = mw.WriteField("start_offset", strconv.FormatInt(startOffset, 10))

		fw, err := mw.CreateFormFile("video_file_chunk", fileName)
		if err != nil {
			return fmt.Errorf("create chunk form file: %w", err)
		}
		if _, err := fw.Write(chunk); err != nil {
			return fmt.Errorf("write chunk: %w", err)
		}
		mw.Close()

		req, err := http.NewRequest(http.MethodPost, transferURL, &chunkBuf)
		if err != nil {
			return fmt.Errorf("create chunk request: %w", err)
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())

		chunkResp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("send chunk: %w", err)
		}

		var chunkResult struct {
			StartOffset string `json:"start_offset"`
			EndOffset   string `json:"end_offset"`
			Error       struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		chunkBody, _ := io.ReadAll(chunkResp.Body)
		chunkResp.Body.Close()

		if err := json.Unmarshal(chunkBody, &chunkResult); err != nil {
			return fmt.Errorf("parse chunk response: %w — body: %s", err, string(chunkBody))
		}
		if chunkResult.Error.Code != 0 {
			return fmt.Errorf("chunk error %d: %s — session: %s offset: %d body: %s",
				chunkResult.Error.Code, chunkResult.Error.Message,
				uploadSessionID, startOffset, string(chunkBody))
		}

		// Update offsets dari response
		newStart, _ := strconv.ParseInt(chunkResult.StartOffset, 10, 64)
		newEnd, _ := strconv.ParseInt(chunkResult.EndOffset, 10, 64)

		// Proteksi infinite loop: kalau offset tidak bergerak, pakai chunkSize
		if newStart == startOffset {
			startOffset += int64(n)
			endOffset = startOffset + chunkSize
		} else {
			startOffset = newStart
			endOffset = newEnd
		}
	}

	// ── Phase 3: FINISH ──────────────────────────────────────────────────────
	finishURL := fmt.Sprintf("%s/%s/act_%s/advideos",
		metaAPIBase, metaAPIVersion, adAccountID)

	finishData := url.Values{
		"access_token":      {accessToken},
		"upload_phase":      {"finish"},
		"upload_session_id": {uploadSessionID},
		"title":             {fileName},
	}

	finishResp, err := http.PostForm(finishURL, finishData)
	if err != nil {
		return fmt.Errorf("finish upload: %w", err)
	}
	defer finishResp.Body.Close()

	var finishResult struct {
		Success bool   `json:"success"`
		VideoID string `json:"video_id"`
		Error   struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	finishBody, _ := io.ReadAll(finishResp.Body)
	if err := json.Unmarshal(finishBody, &finishResult); err != nil {
		return fmt.Errorf("parse finish response: %w — body: %s", err, string(finishBody))
	}
	if finishResult.Error.Code != 0 {
		return fmt.Errorf("finish error %d: %s", finishResult.Error.Code, finishResult.Error.Message)
	}

	// Gunakan video_id dari finish response jika ada, fallback ke uploadSessionID
	finalVideoID := finishResult.VideoID
	if finalVideoID == "" {
		finalVideoID = videoID
	}

	// ── Get thumbnail (retry karena Meta butuh waktu proses) ──────────────────
	thumbnail := ""
	for i := 0; i < 6; i++ {
		// Tunggu dulu sebelum coba — video perlu waktu transcode
		waitSeconds := []int{5, 10, 15, 20, 30, 30}
		time.Sleep(time.Duration(waitSeconds[i]) * time.Second)

		t, err := FetchVideoThumbnail(accessToken, finalVideoID)
		if err == nil && t != "" {
			thumbnail = t
			break
		}
	}

	return models.UpdateCreativeAfterUpload(creative.ID, "", finalVideoID, thumbnail, "done")
}

// ProcessImageUpload handles image upload background job
func ProcessImageUpload(creative *models.Creative, accessToken, adAccountID, tempFilePath string) {
	defer os.Remove(tempFilePath)
	_ = models.UpdateCreativeStatus(creative.ID, "uploading", "")

	hash, err := UploadImage(accessToken, adAccountID, tempFilePath)
	if err != nil {
		_ = models.UpdateCreativeStatus(creative.ID, "failed", err.Error())
		return
	}
	_ = models.UpdateCreativeAfterUpload(creative.ID, hash, "", "", "done")
}

// ProcessVideoUpload handles video upload background job
func ProcessVideoUpload(creative *models.Creative, accessToken, adAccountID, tempFilePath string) {
	defer os.Remove(tempFilePath)
	_ = models.UpdateCreativeStatus(creative.ID, "uploading", "")

	if err := UploadVideoResumable(accessToken, adAccountID, tempFilePath, creative); err != nil {
		_ = models.UpdateCreativeStatus(creative.ID, "failed", err.Error())
	}
}

// FetchVideoThumbnail mengambil thumbnail URL untuk video yang sudah ada di Meta.
// Meta kadang return GIF placeholder saat video masih diproses — kita skip itu.
func FetchVideoThumbnail(accessToken, videoID string) (string, error) {
	thumbURL := fmt.Sprintf("%s/%s/%s?fields=picture,thumbnails{uri,width,height}&access_token=%s",
		metaAPIBase, metaAPIVersion, videoID, accessToken)

	resp, err := http.Get(thumbURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Picture    string `json:"picture"`
		Thumbnails struct {
			Data []struct {
				URI    string `json:"uri"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"data"`
		} `json:"thumbnails"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	// Pilih thumbnail terbesar dari thumbnails.data (lebih reliable dari picture)
	// Ini adalah frame asli video, bukan placeholder
	bestURI := ""
	bestSize := 0
	for _, t := range result.Thumbnails.Data {
		size := t.Width * t.Height
		if size > bestSize && t.URI != "" && !isPlaceholderURL(t.URI) {
			bestSize = size
			bestURI = t.URI
		}
	}
	if bestURI != "" {
		return bestURI, nil
	}

	// Fallback ke picture, tapi skip kalau itu placeholder GIF
	if result.Picture != "" && !isPlaceholderURL(result.Picture) {
		return result.Picture, nil
	}

	return "", nil // Video belum siap, coba lagi nanti
}

// isPlaceholderURL deteksi apakah URL adalah placeholder Meta (GIF animasi loading)
func isPlaceholderURL(u string) bool {
	// Meta placeholder biasanya .gif atau mengandung path tertentu
	return strings.HasSuffix(strings.ToLower(u), ".gif") ||
		strings.Contains(u, "rsrc.php")
}
