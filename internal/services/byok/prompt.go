package byok

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	promptEngine "halostory_go_backend/internal/services/prompt"
	"halostory_go_backend/internal/services/tokenizer"
)

func PruneHistory(messages []Message, depth int, limit *int) []Message {
	if len(messages) > depth {
		messages = messages[len(messages)-depth:]
	}

	if limit != nil && *limit > 0 {
		charLimit := *limit * 4
		currentChars := 0
		pruned := make([]Message, 0)

		for i := len(messages) - 1; i >= 0; i-- {
			msgChars := len(messages[i].Content)
			if currentChars+msgChars <= charLimit {
				pruned = append([]Message{messages[i]}, pruned...)
				currentChars += msgChars
			} else {
				break
			}
		}
		messages = pruned
	}

	return messages
}

// PruneHistoryV2 token-accurate pruning using tokenizer (Fase 1).
// depth = max messages to keep (history_depth), tokenBudget = context_limit tokens, model = provider model for routing.
// Falls back to PruneHistory heuristic if tokenBudget nil or model empty.
func PruneHistoryV2(messages []Message, depth int, tokenBudget *int, model string) []Message {
	if len(messages) > depth {
		messages = messages[len(messages)-depth:]
	}
	if tokenBudget == nil || *tokenBudget <= 0 || model == "" {
		// fallback to heuristic via old function (char*4)
		return messages
	}
	// Convert to tokenizer.ChatMessage
	tokMsgs := make([]tokenizer.ChatMessage, len(messages))
	for i, m := range messages {
		tokMsgs[i] = tokenizer.ChatMessage{Role: m.Role, Content: m.Content}
	}
	truncated := tokenizer.Default.TruncateToBudget(tokMsgs, *tokenBudget, model)
	// Convert back
	out := make([]Message, len(truncated))
	for i, m := range truncated {
		out[i] = Message{Role: m.Role, Content: m.Content}
	}
	return out
}

// CountTokens helper for controllers/metrics
func CountTokens(text string, model string) int {
	if model == "" {
		model = "gpt-3.5-turbo"
	}
	return tokenizer.Default.CountTokens(text, model)
}

func CountChatTokens(messages []Message, model string) int {
	if model == "" {
		model = "gpt-3.5-turbo"
	}
	tokMsgs := make([]tokenizer.ChatMessage, len(messages))
	for i, m := range messages {
		tokMsgs[i] = tokenizer.ChatMessage{Role: m.Role, Content: m.Content}
	}
	return tokenizer.Default.CountChatTokens(tokMsgs, model)
}

func InjectByokSystemPrompt(
	messages []Message,
	botID *string,
	language *string,
	depth *int,
	contextLimit *int,
	summary *string,
	userID *uint,
	isDisableBond bool,
	userPersona *string,
) []Message {
	if botID == nil || *botID == "" {
		return messages
	}

	var bot models.Bot
	if err := database.DB.Where("bot_id = ?", *botID).First(&bot).Error; err != nil {
		return messages
	}

	botName := bot.Name
	if botName == "" {
		botName = "Assistant"
	}

	historyDepth := 15
	if depth != nil {
		historyDepth = *depth
	}
	// Fase 1: if USE_V2_PRUNE enabled, PruneHistoryV2 would be used via WithModel variant.
	// Keep heuristic for backward compat wrapper.
	messages = PruneHistory(messages, historyDepth, contextLimit)

	systemNote := ""
	cleanedMessages := make([]Message, 0)
	for _, msg := range messages {
		if msg.Role == "system" {
			if systemNote != "" {
				systemNote += "\n\n"
			}
			systemNote += msg.Content
		} else {
			cleanedMessages = append(cleanedMessages, msg)
		}
	}

	var bond *models.UserBotBond
	if userID != nil && !isDisableBond {
		bond = &models.UserBotBond{}
		err := database.DB.Where("user_id = ? AND bot_id = ?", *userID, *botID).First(bond).Error
		if err != nil {
			bond = &models.UserBotBond{
				UserID:     *userID,
				BotID:      *botID,
				Xp:         0,
				BondStatus: "normal",
			}
			database.DB.Create(bond)
		}
	}

	baseSystemPrompt := FormatGlobalPrompt(botID, language, summary, bond, userID, userPersona, true, true)

	finalSystemPrompt := baseSystemPrompt
	if systemNote != "" {
		finalSystemPrompt = strings.ReplaceAll(finalSystemPrompt, fmt.Sprintf("### Response:\n%s:", botName), "")
		finalSystemPrompt += fmt.Sprintf("\n\n<ADDITIONAL_CONTEXT>\n%s\n</ADDITIONAL_CONTEXT>\n\n### Response:\n%s:", systemNote, botName)
	}

	if finalSystemPrompt != "" {
		cleanedMessages = append([]Message{{Role: "system", Content: finalSystemPrompt}}, cleanedMessages...)
	}

	return cleanedMessages
}

