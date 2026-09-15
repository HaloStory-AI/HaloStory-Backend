package services

// DB fallback untuk antrean chat saat Redis down.
// Path normal (Redis sehat): fungsi *Resilient* berperilaku identik dengan
// fungsi Redis asli — DB tidak tersentuh (zero overhead).
// Saat Redis gagal: tulis/baca ke tabel chat_jobs, worker klaim via ClaimDbJob.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// staleClaimAfter — job 'processing' yang terkunci lebih lama dari ini
// dianggap yatim (worker mati saat Redis down) dan boleh diklaim ulang.
const staleClaimAfter = 5 * time.Minute

// dbSweepScanLimit — batas klaim per panggilan ClaimDbJob.
const dbClaimAttempts = 3

// IsRedisDown membedakan Redis benar-benar down (network/timeout/closed)
// dari error non-koneksi (redis.Nil = key tidak ada = Redis sehat,
// WRONGPASS/NOAUTH = config error, tetap fallback agar user tidak 500
// tapi dicatat CRITICAL oleh pemanggil via log).
func IsRedisDown(err error) bool {
	if err == nil || err == redis.Nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, s := range []string{
		"connect", "connection", "refused", "reset", "timeout",
		"deadline", "closed", "eof", "no such host", "network",
		"broken pipe", "i/o",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	// Error Redis lain yang tidak dikenal saat operasi tulis/baca —
	// anggap down agar user tetap terlayani via DB, dengan log keras.
	return true
}

// IsRedisUp mengecek konektivitas Redis dengan timeout singkat.
// Dipakai stream controller untuk memilih mode SSE vs polling.
func IsRedisUp() bool {
	ctx, cancel := context.WithTimeout(RedisCtx, 2*time.Second)
	defer cancel()
	return RedisClient.Ping(ctx).Err() == nil
}

func logRedisFail(op, uuid string, err error) {
	msg := ""
	if err != nil {
		msg = strings.ToLower(err.Error())
	}
	level := "WARN"
	if strings.Contains(msg, "wrongpass") || strings.Contains(msg, "noauth") || strings.Contains(msg, "auth") {
		level = "CRITICAL (cek REDIS_PASSWORD, kemungkinan config mismatch)"
	}
	log.Printf("ChatFallback [%s] %s uuid=%s err=%v — alih ke DB", level, op, uuid, err)
}

// StoreChatRequest menyimpan payload chat: coba Redis dulu (sekali + 1 retry
// cepat), gagal → INSERT ke chat_jobs. Return viaDB=true jika lewat DB.
// Error hanya jika Redis DAN DB sama-sama gagal.
func StoreChatRequest(uuid string, payload []byte, ttl time.Duration) (viaDB bool, err error) {
	if err := StoreChatResult(uuid, payload, ttl); err == nil {
		return false, nil
	} else {
		logRedisFail("store", uuid, err)
		time.Sleep(200 * time.Millisecond)
		if err2 := StoreChatResult(uuid, payload, ttl); err2 == nil {
			return false, nil
		} else {
			logRedisFail("store-retry", uuid, err2)
		}
	}

	job := models.ChatJob{
		UUID:    uuid,
		Payload: string(payload),
		Status:  models.ChatJobQueued,
	}
	if dbErr := database.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&job).Error; dbErr != nil {
		return true, fmt.Errorf("redis down dan db insert gagal: %w", dbErr)
	}
	log.Printf("ChatFallback [WARN] store uuid=%s via DB (Redis down)", uuid)
	return true, nil
}

// EnqueueResilient antrekan job: coba Redis, gagal → pastikan baris DB ada
// (INSERT DO NOTHING — baris mungkin sudah dibuat StoreChatRequest).
// Return nil jika job aman di salah satu store.
func EnqueueResilient(queueName, uuid string, payload []byte) error {
	if err := Enqueue(queueName, payload); err == nil {
		return nil
	} else {
		logRedisFail("enqueue", uuid, err)
	}

	job := models.ChatJob{
		UUID:    uuid,
		Payload: string(payload),
		Status:  models.ChatJobQueued,
	}
	if dbErr := database.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&job).Error; dbErr != nil {
		return fmt.Errorf("redis down dan db insert gagal: %w", dbErr)
	}
	log.Printf("ChatFallback [WARN] enqueue uuid=%s via DB (Redis down)", uuid)
	return nil
}

// GetChatResultResilient membaca hasil: Redis dulu, gagal/hilang → DB.
func GetChatResultResilient(uuid string) ([]byte, error) {
	if data, err := GetChatResult(uuid); err == nil {
		return data, nil
	}
	return GetChatResultFallback(uuid)
}

