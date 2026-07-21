package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"halostory_go_backend/internal/config"
)

type laravelCryptPayload struct {
	IV    string `json:"iv"`
	Value string `json:"value"`
	MAC   string `json:"mac"`
}

func getEncryptionKey() ([]byte, error) {
	appKey := config.Env("APP_KEY", "")
	if appKey == "" {
		return nil, errors.New("APP_KEY is not set")
	}

	if strings.HasPrefix(appKey, "base64:") {
		decoded, err := base64.StdEncoding.DecodeString(appKey[7:])
		if err != nil {
			return nil, err
		}
		if len(decoded) != 32 {
			return nil, errors.New("APP_KEY decoded length must be 32 bytes for AES-256-CBC")
		}
		return decoded, nil
	}

	hash := sha256.Sum256([]byte(appKey))
	return hash[:], nil
}

func EncryptBYOKAPIKey(plaintext string) (string, error) {
	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}

	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)
	ciphertext := make([]byte, len(padded))

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	base64IV := base64.StdEncoding.EncodeToString(iv)
	base64Value := base64.StdEncoding.EncodeToString(ciphertext)

	mac := computeHMAC(key, base64IV, base64Value)

	payload := laravelCryptPayload{
		IV:    base64IV,
		Value: base64Value,
		MAC:   mac,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(jsonBytes), nil
}

func DecryptBYOKAPIKey(encrypted string) (string, error) {
	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}

	jsonBytes, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	var payload laravelCryptPayload
	if err := json.Unmarshal(jsonBytes, &payload); err != nil {
		return "", err
	}

	expectedMAC := computeHMAC(key, payload.IV, payload.Value)
	if !hmac.Equal([]byte(payload.MAC), []byte(expectedMAC)) {
		return "", errors.New("MAC mismatch — encrypted data may have been tampered with")
	}

	iv, err := base64.StdEncoding.DecodeString(payload.IV)
	if err != nil {
		return "", err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(payload.Value)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < aes.BlockSize || len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext is not a multiple of block size")
	}

	padded := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(padded, ciphertext)

	plaintext, err := pkcs7Unpad(padded)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func computeHMAC(key []byte, iv, value string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(iv + value))
	return hex.EncodeToString(mac.Sum(nil))
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padBytes := make([]byte, padding)
	for i := range padBytes {
		padBytes[i] = byte(padding)
	}
	return append(data, padBytes...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}
	padding := int(data[len(data)-1])
	if padding > len(data) || padding > aes.BlockSize {
		return nil, errors.New("invalid padding")
	}
	return data[:len(data)-padding], nil
}
