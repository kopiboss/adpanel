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

	"adpanel/models"
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

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

	return "", fmt.Errorf("no image hash in response")
}

// UploadVideoResumable streams a video to Meta using resumable upload API
// without buffering the entire file in memory
func UploadVideoResumable(accessToken, adAccountID, filePath string, creative *models.Creative) error {
	// Step 1: Initialize upload session
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	fileSize := fileInfo.Size()

	initURL := fmt.Sprintf("%s/%s/act_%s/advideos", metaAPIBase, metaAPIVersion, adAccountID)
	initData := url.Values{
		"access_token": {accessToken},
		"upload_phase": {"start"},
		"file_size":    {fmt.Sprintf("%d", fileSize)},
		"file_name":    {fileInfo.Name()},
	}

	resp, err := http.PostForm(initURL, initData)
	if err != nil {
		return fmt.Errorf("init upload: %w", err)
	}
	defer resp.Body.Close()

	var initResult struct {
		VideoID      string `json:"video_id"`
		StartOffset  string `json:"start_offset"`
		EndOffset    string `json:"end_offset"`
		UploadDomain string `json:"upload_domain"`
		Error        struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&initResult); err != nil {
		return fmt.Errorf("parse init response: %w", err)
	}

	if initResult.Error.Code != 0 {
		return fmt.Errorf("init error %d: %s", initResult.Error.Code, initResult.Error.Message)
	}

	videoID := initResult.VideoID

	// Step 2: Upload chunks
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	var startOffset, endOffset int64
	fmt.Sscanf(initResult.StartOffset, "%d", &startOffset)
	fmt.Sscanf(initResult.EndOffset, "%d", &endOffset)

	for startOffset < fileSize {
		chunkSize := endOffset - startOffset
		chunk := make([]byte, chunkSize)

		if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
			return fmt.Errorf("seek: %w", err)
		}
		n, err := io.ReadFull(file, chunk)
		if err != nil && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("read chunk: %w", err)
		}
		chunk = chunk[:n]

		var chunkBuf bytes.Buffer
		mw := multipart.NewWriter(&chunkBuf)
		_ = mw.WriteField("access_token", accessToken)
		_ = mw.WriteField("upload_phase", "transfer")
		_ = mw.WriteField("video_id", videoID)
		_ = mw.WriteField("start_offset", fmt.Sprintf("%d", startOffset))

		pw, _ := mw.CreateFormFile("video_file_chunk", fileInfo.Name())
		pw.Write(chunk)
		mw.Close()

		uploadURL := fmt.Sprintf("https://%s/%s/act_%s/advideos",
			initResult.UploadDomain, metaAPIVersion, adAccountID)
		if initResult.UploadDomain == "" {
			uploadURL = fmt.Sprintf("%s/%s/act_%s/advideos",
				metaAPIBase, metaAPIVersion, adAccountID)
		}

		req, _ := http.NewRequest(http.MethodPost, uploadURL, &chunkBuf)
		req.Header.Set("Content-Type", mw.FormDataContentType())

		chunkResp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("upload chunk: %w", err)
		}

		var chunkResult struct {
			StartOffset string `json:"start_offset"`
			EndOffset   string `json:"end_offset"`
			Error       struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		json.NewDecoder(chunkResp.Body).Decode(&chunkResult)
		chunkResp.Body.Close()

		if chunkResult.Error.Code != 0 {
			return fmt.Errorf("chunk error %d: %s", chunkResult.Error.Code, chunkResult.Error.Message)
		}

		fmt.Sscanf(chunkResult.StartOffset, "%d", &startOffset)
		fmt.Sscanf(chunkResult.EndOffset, "%d", &endOffset)
	}

	// Step 3: Finish upload
	finishData := url.Values{
		"access_token": {accessToken},
		"upload_phase": {"finish"},
		"video_id":     {videoID},
	}

	finishResp, err := http.PostForm(
		fmt.Sprintf("%s/%s/act_%s/advideos", metaAPIBase, metaAPIVersion, adAccountID),
		finishData,
	)
	if err != nil {
		return fmt.Errorf("finish upload: %w", err)
	}
	defer finishResp.Body.Close()

	var finishResult struct {
		Success bool `json:"success"`
		Error   struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}

	json.NewDecoder(finishResp.Body).Decode(&finishResult)

	if finishResult.Error.Code != 0 {
		return fmt.Errorf("finish error %d: %s", finishResult.Error.Code, finishResult.Error.Message)
	}

	// Step 4: Get thumbnail
	thumbnailURL := fmt.Sprintf(
		"%s/%s/%s?fields=picture&access_token=%s",
		metaAPIBase, metaAPIVersion, videoID, accessToken,
	)

	thumbResp, err := http.Get(thumbnailURL)
	var thumbnail string
	if err == nil {
		defer thumbResp.Body.Close()
		var thumbResult struct {
			Picture string `json:"picture"`
		}
		if json.NewDecoder(thumbResp.Body).Decode(&thumbResult) == nil {
			thumbnail = thumbResult.Picture
		}
	}

	return models.UpdateCreativeAfterUpload(creative.ID, "", videoID, thumbnail, "done")
}

// ProcessImageUpload handles image upload job
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

// ProcessVideoUpload handles video upload job
func ProcessVideoUpload(creative *models.Creative, accessToken, adAccountID, tempFilePath string) {
	defer os.Remove(tempFilePath)

	_ = models.UpdateCreativeStatus(creative.ID, "uploading", "")

	if err := UploadVideoResumable(accessToken, adAccountID, tempFilePath, creative); err != nil {
		_ = models.UpdateCreativeStatus(creative.ID, "failed", err.Error())
	}
}