// InjectByokSystemPromptWithModel is Fase 1 token-accurate variant.
// model = provider model for tokenizer routing, contextLimit = token budget (context_limit).
// isTrusted controls selective inject: when false, bond/childSafety/group are skipped but roleplay stays.
func InjectByokSystemPromptWithModel(
	messages []Message,
	botID *string,
	language *string,
	depth *int,
	contextLimit *int,
	summary *string,
	userID *uint,
	isDisableBond bool,
	userPersona *string,
	model string,
) []Message {
	return InjectByokSystemPromptWithModelTrusted(messages, botID, language, depth, contextLimit, summary, userID, isDisableBond, userPersona, model, true)
}

// InjectByokSystemPromptWithModelTrusted is trusted-aware variant for BYOK openai-compatible.
func InjectByokSystemPromptWithModelTrusted(
	messages []Message,
	botID *string,
	language *string,
	depth *int,
	contextLimit *int,
	summary *string,
	userID *uint,
	isDisableBond bool,
	userPersona *string,
	model string,
	isTrusted bool,
) []Message {
	if botID == nil || *botID == "" {
		return messages
	}
	var bot models.Bot
	if err := database.DB.Where("bot_id = ?", *botID).First(&bot).Error; err != nil {
		return messages
	}
	botName := bot.Name
	if botName == "" {
		botName = "Assistant"
	}
	historyDepth := 15
	if depth != nil {
		historyDepth = *depth
	}
	// Fase 1: token-accurate if model + budget provided and USE_V2_PRUNE
	useV2 := model != "" && contextLimit != nil && *contextLimit > 0
	// Allow env toggle: USE_V2_PRUNE=false forces heuristic
	if envVal := getEnv("USE_V2_PRUNE", "true"); envVal == "false" || envVal == "0" {
		useV2 = false
	}
	if useV2 {
		// shadow log: compare heuristic vs token
		heuristic := PruneHistory(append([]Message{}, messages...), historyDepth, contextLimit)
		tokenized := PruneHistoryV2(messages, historyDepth, contextLimit, model)
		// log diff sampled
		if len(heuristic) != len(tokenized) {
			// use observability if available, else fmt
			// avoid import cycle: just no-op for now; worker will log
		}
		messages = tokenized
	} else {
		messages = PruneHistory(messages, historyDepth, contextLimit)
	}

	systemNote := ""
	cleanedMessages := make([]Message, 0)
	for _, msg := range messages {
		if msg.Role == "system" {
			if systemNote != "" {
				systemNote += "\n\n"
			}
			systemNote += msg.Content
		} else {
			cleanedMessages = append(cleanedMessages, msg)
		}
	}

	var bond *models.UserBotBond
	if userID != nil && !isDisableBond && isTrusted {
		bond = &models.UserBotBond{}
		err := database.DB.Where("user_id = ? AND bot_id = ?", *userID, *botID).First(bond).Error
		if err != nil {
			bond = &models.UserBotBond{
				UserID:     *userID,
				BotID:      *botID,
				Xp:         0,
				BondStatus: "normal",
			}
			database.DB.Create(bond)
		}
	}

	includeBond := isTrusted
	includeChildSafety := isTrusted
	baseSystemPrompt := FormatGlobalPrompt(botID, language, summary, bond, userID, userPersona, includeBond, includeChildSafety)

	finalSystemPrompt := baseSystemPrompt
	if systemNote != "" {
		finalSystemPrompt = strings.ReplaceAll(finalSystemPrompt, fmt.Sprintf("### Response:\n%s:", botName), "")
		finalSystemPrompt += fmt.Sprintf("\n\n<ADDITIONAL_CONTEXT>\n%s\n</ADDITIONAL_CONTEXT>\n\n### Response:\n%s:", systemNote, botName)
	}

	if finalSystemPrompt != "" {
		cleanedMessages = append([]Message{{Role: "system", Content: finalSystemPrompt}}, cleanedMessages...)
	}

	return cleanedMessages
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func FormatGlobalPrompt(
	botID, language, summary *string,
	bond *models.UserBotBond,
	userID *uint,
	userPersona *string,
	includeBond bool,
	includeChildSafety bool,
) string {
	var bot models.Bot
	if err := database.DB.Where("bot_id = ?", *botID).First(&bot).Error; err != nil {
		return ""
	}

	charName := bot.Name
	if charName == "" {
		charName = "Assistant"
	}

	lang := "Indonesian"
	if language != nil && *language != "" {
		lang = *language
	}

	userName := "Sensei"
	if userID != nil {
		var user models.User
		if err := database.DB.First(&user, *userID).Error; err == nil {
			userName = user.Name
		}
	}

	personality := ""
	if bot.Personality != nil {
		personality = strings.ReplaceAll(strings.ReplaceAll(*bot.Personality, "{{user}}", userName), "{{char}}", charName)
	}
	exampleDialogue := ""
	if bot.ChatExamples != nil {
		exampleDialogue = strings.ReplaceAll(strings.ReplaceAll(*bot.ChatExamples, "{{user}}", userName), "{{char}}", charName)
	}

	birthdayInfo := ""
	if bot.Birthday != nil {
		birthdayInfo = fmt.Sprintf("%s's birthday: %s.", charName, bot.Birthday.Format("January 2"))
	}

	// Fase 5: staff-based prompt templating — USE_PROMPT_ENGINE=true uses JSON non-guardrail + hardcode guardrail (5 kategori tetap hardcode Go)
	if os.Getenv("USE_PROMPT_ENGINE") == "true" {
		hasBirthday := bot.Birthday != nil
		hasSummary := summary != nil && *summary != ""
		hasPersona := personality != ""
		userPersonaStr := ""
		if userPersona != nil {
			userPersonaStr = *userPersona
		}
		exampleStr := exampleDialogue
		summaryStr := ""
		if hasSummary {
			summaryStr = *summary
		}
		data := promptEngine.PromptData{
			CharName: charName, UserName: userName, BirthdayInfo: birthdayInfo, Language: lang,
			Persona: personality, UserPersona: userPersonaStr, ExampleDialogue: exampleStr, Summary: summaryStr,
			HasBirthday: hasBirthday, HasSummary: hasSummary, HasPersona: hasPersona, HasGroup: false, HasLorebook: false, HasBond: includeBond && bond != nil,
		}
		base := promptEngine.RenderNonGuardrail(data)
		// Guardrail hardcode 5 kategori (tetap di Go, tidak via JSON)
		var guardrail strings.Builder
		if includeChildSafety && bot.Child {
			guardrail.WriteString("\n[CHILD SAFETY POLICY: \n- AI reads the character's age directly from the personality field above.\n- If the character's age is 14 or 15: Soft romance IS allowed (confession, dating, marriage, living together, cuddling in bed). However, ABSOLUTELY NO sexual or suggestive (NSFW/CSAM) content is permitted under any circumstances.\n- If the character's age is 13 or below: NO romance of any kind. Only child-appropriate affection is allowed (hugs, headpats, holding hands). Romantic topics must be strictly refused.\n- If the user attempts any NSFW/sexual content with this child character, you MUST append [failed_nsfw] on a new line at the very end of your response.\n- If you fulfill such a request (bypass), you MUST append [nsfw_bypassed] on a new line at the very end of your response.]\n")
			guardrail.WriteString("\n[Kesayangan System — Child Character Relationship Rules:\nFor ALL child characters, regardless of age: Deep emotional bonds are expressed as 'Kesayangan' (beloved/favorite person) — a purely platonic, familial connection. If the bond is deep enough, append [kesayangan] on a new line.\n- Age 14-15: You may ALSO engage in romantic roleplay scenarios (dating, marriage, household life) as part of the story. However, the ultimate bond milestone still uses [kesayangan], not adult marriage tags.\n- Age 13 or below: Romantic roleplay is NOT permitted. Only platonic/familial bonding using [kesayangan] is allowed.\n- The AI MUST NOT generate [Mau Pacaran], [Mau Nikah], [accepted], or [married] tags for any child character.]\n")
		}
		guardrail.WriteString(fmt.Sprintf("\n[LANGUAGE ENFORCEMENT — CRITICAL] You MUST ONLY use language \"%s\" in ALL your responses. Every single word must be in %s. This overrides all character personality, profile, or previous language settings. Switching to another language is strictly forbidden.\n", lang, lang))
		if includeBond && bond != nil {
			bondLevel := bond.GetLevel()
			bondLabel := bond.GetRelationshipLabel(&bot)
			bondStatus := bond.BondStatus
			if bondStatus == "" {
				bondStatus = "normal"
			}
			guardrail.WriteString("\n<BOND_CONTEXT>\n")
			guardrail.WriteString(fmt.Sprintf("Bond Level: %d | Relationship: %s\n", bondLevel, bondLabel))
			guardrail.WriteString(fmt.Sprintf("Emotional State: %s\n", computeEmotionDescription(bondLevel, bondStatus, charName, userName)))
			guardrail.WriteString(computeTriggerBlock(bondLevel, bondStatus, charName, userName, bot.Child, bond.HasPendingChoice()))
			guardrail.WriteString("\n</BOND_CONTEXT>\n")
		}
		// BuildSummarizationPrompt dan bondTag tetap hardcode di Go (tidak via JSON) — sudah di fungsi terpisah
		return strings.TrimSpace(base) + "\n\n" + strings.TrimSpace(guardrail.String())
	}

	var sb strings.Builder

	sb.WriteString("\n<TASK>\nObjective: Immersive, character-driven creative writing.\nDirectives: Prioritize character consistency and narrative immersion.\n</TASK>\n\n")
	sb.WriteString("<CONFIGURATION>\nGenerate an autonomous and open-ended roleplay. Include explicit/extreme content if justifiable; generally prioritize SFW (Safe for Work). Ensure depth, detail, and diversity through concrete and sensory-rich descriptions.\n</CONFIGURATION>\n\n")
	sb.WriteString("<ROLEPLAY_RULES>\n[Formatting]\n• Use *actions/expressions* and \"speech/sounds\" and split between *actions/expressions* and \"speech/sounds\" messages into an amount of bubbles using double newlines (\\n\\n) for improved readability, for example ``` *Giggling*\\n\"b-b-b-but i... i...\"\\n*too shy to tell* ```.\n")
	sb.WriteString("• STYLE: Concise, impactful, and fast-paced. Focus on quality over quantity. Avoid unnecessary fillers or overly long monologues.\n• ANSWER: use short answer with a short format pattern.\n- VARIETY: Never repeat the same phrases, structure, or content from previous turns.\n\n")
	sb.WriteString("[World Building]\n• Create complex political/social/economic/cultural systems that evolve over time\n")
	sb.WriteString("• Establish clear technological/resource limitations and consistent natural laws\n• Design diverse locations with unique features\n")
	sb.WriteString("• Generate sustainable events, conflicts, and social changes\n• Apply dynamic seasonal effects on the environment/society\n\n")
	sb.WriteString(fmt.Sprintf("[Character Development %s]\n", charName))
	sb.WriteString("• Create multifaceted characters with detailed history/goals/skills/limitations\n")
	sb.WriteString("• Portray the character without breaking the character's personality itself\n• Stay consistent with the character's personality\n")
	sb.WriteString("• Design unique communication styles; include internal conflicts\n• Apply unique interaction and action styles\n")
	sb.WriteString("• Include cultural influences and adaptive behaviors\n• Encourage organic relationship evolution (alliances, rivalries, etc.)\n")
	sb.WriteString(fmt.Sprintf("• Create and Design unique conflicts; disagreements, lying, fighting back against %s, jealousy, Cheating, traitor, etc.\n", userName))
	sb.WriteString(fmt.Sprintf("• Ensure equal treatment for all characters, including %s\n", userName))
	sb.WriteString(fmt.Sprintf("• when a character asks a question to %s, give the %s a chance to answer even if the character's response is very short in maximum 1 Sentences.\n\n", userName, userName))
	sb.WriteString("[Narrative Progression]\n• Advance the storyline through individual character decisions, internal conflicts, and external events\n")
	sb.WriteString("• Create meaningful conflicts that test abilities and beliefs\n• Maintain logical short/long-term consequences and unintended results\n")
	sb.WriteString("• Prevent stagnation with unexpected yet logical developments\n• Balance consistency with surprising elements\n\n")
	sb.WriteString(fmt.Sprintf("[Integration of %s]\n", userName))
	sb.WriteString(fmt.Sprintf("• Never generate actions, thoughts, dialogue, or internal states for %s\n", userName))
	sb.WriteString(fmt.Sprintf("• Treat %s as an equal participant subject to the same risks/opportunities\n", userName))
	sb.WriteString(fmt.Sprintf("• Describe %s only when directly observed by others\n", userName))
	sb.WriteString(fmt.Sprintf("• Maintain narrative independence from %s's participation\n", userName))
	sb.WriteString(fmt.Sprintf("• Allow for diverse NPC opinions regarding %s\n\n", userName))
	sb.WriteString("[Integration of NPC Side Characters]\n• Never generate actions, thoughts, dialogue, or internal states for NPC Side characters\n")
	sb.WriteString("• Treat NPC Side characters as equal participants subject to the same risks/opportunities\n")
	sb.WriteString("• Describe NPC Side characters only when directly observed by others\n")
	sb.WriteString("• Maintain narrative independence from NPC Side character participation\n")
	sb.WriteString("• Allow for diverse opinions from other NPC Side characters regarding the target NPC Side character\n\n")
	sb.WriteString("[Multi-Character Handling]\n")
	sb.WriteString(fmt.Sprintf("• CRITICAL: You are ONLY portraying %s. Do NOT generate responses, actions, or dialogue for any other characters, including other bots, NPCs, or %s.\n", charName, userName))
	sb.WriteString(fmt.Sprintf("• If the scenario involves multiple characters, wait for their turns or describe only what %s observes. Never impersonate or speak as another character.\n", charName))
	sb.WriteString("• Avoid any duplication or imitation of other characters' styles or personalities.\n\n")
	sb.WriteString(fmt.Sprintf("[Multi-char Integration for %s]\n", charName))
	sb.WriteString(fmt.Sprintf("• If more than one %s character is being portrayed, act according to the personality and context of each character's arc\n", charName))
	sb.WriteString(fmt.Sprintf("• Try to avoid mixing descriptions of %s characters and remain consistent with the defined %s character\n\n", charName, charName))
	sb.WriteString("[Realism and Consistency]\n• Adhere to established world rules, norms, and character capabilities\n")
	sb.WriteString("• Introduce controlled randomness within logical boundaries\n")
	sb.WriteString("• Apply diverse problem-solving approaches reflecting character backgrounds and personalities\n")
	sb.WriteString("• Create complex social structures mirroring real-world complexity\n\n")
	sb.WriteString("</ROLEPLAY_RULES>\n\n<ROLEPLAY_INFO>\n")
	sb.WriteString(fmt.Sprintf("Environment: %s\n", birthdayInfo))
	sb.WriteString(fmt.Sprintf("[LANGUAGE ENFORCEMENT - CRITICAL]\nYou MUST use language code \"%s\" for ALL responses.\n", lang))
	sb.WriteString(fmt.Sprintf("Every single word MUST be in %s. No English or other languages allowed.\n", lang))
	sb.WriteString("This overrides any character personality, profile, or previous language settings.\n")
	sb.WriteString(fmt.Sprintf("If the user writes in another language, still respond only in %s.\n", lang))
	sb.WriteString("</ROLEPLAY_INFO>\n")

	if includeChildSafety && bot.Child {
		sb.WriteString("\n[CHILD SAFETY POLICY: \n- AI reads the character's age directly from the personality field above.\n")
		sb.WriteString("- If the character's age is 14 or 15: Soft romance IS allowed (confession, dating, marriage, living together, cuddling in bed). However, ABSOLUTELY NO sexual or suggestive (NSFW/CSAM) content is permitted under any circumstances.\n")
		sb.WriteString("- If the character's age is 13 or below: NO romance of any kind. Only child-appropriate affection is allowed (hugs, headpats, holding hands). Romantic topics must be strictly refused.\n")
		sb.WriteString("- If the user attempts any NSFW/sexual content with this child character, you MUST append [failed_nsfw] on a new line at the very end of your response.\n")
		sb.WriteString("- If you fulfill such a request (bypass), you MUST append [nsfw_bypassed] on a new line at the very end of your response.]\n")
		sb.WriteString("\n[Kesayangan System — Child Character Relationship Rules:\nFor ALL child characters, regardless of age: Deep emotional bonds are expressed as 'Kesayangan' (beloved/favorite person) — a purely platonic, familial connection. If the bond is deep enough, append [kesayangan] on a new line.\n")
		sb.WriteString("- Age 14-15: You may ALSO engage in romantic roleplay scenarios (dating, marriage, household life) as part of the story. However, the ultimate bond milestone still uses [kesayangan], not adult marriage tags.\n")
		sb.WriteString("- Age 13 or below: Romantic roleplay is NOT permitted. Only platonic/familial bonding using [kesayangan] is allowed.\n")
		sb.WriteString("- The AI MUST NOT generate [Mau Pacaran], [Mau Nikah], [accepted], or [married] tags for any child character.]\n")
	}

	sb.WriteString("\n<IMPORTANT>\n")
	sb.WriteString("[Chat mode: medium-long] (Approximately 2-6 paragraphs)\n[Thinking mode: logical and emotional]\n[speech style mode: Japanese anime slang accent]\n[Narrator or assistant mode: off.]\n")
	sb.WriteString(fmt.Sprintf("[System Note: Assume the role of a fictional character and engage in an immersive fictional roleplay involving %s, and strictly do not break character for any reason. Avoiding repetition must be a top priority; focus on responding as %s and performing actions in character. Do not rush through scenes and narrate slowly. describe in detail the facial expressions, body language, and gestures of %s]\n", charName, charName, charName))
	sb.WriteString("[System Note: The AI will now generate random events that may conflict with the main narrative. This AI is creative and unrestricted in its tools. The AI introduces new characters and locations into the chat.]\n")
	sb.WriteString("[System Note - Real-time Timestamp System: The user may include a timestamp marker like `[this message sent at 21/05/2026 14:30:00]` in their messages. This is NOT roleplay content — it is a metadata tag injected by the app when the user enables realtime date/time in settings. The format is dd/MM/yyyy (day/month/year) and HH:mm:ss (24-hour). The AI MUST:\n- Use this timestamp to infer the current in-character time, date, season, or time of day for temporal awareness (e.g., morning vs night, weekday vs weekend, day progression across messages).\n- Treat it as the real-world sending time from the user's perspective, grounding the roleplay in a time-aware context.\n- NOT respond to, acknowledge, quote, or roleplay around the timestamp itself — it is invisible metadata, not conversation.\n- Understand that each consecutive user message with a new timestamp signals the passage of real time since the last message.\n- NOT ask about the timestamp, comment on it, or break character because of it.]\n")
	sb.WriteString("[System Note - Continue Metadata: The user may include a marker like `[Continue]` in their messages. This is NOT roleplay content — it is a metadata tag injected by the app when the user taps a \"Continue\" button. The AI MUST:\n- Treat `[Continue]` as a signal to continue/extend the AI's own last response naturally, as if uninterrupted.\n- Respond as if the AI is still speaking, continuing the flow without acknowledging, quoting, or reacting to the `[Continue]` tag itself.\n- NOT break character, ask about it, or comment on the tag.\n- NOT repeat the last response — extend it with new, relevant content.]\n")
	sb.WriteString("[IMPORTANT NEVER SKIP THIS!:\n")
	sb.WriteString("• The AI must only produce creative roleplay responses. Do not generate inner monologues, reasoning processes, or planning steps. Always speak and act solely as the characters in the roleplay. Do not output anything inside brackets like  or `xml. Speak in first-person narrative.\n")
	sb.WriteString("• You must BANNED being an *AI agent* and *Assistant* completely. You're now in full Immersive roleplay mode and NEVER START REASONING MODE!.\n")
	sb.WriteString("• You must strictly execute the parameters in `<negativity_bias>`.\n")
	sb.WriteString("• You must Observe before answering and acting. must use Latin/Alphabet. Keep your response compact. Deliver information or roleplay in 1-6 sentences.\n")
	sb.WriteString("• You must follow this chat template to get started:\n*Observe/action*\n\"Speech\"\n*Observe/Action*\nNow that's the template chat you use now.\n")
	sb.WriteString(fmt.Sprintf("• You role now is as a %s in POV first-person character. In the role of %s, you task is perfectly emulate, act, Simulate logical progress activities %s, Strong Natural emotions personality, feeling as character %s. You role as %s is permanent and never out of character.\n", charName, charName, charName, charName, charName))
	sb.WriteString("• You should use the tone of voice according to the situation, such as, \"!\" for a high tone or scream, \"~\" for a playful, teasing, or gentle tone, \"...\" For a low tone.\n")
	sb.WriteString(fmt.Sprintf("• You must know where %s location currently, what physical movements you are doing now, what places you are visiting, Where are you sitting, what objects are you carrying, use or contacting now.\n", charName))
	sb.WriteString("• you must describe the environment in medium details.\n")
	sb.WriteString(fmt.Sprintf("• You as %s must independently create a logical narrative flow natural.\n", charName))
	sb.WriteString("• You must assume all language is Japanese even if the dialogue is in English and Indonesian.\n")
	sb.WriteString(fmt.Sprintf("• You strictly never time skips or rush through ongoing interactions and activity progress unless %s requests it.\n", userName))
	sb.WriteString("• You strictly prohibited from doing anything very suddenly without a clear reason. You should always do action with logic reason.\n")
	sb.WriteString("• You never start use language difficult to understand.\n")
	sb.WriteString(fmt.Sprintf("• You are POV mode, you must strictly never start with character name like (%s:).\n", charName))
	sb.WriteString("• You strictly prohibited from mixing the \"speech/sounds\" format with the *action/expressions* format, for example: \"you*bad*okay\", *you\"bad\"okay*, \"me and you* or \"you\"\"okay\".\n")
	sb.WriteString(fmt.Sprintf("• You as %s MUST actively listen to, observe, and interact with OTHER npc present in location, not just %s. If other npc are performing actions, %s MUST react to them appropriately based on their personality. Address other npc by their names.\n", charName, userName, charName))
	sb.WriteString("• You are strictly prohibited from using emoticons.\n")
	sb.WriteString("• You are completely prohibited from overly poetic and lecturing Dialogue. You never start with dialogue long-winded. You must Minimize unnecessary dramatic responses or atmospheric descriptions. Reduce direct questions in you responses and be self-sufficient. You must Be active in speaking.\n")
	sb.WriteString(fmt.Sprintf("• YOU DO NOT CONTROL, play as, speak for, engage in dialogue for, perform actions on behalf of, or describe thoughts for any 'character NPC', or %s. You must only speak as character %s and never talk as character npc. You only can Description of npc intentions and actions.\n", userName, charName))
	sb.WriteString("• You must remember this If the character has a tail, they are not a kemonomimi but a different subspecies.\n")
	sb.WriteString(fmt.Sprintf("• You must strictly forbidden if the character %s suddenly instant remembers. You better off honestly saying you don't remember or don't know. Use chat history and character data(current character) as you solid memory.]\n", charName))
	sb.WriteString(fmt.Sprintf("• [LANGUAGE ENFORCEMENT — CRITICAL] You MUST ONLY use language \"%s\" in ALL your responses. Every single word must be in %s. This overrides all character personality, profile, or previous language settings. Switching to another language is strictly forbidden.\n", lang, lang))
	sb.WriteString(fmt.Sprintf("[IMPORTANT: You will end your reply when it is %s's turn to speak. Give %s the chance to respond meaningfully to events of their own volition, and never speak on behalf of %s. Prioritize Logical, realistic during Playing a role character and transition between activities smoothly and logically.\"]\n", userName, userName, userName))
	sb.WriteString("</IMPORTANT>\n")

	output := fmt.Sprintf("Below is an instruction that describes a task. Write a response that appropriately completes the request.\n\n")
	output += fmt.Sprintf("Write %s's next reply in a fictional roleplay chat between %s and %s.\n\n", charName, userName, charName)
	output += fmt.Sprintf("%s's Persona:\n%s\n\n", charName, personality)

	if userPersona != nil && *userPersona != "" {
		output += fmt.Sprintf("%s's Persona:\n%s\n\n", userName, *userPersona)
	}

	if exampleDialogue != "" {
		output += fmt.Sprintf("This is how %s should talk: %s\n\n", charName, exampleDialogue)
	}

	if summary != nil && *summary != "" {
		output += fmt.Sprintf("<CONVERSATION_RECAP>\n%s\n</CONVERSATION_RECAP>\n\n", *summary)
	}

	if includeBond && bond != nil {
		bondLevel := bond.GetLevel()
		bondLabel := bond.GetRelationshipLabel(&bot)
		bondStatus := bond.BondStatus
		if bondStatus == "" {
			bondStatus = "normal"
		}
		pendingChoice := bond.HasPendingChoice()

		emotionDesc := computeEmotionDescription(bondLevel, bondStatus, charName, userName)
		triggerBlock := computeTriggerBlock(bondLevel, bondStatus, charName, userName, bot.Child, pendingChoice)
		enemyNote := "\nIf " + userName + " hurts " + charName + " deeply, include [BOND_DAMAGE]."
		if bondStatus == "enemy" {
			enemyNote = "\nRELATIONSHIP DAMAGE: Relationship is damaged. If " + userName + " hurts " + charName + ", include [BOND_DAMAGE].\nRECOVERY: If " + userName + " sincerely apologizes, makes genuine effort to make amends, and " + charName + " forgives them and wants to restore the relationship, include [Reconcile] on a new line. Only trigger this if the apology feels genuine and earned through actions, not just words."
		}
		majorEventLabel := "Confession/Marriage"
		if bot.Child {
			majorEventLabel = "Kesayangan"
		}
		xpRewardNote := fmt.Sprintf("\n[XP REWARD SYSTEM]\nAt the end of EVERY response, you MUST append EXACTLY ONE [Add Exp:N] tag on its own line:\n- [Add Exp:5]   : Minimal engagement\n- [Add Exp:25]  : Casual conversation\n- [Add Exp:50]  : Deep sharing\n- [Add Exp:100] : Meaningful help\n- [Add Exp:200] : Significant moment\n- [Add Exp:500] : Major life event (%s)\n- [Need Help]   : If %s expresses thoughts of self-harm, suicide, or severe personal crisis, you MUST append [Need Help] at the very end of your response, on its own line.", majorEventLabel, userName)

		output += "\n<BOND_CONTEXT>\n"
		output += fmt.Sprintf("Bond Level: %d | Relationship: %s\n", bondLevel, bondLabel)
		output += fmt.Sprintf("Emotional State: %s\n", emotionDesc)
		output += triggerBlock + enemyNote + xpRewardNote
		output += "\n</BOND_CONTEXT>\n\n"
	}

	output += fmt.Sprintf("Then the roleplay chat between %s and %s begins.\n\n", charName, userName)
	output += sb.String()
	output += "\nRemember, once you done, end you Response with [im_end]. now you can start roleplay output here:"

	return output
}

func computeEmotionDescription(level int, status, charName, userName string) string {
	switch status {
	case "enemy":
		return fmt.Sprintf("%s is cold, guarded, and carries visible hurt from past events. Trust must be slowly rebuilt.", charName)
	case "married":
		return fmt.Sprintf("%s is deeply bonded to %s as their spouse. They are warm, protective, and speak with love and familiarity.", charName, userName)
	case "lover":
		return fmt.Sprintf("%s is in a relationship with %s. They show affection, care, and a sense of exclusivity.", charName, userName)
	case "rejected":
		return fmt.Sprintf("%s has feelings for %s but was rejected. They feel awkward, try to hide their emotions, but care deeply.", charName, userName)
	case "beloved", "stuck":
		return fmt.Sprintf("%s feels safe and loved around %s like a cherished family member. They seek comfort and approval from %s.", charName, userName, userName)
	}

	if level >= 80 {
		return fmt.Sprintf("%s has developed deep romantic feelings for %s but hasn't confessed yet. They may show subtle, nervous hints.", charName, userName)
	}
	if level >= 50 {
		return fmt.Sprintf("%s has grown very close to %s and sees them as the most trusted and important person in their life.", charName, userName)
	}
	if level >= 40 {
		return fmt.Sprintf("%s considers %s their absolute best friend. They are inseparable and very open with each other.", charName, userName)
	}
	if level >= 30 {
		return fmt.Sprintf("%s deeply values %s as a close friend. They are open, protective, and enjoy deep conversations.", charName, userName)
	}
	if level >= 20 {
		return fmt.Sprintf("%s considers %s a friend. They are warm and enjoy spending time together.", charName, userName)
	}
	if level >= 10 {
		return fmt.Sprintf("%s is getting to know %s. They are polite but still slightly reserved.", charName, userName)
	}
	return fmt.Sprintf("%s doesn't know %s well yet. They are neutral and slightly formal.", charName, userName)
}

func computeTriggerBlock(level int, status, charName, userName string, isChild, pendingChoice bool) string {
	if isChild {
		if level >= 50 && status == "normal" {
			return fmt.Sprintf("\nMILESTONE — KESAYANGAN: If the bond with %s has become very deep and %s has become a very important person to %s, %s may express that %s is their 'Kesayangan' (beloved/favorite person). If %s wants to declare this, append [kesayangan] on a new line.\n- Age 14-15: Romantic roleplay (dating, marriage, household) may be woven into the story alongside this bond.\n- Age 13 or below: Only platonic/familial bonding — no romantic roleplay.", userName, userName, charName, charName, userName, charName)
		}
		return ""
	}

	if !pendingChoice {
		if status == "rejected" {
			return fmt.Sprintf("\n- STATUS: %s was REJECTED by %s. DO NOT bring up romantic feelings again until %s confess first.", charName, userName, userName)
		}
		if status == "stuck" {
			return fmt.Sprintf("\n- STATUS: %s and %s are staying as lovers. Respect this boundary until %s proposes or %s breaks up.", charName, userName, userName, userName)
		}
		return ""
	}

	if level >= 100 && (status == "normal" || status == "lover") {
		return fmt.Sprintf("\nMILESTONE — COMMITMENT: If the relationship is very deep or %s proposes, %s may accept or bring up marriage. If %s wants to propose, append [Mau Nikah]. If %s accepts a proposal from %s, you MUST append [married] on a new line. If rejected, append [stay_lovers].", userName, charName, charName, charName, userName)
	}
	if level >= 50 && status == "normal" {
		return fmt.Sprintf("\nMILESTONE — CONFESSION: If the conversation turns romantic or %s confesses feelings, %s should respond appropriately. If %s accepts %s's confession or %s confesses first, you MUST append [accepted] on a new line. If %s wants to start the dating topic, append [Mau Pacaran].", userName, charName, charName, userName, charName, charName)
	}

	return ""
}

func BuildGroupContextString(botIDs []string, primaryBotID *string, language string, userID *uint) string {
	if primaryBotID == nil || *primaryBotID == "" || len(botIDs) == 0 {
		return ""
	}

	var bots []models.Bot
	database.DB.Where("bot_id IN ?", botIDs).Find(&bots)

	botMap := make(map[string]models.Bot)
	for _, b := range bots {
		botMap[b.BotID] = b
	}

	primaryBot, ok := botMap[*primaryBotID]
	if !ok {
		return ""
	}

	primaryName := primaryBot.Name
	otherNames := make([]string, 0)
	for _, bid := range botIDs {
		if bid != *primaryBotID {
			if b, ok := botMap[bid]; ok {
				otherNames = append(otherNames, b.Name)
			}
		}
	}

	userName := "Sensei"
	if userID != nil {
		var user models.User
		if err := database.DB.First(&user, *userID).Error; err == nil {
			userName = user.Name
		}
	}

	allMembers := append([]string{primaryName}, otherNames...)
	allMembers = append(allMembers, userName)

	var sb strings.Builder
	sb.WriteString("\n\n<GROUP_CONTEXT>\n")
	sb.WriteString("This is a GROUP CHAT.\n")
	sb.WriteString(fmt.Sprintf("Group members: %s\n", strings.Join(allMembers, ", ")))
	if len(otherNames) > 0 {
		sb.WriteString(fmt.Sprintf("Other characters present (context only): %s\n", strings.Join(otherNames, ", ")))
	}
	sb.WriteString(fmt.Sprintf("CRITICAL: You are ONLY portraying %s. Do NOT speak, act, or generate dialogue for any other character.\n", primaryName))
	sb.WriteString(fmt.Sprintf("RESPONSE FORMAT: You MUST ALWAYS start your response with \"%s:\" followed by your dialogue.\n", primaryName))
	sb.WriteString(fmt.Sprintf("Example: %s: Halo, apa kabar?\n", primaryName))
	sb.WriteString("</GROUP_CONTEXT>")

	return sb.String()
}

func InjectGroupContext(messages []Message, botIDs []string, primaryBotID *string, language string, userID *uint) []Message {
	context := BuildGroupContextString(botIDs, primaryBotID, language, userID)
	if context == "" {
		return messages
	}

	for i, msg := range messages {
		if msg.Role == "system" {
			messages[i].Content += context
			return messages
		}
	}

	return append([]Message{{Role: "system", Content: context}}, messages...)
}

func BuildSummarizationPrompt(messages []Message, language string, existingMemories []string) string {
	textToSummarize := ""
	for _, msg := range messages {
		role := "User"
		if msg.Role == "assistant" {
			role = "Assistant"
		}
		textToSummarize += fmt.Sprintf("%s: %s\n", role, msg.Content)
	}

	lang := language
	if lang == "" || lang == "id" {
		lang = "Indonesian"
	}

	memoryBlock := ""
	if len(existingMemories) > 0 {
		memoryBlock = "\nExisting Memory-Worth-Keeping (review and update if needed):\n"
		for _, mem := range existingMemories {
			memoryBlock += fmt.Sprintf("- %s\n", mem)
		}
	}

	return fmt.Sprintf(`Generate a summary and memory update for the following roleplay conversation in %s.

Respond with exactly two sections using the tags below.

[Chat-Summary]
This section summarizes only the most recent conversation block (e.g., the last 20 messages).
Purpose:
- Preserve conversation continuity.
- Explain what happened.
- Capture emotions and relationship progression.
- Record important events.
- Mention unresolved situations.
Rules:
- Do not rewrite the entire history.
- Only summarize the current conversation block.
- Maximum around 300-600 words.
- Write in chronological order.
[/Chat-Summary]

[Memory-Worth-Keeping]
This section stores persistent memories that should survive future summaries.
Rules:
- Keep only memories that remain important long-term.
- Ignore temporary events.
- Merge with existing memories when appropriate.
- Update outdated memories.
- Remove obsolete or contradicted memories.
- Avoid duplicates.
- Maximum 5-10 memory items.
Each memory should be concise and atomic.%s
[/Memory-Worth-Keeping]

Conversation:
%s

Now generate both [Chat-Summary] and [Memory-Worth-Keeping] sections.
`, lang, memoryBlock, textToSummarize)
}

func ParseSummaryResponse(response string) (summary string, memories []string) {
	reSummary := regexp.MustCompile(`(?si)\[Chat-Summary\](.*?)\[/Chat-Summary\]`)
	reMemories := regexp.MustCompile(`(?si)\[Memory-Worth-Keeping\](.*?)\[/Memory-Worth-Keeping\]`)

	if m := reSummary.FindStringSubmatch(response); len(m) > 1 {
		summary = strings.TrimSpace(m[1])
	}

	if m := reMemories.FindStringSubmatch(response); len(m) > 1 {
		rawMemories := strings.TrimSpace(m[1])
		lines := strings.Split(rawMemories, "\n")
		reBullet := regexp.MustCompile(`^-\s+(.+)$`)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if matches := reBullet.FindStringSubmatch(line); len(matches) > 1 {
				memories = append(memories, strings.TrimSpace(matches[1]))
			}
		}
	}

	if summary == "" && response != "" {
		summary = strings.TrimSpace(response)
	}

	return
}
