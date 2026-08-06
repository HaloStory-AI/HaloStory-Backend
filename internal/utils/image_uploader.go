package utils

import (
	"bytes"
	"fmt"
	"io"
	"encoding/json"
	"mime/multipart"
	"net/http"

	"github.com/google/uuid"
	"github.com/gabriel-vasile/mimetype"
)

type UploaderResponse struct {
	StatusCode int `json:"status_code"`
	Success    struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"success"`
	Image struct {
		Name             string      `json:"name"`
		Extension        string      `json:"extension"`
		Size             int         `json:"size"`
		Width            int         `json:"width"`
		Height           int         `json:"height"`
		Date             string      `json:"date"`
		DateGmt          string      `json:"date_gmt"`
		StorageID        interface{} `json:"storage_id"`
		Description      interface{} `json:"description"`
		Nsfw             string      `json:"nsfw"`
		Md5              string      `json:"md5"`
		Storage          string      `json:"storage"`
		OriginalFilename string      `json:"original_filename"`
		OriginalExifdata interface{} `json:"original_exifdata"`
		Views            string      `json:"views"`
		IDEncoded        string      `json:"id_encoded"`
		Filename         string      `json:"filename"`
		Ratio            float64     `json:"ratio"`
		SizeFormatted    string      `json:"size_formatted"`
		Mime             string      `json:"mime"`
		Bits             int         `json:"bits"`
		Channels         interface{} `json:"channels"`
		URL              string      `json:"url"`
		URLViewer        string      `json:"url_viewer"`
		Thumb            struct {
			Filename      string      `json:"filename"`
			Name          string      `json:"name"`
			Width         int         `json:"width"`
			Height        int         `json:"height"`
			Ratio         int         `json:"ratio"`
			Size          int         `json:"size"`
			SizeFormatted string      `json:"size_formatted"`
			Mime          string      `json:"mime"`
			Extension     string      `json:"extension"`
			Bits          int         `json:"bits"`
			Channels      interface{} `json:"channels"`
			URL           string      `json:"url"`
		} `json:"thumb"`
		Medium struct {
			Filename      string      `json:"filename"`
			Name          string      `json:"name"`
			Width         int         `json:"width"`
			Height        int         `json:"height"`
			Ratio         float64     `json:"ratio"`
			Size          int         `json:"size"`
			SizeFormatted string      `json:"size_formatted"`
			Mime          string      `json:"mime"`
			Extension     string      `json:"extension"`
			Bits          int         `json:"bits"`
			Channels      interface{} `json:"channels"`
			URL           string      `json:"url"`
		} `json:"medium"`
		ViewsLabel string `json:"views_label"`
		DisplayURL string `json:"display_url"`
		HowLongAgo string `json:"how_long_ago"`
	} `json:"image"`
	StatusTxt string `json:"status_txt"`
}

func UploadPicture(picture io.Reader) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("key", "6d207e02198a847aa98d0a2a901485a5")
	writer.WriteField("action", "upload")
	writer.WriteField("format", "json")

	mType, err := mimetype.DetectReader(picture) 
	if err != nil {
		return "", fmt.Errorf("[ERROR] Mimetype Detect: %s", err.Error())
	}

	if seeker, ok := picture.(io.Seeker); ok {
		_, _ = seeker.Seek(0, io.SeekStart)
	}

	fileWritter, err := writer.CreateFormFile("source", uuid.New().String()+mType.Extension())
	if err != nil {
		return "", fmt.Errorf("[ERROR] FormFile: %s", err.Error())
	}

	_, err = io.Copy(fileWritter, picture)
	if err != nil {
		return "", fmt.Errorf("[ERROR] FileWritter: %s", err.Error())
	}

	writer.Close()

	httpRequest, err := http.NewRequest("POST", "https://freeimage.host/api/1/upload", &body)
	if err != nil {
		return "", fmt.Errorf("[ERROR] HttpRequest: %s", err.Error())
	}
	httpRequest.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return "", fmt.Errorf("[ERROR] Response Client: %s", err.Error())
	}
	defer response.Body.Close()

	resp, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("[ERROR] Read Response Bytes: %s", err.Error())
	}
	
	var uploadResponse UploaderResponse
	err = json.Unmarshal(resp, &uploadResponse)
	if err != nil {
		return "", fmt.Errorf("[ERROR] JSON: %s", err.Error())
	}
	if uploadResponse.StatusCode > 400 {
		return "", fmt.Errorf("[ERROR] Response Provider")
	}

	return uploadResponse.Image.DisplayURL, nil
}
