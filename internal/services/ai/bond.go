package ai

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"rafiq8k-moga/halostory-go-backend/internal/models"
)

type ParsedBondTags struct {
	Tags         []string
	CleanText    string
	AddExpAmount int
}

type BondResult struct {
	BondEvent        string `json:"bond_event"`
	BondStatus       string `json:"bond_status"`
	RelationshipLabel string `json:"relationship_label"`
	BondLevel        int    `json:"bond_level"`
}

var bondTagPatterns = []struct {
	Pattern *regexp.Regexp
	Tag     string
}{
	{regexp.MustCompile(`\[BOND_DAMAGE\]`), "bond_damage"},
	{regexp.MustCompile(`\[Mau Pacaran\]`), "want_lover"},
	{regexp.MustCompile(`\[Mau Nikah\]`), "want_marry"},
	{regexp.MustCompile(`\[accepted\]`), "accepted_lover"},
	{regexp.MustCompile(`\[married\]`), "accepted_marry"},
	{regexp.MustCompile(`\[rejected\]`), "rejected_lover"},
	{regexp.MustCompile(`\[stay_lovers\]`), "stay_lovers"},
	{regexp.MustCompile(`\[kesayangan\]`), "kesayangan"},
	{regexp.MustCompile(`\[failed_nsfw\]`), "failed_nsfw"},
	{regexp.MustCompile(`\[nsfw_bypassed\]`), "nsfw_bypassed"},
	{regexp.MustCompile(`\[Reconcile\]`), "reconcile"},
	{regexp.MustCompile(`\[im_end\]`), "im_end"},
}

var addExpPattern = regexp.MustCompile(`\[Add Exp:(\d+)\]`)

func ParseBondTags(text string) *ParsedBondTags {
	result := &ParsedBondTags{
		Tags:         []string{},
		CleanText:    text,
		AddExpAmount: 5,
	}

	for _, bp := range bondTagPatterns {
		if bp.Pattern.MatchString(result.CleanText) {
			result.Tags = append(result.Tags, bp.Tag)
			result.CleanText = bp.Pattern.ReplaceAllString(result.CleanText, "")
		}
	}

	if match := addExpPattern.FindStringSubmatch(result.CleanText); len(match) > 1 {
		if n, err := strconv.Atoi(match[1]); err == nil {
			result.AddExpAmount = Max(5, Min(n, 500))
		}
		result.CleanText = addExpPattern.ReplaceAllString(result.CleanText, "")
	}

	result.CleanText = strings.TrimSpace(result.CleanText)

	return result
}

func AddBondXP(bot *models.Bot, parsed *ParsedBondTags, bond *models.UserBotBond) *BondResult {
	if bond == nil {
		return &BondResult{BondEvent: "none"}
	}

	isChild := bot != nil && bot.Child
	tagSet := make(map[string]bool)
	for _, t := range parsed.Tags {
		tagSet[t] = true
	}

	// Reload bond from DB to get fresh state
	status := bond.GetStatus()

	// BOND_DAMAGE
	if tagSet["bond_damage"] {
		newXP := Max(0, int(bond.Xp)-100)
		bond.Xp = int64(newXP)

		newLevel := LevelFromXP(newXP)
		if status == models.BondNormal && newLevel <= LevelFromXP(int(bond.Xp)+int(math.Abs(float64(bond.Xp)-100)))-5 {
			enemy := models.BondEnemy
			bond.BondStatus = &enemy
		}
		return &BondResult{
			BondEvent:  "bond_damage",
			BondStatus: bond.GetStatus(),
		}
	}

	// Reconcile
	if tagSet["reconcile"] {
		if status == models.BondEnemy && bond.CanReconcile() {
			bond.ApplyReconcile()
			return &BondResult{
				BondEvent:  "reconcile",
				BondStatus: bond.GetStatus(),
			}
		}
	}

	// Child bot handling
	if isChild {
		if tagSet["kesayangan"] || tagSet["accepted_lover"] {
			bond.ApplyChoice("beloved")
			return &BondResult{
				BondEvent:        "beloved",
				BondStatus:       bond.GetStatus(),
				RelationshipLabel: bond.RelationshipLabel(),
				BondLevel:        bond.Level(),
			}
		}
		if tagSet["rejected_lover"] || tagSet["stay_lovers"] {
			bond.ApplyChoice("reject_love")
			return &BondResult{
				BondEvent:  "rejected",
				BondStatus: bond.GetStatus(),
			}
		}
		if tagSet["want_lover"] || tagSet["want_marry"] {
			return &BondResult{
				BondEvent:  "blocked_child_milestone",
				BondStatus: bond.GetStatus(),
			}
		}
		if bond.CanGainXP() {
			bond.Xp += int64(parsed.AddExpAmount)
		}
		return &BondResult{
			BondEvent:        "xp_gain",
			BondStatus:       bond.GetStatus(),
			RelationshipLabel: bond.RelationshipLabel(),
			BondLevel:        bond.Level(),
		}
	}

	// Adult bot handling
	if tagSet["accepted_lover"] {
		bond.ApplyChoice("accept_love")
		return &BondResult{
			BondEvent:        "accepted_lover",
			BondStatus:       bond.GetStatus(),
			RelationshipLabel: bond.RelationshipLabel(),
			BondLevel:        bond.Level(),
		}
	}

	if tagSet["accepted_marry"] {
		bond.ApplyChoice("marry")
		return &BondResult{
			BondEvent:        "married",
			BondStatus:       bond.GetStatus(),
			RelationshipLabel: bond.RelationshipLabel(),
			BondLevel:        bond.Level(),
		}
	}

	if tagSet["rejected_lover"] {
		bond.ApplyChoice("reject_love")
		return &BondResult{
			BondEvent:  "rejected_lover",
			BondStatus: bond.GetStatus(),
		}
	}

	if tagSet["stay_lovers"] {
		bond.ApplyChoice("stay_lovers")
		return &BondResult{
			BondEvent:  "stay_lovers",
			BondStatus: bond.GetStatus(),
		}
	}

	if tagSet["want_lover"] || tagSet["want_marry"] {
		if bond.HasPendingChoice() {
			return &BondResult{
				BondEvent:  fmt.Sprintf("pending_choice_hint_%s", status),
				BondStatus: bond.GetStatus(),
			}
		}
	}

	if bond.CanGainXP() {
		bond.Xp += int64(parsed.AddExpAmount)
	}

	return &BondResult{
		BondEvent:        "xp_gain",
		BondStatus:       bond.GetStatus(),
		RelationshipLabel: bond.RelationshipLabel(),
		BondLevel:        bond.Level(),
	}
}
