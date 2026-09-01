package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"halostory_go_backend/internal/services/prompt"

	"github.com/gin-gonic/gin"
)

func ListPromptTemplates(c *gin.Context) {
	items := prompt.LoadDefault()
	c.JSON(http.StatusOK, gin.H{"templates": items})
}

func RenderPromptPreview(c *gin.Context) {
	var body struct {
		CharName         string `json:"char_name"`
		UserName         string `json:"user_name"`
		BirthdayInfo     string `json:"birthday_info"`
		Language         string `json:"language"`
		Persona          string `json:"persona"`
		UserPersona      string `json:"user_persona"`
		ExampleDialogue  string `json:"example_dialogue"`
		Summary          string `json:"summary"`
		GroupContext     string `json:"group_context"`
		LorebookInjection string `json:"lorebook_injection"`
		BondLevel        int    `json:"bond_level"`
		Items            []prompt.PromptItem `json:"items"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// block {{system}}
	raw, _ := json.Marshal(body.Items)
	if strings.Contains(strings.ToLower(string(raw)), "{{system") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "blocked pattern {{system}}"})
		return
	}
	items := body.Items
	if len(items) == 0 {
		items = prompt.LoadDefault()
	}
	// limit 8k per item
	for _, it := range items {
		if len(it.Content) > 8000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "content too long >8000"})
			return
		}
	}
	data := prompt.PromptData{
		CharName: body.CharName, UserName: body.UserName, BirthdayInfo: body.BirthdayInfo,
		Language: body.Language, Persona: body.Persona, UserPersona: body.UserPersona,
		ExampleDialogue: body.ExampleDialogue, Summary: body.Summary, GroupContext: body.GroupContext,
		LorebookInjection: body.LorebookInjection, BondLevel: body.BondLevel,
		HasBirthday: body.BirthdayInfo != "", HasSummary: body.Summary != "", HasPersona: body.Persona != "",
		HasGroup: body.GroupContext != "", HasLorebook: body.LorebookInjection != "", HasBond: body.BondLevel > 0,
	}
	rendered := prompt.Render(data, items)
	c.JSON(http.StatusOK, gin.H{"rendered": rendered, "tokens": len(rendered)/4, "items_rendered": len(items)})
}

func UpdatePromptTemplate(c *gin.Context) {
	var items []prompt.PromptItem
	if err := c.ShouldBindJSON(&items); err != nil {
		// try wrapper {templates: [...]}
		var wrap struct { Templates []prompt.PromptItem `json:"templates"` }
		if err2 := c.ShouldBindJSON(&wrap); err2 != nil || len(wrap.Templates) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		items = wrap.Templates
	}
	// validate
	raw, _ := json.Marshal(items)
	if strings.Contains(strings.ToLower(string(raw)), "{{system") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "blocked pattern {{system}}"})
		return
	}
	for _, it := range items {
		if len(it.Content) > 8000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "content too long >8000"})
			return
		}
		if it.Content != "" && it.Content != strings.TrimSpace(it.Content) {
			// allow
		}
	}
	if err := prompt.SaveDefault(items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "saved", "count": len(items)})
}
