package byok

import (
	"encoding/json"
	"net/http"

	"halostory_go_backend/internal/services"

	"github.com/gin-gonic/gin"
)

func ChatStatus(c *gin.Context) {
	uuid := c.Param("uuid")

	data, err := services.GetChatResultResilient(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "not_found",
			"message": "Chat request not found or expired",
		})
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to parse chat result",
		})
		return
	}

	if tid, ok := result["trace_id"].(string); ok && tid != "" {
		c.Header("X-Trace-Id", tid)
		c.Header("X-Request-Id", tid)
	}

	status, _ := result["status"].(string)
	if status == "completed" {
		statusCode := http.StatusOK
		if code, ok := result["status_code"].(float64); ok {
			statusCode = int(code)
		}

		responseData := result["response"]
		c.JSON(statusCode, responseData)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": status,
		"uuid":   uuid,
	})
}
