package models

import (
	"math"
	"strings"
	"time"
)

type UserBotBond struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"index" json:"user_id"`
	BotID         string    `gorm:"index" json:"bot_id"`
	Xp            int       `json:"xp"`
	BondStatus    string    `json:"bond_status"`
	ChoiceAtLevel *int      `json:"choice_at_level"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	Bot *Bot `gorm:"foreignKey:BotID;references:BotID" json:"bot,omitempty"`
}

func (UserBotBond) TableName() string {
	return "user_bot_bonds"
}

// GetRequiredXpForLevel menghitung batas XP yang dibutuhkan untuk mencapai level tertentu
// Menggunakan formula Cubic Curve: (0.250133... * x^3) - (1.14 * x^2) + (15.166... * x)
func (b *UserBotBond) GetRequiredXpForLevel(level int) int {
	if level <= 1 {
		return 0
	}

	x := float64(level)
	xp := (0.2501333333 * math.Pow(x, 3)) - (1.14 * math.Pow(x, 2)) + (15.16666666 * x)

	if xp < 0 {
		return 0
	}
	return int(math.Round(xp))
}

// GetLevel menghitung level saat ini berdasarkan total XP
func (b *UserBotBond) GetLevel() int {
	if b.Xp <= 0 {
		return 1
	}

	// O(1) initial guess via inverse cubic: level ≈ cbrt(4 * XP)
	guess := int(math.Floor(math.Pow(4.0*float64(b.Xp), 1.0/3.0)))

	for b.GetRequiredXpForLevel(guess+1) <= b.Xp {
		guess++
	}
	for b.GetRequiredXpForLevel(guess) > b.Xp && guess > 1 {
		guess--
	}

	return guess
}

func (b *UserBotBond) GetRelationshipLabel(bot *Bot) string {
	level := b.GetLevel()
	status := b.BondStatus
	if status == "" {
		status = "normal"
	}

	isChild := bot != nil && bot.Child

	if isChild {
		if status == "enemy" {
			return "Musuh"
		}
		if status == "beloved" || status == "lover" || status == "married" {
			return "Kesayangan"
		}
		if status == "rejected" {
			return "Menolak"
		}
		if level >= 50 {
			return "Kesayangan"
		}
	} else {
		if status == "enemy" {
			return "Musuh"
		}
		if status == "married" {
			return "Suami/Istri"
		}
		if status == "lover" {
			return "Pacar"
		}
		if status == "rejected" {
			return "Menolak Perasaan"
		}
		if level >= 50 {
			return "Naksir"
		}
	}

	if level >= 40 {
		return "Sahabat Terbaik"
	}
	if level >= 30 {
		return "Sahabat"
	}
	if level >= 20 {
		return "Teman Dekat"
	}
	if level >= 10 {
		return "Teman"
	}
	if level >= 5 {
		return "Mulai Kenal"
	}

	if bot != nil {
		context := strings.ToLower(func() string {
			desc := ""
			pers := ""
			if bot.Description != nil {
				desc = *bot.Description
			}
			if bot.Personality != nil {
				pers = *bot.Personality
			}
			return desc + " " + pers
		}())

		if strings.Contains(context, "sensei") || strings.Contains(context, "guru") || strings.Contains(context, "teacher") {
			return "Murid & Guru"
		}
		if strings.Contains(context, "trainer") || strings.Contains(context, "coach") || strings.Contains(context, "pelatih") {
			return "Atlet & Pelatih"
		}
		if strings.Contains(context, "doctor") || strings.Contains(context, "dokter") || strings.Contains(context, "nurse") {
			return "Pasien & Medis"
		}
		if strings.Contains(context, "manager") || strings.Contains(context, "boss") || strings.Contains(context, "atasan") {
			return "Staf & Atasan"
		}
		if strings.Contains(context, "maid") || strings.Contains(context, "pelayan") || strings.Contains(context, "butler") {
			return "Tuan & Pelayan"
		}
	}

	return "Orang Asing"
}

func (b *UserBotBond) HasPendingChoice() bool {
	level := b.GetLevel()
	status := b.BondStatus
	if status == "" {
		status = "normal"
	}

	if level >= 100 && (status == "normal" || status == "lover") {
		return true
	}
	if level >= 50 && status == "normal" {
		return true
	}
	return false
}

func (b *UserBotBond) ApplyChoice(choice string) {
	level := b.GetLevel()
	
	switch choice {
	case "accept_love":
		b.BondStatus = "lover"
		b.ChoiceAtLevel = &level
	case "reject_love":
		b.BondStatus = "rejected"
		b.ChoiceAtLevel = &level
	case "marry":
		b.BondStatus = "married"
		b.ChoiceAtLevel = &level
	case "stay_lovers":
		b.BondStatus = "stuck"
		b.ChoiceAtLevel = &level
	case "beloved":
		b.BondStatus = "beloved"
		b.ChoiceAtLevel = &level
	}
}
