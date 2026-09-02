package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"halostory_go_backend/internal/services/prompt"

	"github.com/gin-gonic/gin"
)

type listTemplatesResponse struct {
	Templates []prompt.PromptItem `json:"templates"`
}

type renderResponse struct {
	Rendered     string `json:"rendered"`
	Tokens       int    `json:"tokens"`
	ItemsRendered int   `json:"items_rendered"`
}

type saveResponse struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func ListPromptTemplates(c *gin.Context) {
	items := prompt.LoadDefault()
	if items == nil {
		items = make([]prompt.PromptItem, 0)
	}
	c.JSON(http.StatusOK, listTemplatesResponse{Templates: items})
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
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	// block {{system}} and <script> in items
	raw, _ := json.Marshal(body.Items)
	lower := strings.ToLower(string(raw))
	if strings.Contains(lower, "{{system") {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "blocked pattern {{system}}"})
		return
	}
	if strings.Contains(lower, "<script") {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "blocked pattern <script"})
		return
	}
	items := body.Items
	if len(items) == 0 {
		items = prompt.LoadDefault()
	}
	// limit 8k per item
	for _, it := range items {
		if len(it.Content) > 8000 {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "content too long >8000"})
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
	c.JSON(http.StatusOK, renderResponse{Rendered: rendered, Tokens: len(rendered) / 4, ItemsRendered: len(items)})
}

func UpdatePromptTemplate(c *gin.Context) {
	rawBody, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "failed to read body"})
		return
	}
	// restore body for potential re-read
	c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))

	var items []prompt.PromptItem
	var wrap struct {
		Templates []prompt.PromptItem `json:"templates"`
	}
	// try array first
	if err := json.Unmarshal(rawBody, &items); err != nil || len(items) == 0 {
		// try wrapper
		if err2 := json.Unmarshal(rawBody, &wrap); err2 != nil || len(wrap.Templates) == 0 {
			// if raw is empty array "[]", treat as empty
			if string(bytes.TrimSpace(rawBody)) == "[]" {
				items = make([]prompt.PromptItem, 0)
			} else {
				msg := err.Error()
				if err2 != nil {
					msg = err2.Error()
				}
				c.JSON(http.StatusBadRequest, errorResponse{Error: msg})
				return
			}
		} else {
			items = wrap.Templates
		}
	}
	// validate via prompt.Validate (role/condition/block/len)
	if err := prompt.Validate(items); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if err := prompt.SaveDefault(items); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, saveResponse{Status: "saved", Count: len(items)})
}
