package auth

import (
	"log"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/utils"
)

// issueTokens membuat access token + refresh token untuk user, lalu menyimpan
// refresh token (hash) ke database. Mengembalikan keduanya.
//
// Safety guard: kalau penyimpanan refresh token gagal (misal tabel refresh_tokens
// belum dibuat), fungsi ini tetap mengembalikan access token — login tidak boleh
// gagal hanya karena fitur tambahan refresh token. Error hanya dikembalikan
// untuk kegagalan generate access token.
func issueTokens(userID uint) (accessToken, refreshToken string, err error) {
	accessToken, err = utils.GenerateJWT(userID)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = utils.GenerateRefreshToken()
	if err != nil {
		log.Println("[ERROR] Gagal generate refresh token: " + err.Error())
		return accessToken, "", nil
	}

	ttlDays := config.EnvInt("REFRESH_TOKEN_TTL_DAYS", 365)
	rt := models.RefreshToken{
		UserID:    userID,
		TokenHash: utils.HashToken(refreshToken),
		ExpiresAt: time.Now().Add(time.Duration(ttlDays) * 24 * time.Hour),
	}
	if err := database.DB.Create(&rt).Error; err != nil {
		log.Println("[ERROR] Gagal menyimpan refresh token: " + err.Error())
		return accessToken, "", nil
	}

	return accessToken, refreshToken, nil
}