// GetChatResultFallback membangun envelope yang identik dengan format Redis
// (payload + status override + response/status_code saat completed).
func GetChatResultFallback(uuid string) ([]byte, error) {
	var job models.ChatJob
	if err := database.DB.Where("uuid = ?", uuid).First(&job).Error; err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(job.Payload), &m); err != nil {
		return nil, fmt.Errorf("payload db corrupt untuk %s: %w", uuid, err)
	}
	m["status"] = string(job.Status)
	if job.Status == models.ChatJobCompleted || job.Status == models.ChatJobFailed {
		if job.Response != nil && *job.Response != "" {
			var resp interface{}
			if err := json.Unmarshal([]byte(*job.Response), &resp); err == nil {
				m["response"] = resp
			} else {
				m["response"] = *job.Response
			}
		}
		m["status_code"] = job.StatusCode
	}
	return json.Marshal(m)
}

// MarkChatCompletedResilient menulis hasil akhir: Redis dulu, gagal → DB.
// Baris DB yang tidak ada (job murni Redis) diabaikan diam-diam.
func MarkChatCompletedResilient(uuid string, response json.RawMessage, statusCode int) error {
	if err := MarkChatCompleted(uuid, response, statusCode); err == nil {
		return nil
	} else {
		logRedisFail("mark-completed", uuid, err)
	}
	return MarkChatCompletedFallback(uuid, response, statusCode)
}

// MarkChatCompletedFallback UPSERT baris chat_jobs: UPDATE jika baris DB ada
// (payload request asli dipertahankan), INSERT minimal jika job murni Redis
// yang selesai saat Redis down (mis. Redis flap di tengah proses).
func MarkChatCompletedFallback(uuid string, response json.RawMessage, statusCode int) error {
	resp := string(response)
	job := models.ChatJob{
		UUID:       uuid,
		Payload:    fmt.Sprintf(`{"uuid":%q,"status":"completed"}`, uuid),
		Status:     models.ChatJobCompleted,
		Response:   &resp,
		StatusCode: statusCode,
	}
	return database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uuid"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "response", "status_code", "locked_by", "locked_at", "updated_at"}),
	}).Create(&job).Error
}

// StoreProcessingResilient menandai job processing: Redis dulu, gagal → UPDATE
// baris DB (khusus job via DB; job Redis diabaikan).
func StoreProcessingResilient(uuid string, data []byte) {
	if err := StoreChatResult(uuid, data, 10*time.Minute); err == nil {
		return
	} else {
		logRedisFail("mark-processing", uuid, err)
	}
	_ = database.DB.Model(&models.ChatJob{}).
		Where("uuid = ? AND status = ?", uuid, string(models.ChatJobQueued)).
		Update("status", string(models.ChatJobProcessing)).Error
}

// ClaimDbJob diklaim worker saat Redis down: ambil 1 job queued (atau
// processing yatim > staleClaimAfter) secara atomik. Aman multi-worker/VPS:
// hanya 1 UPDATE kondisional yang menang (RowsAffected==1).
// Return nil, nil jika tidak ada job.
func ClaimDbJob(workerID string) ([]byte, error) {
	staleBefore := time.Now().Add(-staleClaimAfter)
	for i := 0; i < dbClaimAttempts; i++ {
		var cand models.ChatJob
		err := database.DB.Select("uuid").
			Where("status = ? OR (status = ? AND (locked_at IS NULL OR locked_at < ?))",
				string(models.ChatJobQueued), string(models.ChatJobProcessing), staleBefore).
			Order("created_at ASC").Limit(1).First(&cand).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, nil // tidak ada job
			}
			return nil, err // DB ikut bermasalah — pemanggil backoff
		}

		now := time.Now()
		res := database.DB.Model(&models.ChatJob{}).
			Where("uuid = ? AND (status = ? OR (status = ? AND (locked_at IS NULL OR locked_at < ?)))",
				cand.UUID, string(models.ChatJobQueued), string(models.ChatJobProcessing), staleBefore).
			Updates(map[string]interface{}{
				"status":    string(models.ChatJobProcessing),
				"locked_by": workerID,
				"locked_at": now,
			})
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected == 0 {
			continue // kalah race dengan worker lain, coba kandidat berikutnya
		}
		var full models.ChatJob
		if err := database.DB.Where("uuid = ?", cand.UUID).First(&full).Error; err != nil {
			return nil, err
		}
		log.Printf("ChatFallback: worker %s klaim job DB %s", workerID, cand.UUID)
		return []byte(full.Payload), nil
	}
	return nil, nil
}

// CleanExpiredChatJobs menghapus job completed/failed > 30 menit.
// Dipanggil scheduler tiap 10 menit (pengganti TTL Redis 10 mnt + buffer baca).
func CleanExpiredChatJobs() {
	log.Println("Scheduler: Cleaning expired chat_jobs...")
	res := database.DB.Exec(
		"DELETE FROM chat_jobs WHERE status IN ('completed','failed') AND updated_at < NOW() - INTERVAL 30 MINUTE",
	)
	if res.Error != nil {
		log.Printf("Scheduler: Error cleaning chat_jobs: %v", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		log.Printf("Scheduler: Deleted %d expired chat job(s)", res.RowsAffected)
	}
}
