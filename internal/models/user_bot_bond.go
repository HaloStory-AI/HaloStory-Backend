package models

import (
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

type UserBotBond struct {
	ID            uint   `gorm:"primaryKey;autoIncrement"`
	UserID        uint   `gorm:"not null;index"`
	BotID         string `gorm:"type:varchar(255);not null;index"`
	Xp            int64  `gorm:"default:0"`
	BondStatus    *string `gorm:"type:varchar(255);default:'normal'"`
	ChoiceAtLevel *int   `gorm:"type:int"`
	CreatedAt     time.Time
	UpdatedAt     time.Time

	Bot *Bot `gorm:"foreignKey:BotID;references:BotID"`
}

const (
	MilestoneConfession  = 50
	MilestoneCommitment  = 100
	BondNormal           = "normal"
	BondLover            = "lover"
	BondMarried          = "married"
	BondRejected         = "rejected"
	BondStuck            = "stuck"
	BondEnemy            = "enemy"
	BondBeloved          = "beloved"
)

func (b *UserBotBond) RequiredXPForLevel(level int) int {
	if level <= 1 {
		return 0
	}
	x := float64(level)
	xp := 0.2501333333*math.Pow(x, 3) - 1.14*math.Pow(x, 2) + 15.16666666*x
	return max(0, int(math.Round(xp)))
}

func (b *UserBotBond) Level() int {
	if b.Xp <= 0 {
		return 1
	}
	guess := int(math.Pow(float64(4*b.Xp), 1.0/3.0))
	for b.RequiredXPForLevel(guess+1) <= int(b.Xp) {
		guess++
	}
	for b.RequiredXPForLevel(guess) > int(b.Xp) && guess > 1 {
		guess--
	}
	return guess
}

func (b *UserBotBond) GetStatus() string {
	if b.BondStatus == nil {
		return BondNormal
	}
	return *b.BondStatus
}

func (b *UserBotBond) IsStuck() bool {
	status := b.GetStatus()
	return status == BondRejected || status == BondStuck
}

func (b *UserBotBond) CanGainXP() bool {
	return !b.IsStuck()
}

func (b *UserBotBond) HasPendingChoice() bool {
	level := b.Level()
	status := b.GetStatus()
	if level >= 100 && (status == BondNormal || status == BondLover) {
		return true
	}
	if level >= 50 && status == BondNormal {
		return true
	}
	return false
}

func (b *UserBotBond) CanReconcile() bool {
	return b.GetStatus() == BondEnemy && b.Level() >= 10
}

func (b *UserBotBond) ApplyChoice(choice string) {
	level := b.Level()
	switch choice {
	case "accept_love":
		s := BondLover
		b.BondStatus = &s
		b.ChoiceAtLevel = &level
	case "reject_love":
		s := BondRejected
		b.BondStatus = &s
		b.ChoiceAtLevel = &level
	case "marry":
		s := BondMarried
		b.BondStatus = &s
		b.ChoiceAtLevel = &level
	case "stay_lovers":
		s := BondStuck
		b.BondStatus = &s
		b.ChoiceAtLevel = &level
	case "beloved":
		s := BondBeloved
		b.BondStatus = &s
		b.ChoiceAtLevel = &level
	}
}

func (b *UserBotBond) ApplyReconcile() {
	if b.GetStatus() != BondEnemy {
		return
	}
	s := BondNormal
	b.BondStatus = &s
	level := b.Level()
	b.ChoiceAtLevel = &level
}

func (b *UserBotBond) RelationshipLabel() string {
	if b.Bot == nil {
		return "Orang Asing"
	}

	level := b.Level()
	status := b.GetStatus()
	isChild := b.Bot.Child

	if isChild {
		switch status {
		case BondEnemy:
			return "Musuh"
		case BondBeloved, BondLover, BondMarried:
			return "Kesayangan"
		case BondRejected:
			return "Menolak"
		default:
			if level >= 50 {
				return "Kesayangan"
			}
		}
	} else {
		switch status {
		case BondEnemy:
			return "Musuh"
		case BondMarried:
			return "Suami/Istri"
		case BondLover:
			return "Pacar"
		case BondRejected:
			return "Menolak Perasaan"
		default:
			if level >= 50 {
				return "Naksir"
			}
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

	context := ""
	if b.Bot.Description != nil {
		context += strings.ToLower(*b.Bot.Description)
	}
	if b.Bot.Personality != nil {
		context += " " + strings.ToLower(*b.Bot.Personality)
	}

	switch {
	case strings.Contains(context, "sensei") || strings.Contains(context, "guru") || strings.Contains(context, "teacher"):
		return "Murid & Guru"
	case strings.Contains(context, "trainer") || strings.Contains(context, "coach") || strings.Contains(context, "pelatih"):
		return "Atlet & Pelatih"
	case strings.Contains(context, "doctor") || strings.Contains(context, "dokter") || strings.Contains(context, "nurse"):
		return "Pasien & Medis"
	case strings.Contains(context, "manager") || strings.Contains(context, "boss") || strings.Contains(context, "atasan"):
		return "Staf & Atasan"
	case strings.Contains(context, "maid") || strings.Contains(context, "pelayan") || strings.Contains(context, "butler"):
		return "Tuan & Pelayan"
	}

	return "Orang Asing"
}

func (b *UserBotBond) BondEmotion() string {
	status := b.GetStatus()
	level := b.Level()
	isChild := b.Bot != nil && b.Bot.Child

	if isChild {
		switch status {
		case BondEnemy:
			return "hostile"
		case BondBeloved:
			return "cherished"
		case BondRejected:
			return "awkward"
		default:
			switch {
			case level >= 50:
				return "close"
			case level >= 40:
				return "soulmates_friendship"
			case level >= 30:
				return "best_friend"
			case level >= 20:
				return "close_friend"
			case level >= 10:
				return "friendly"
			case level >= 5:
				return "acquaintance"
			default:
				return "stranger"
			}
		}
	}

	switch status {
	case BondEnemy:
		return "hostile"
	case BondMarried:
		return "deeply_intimate"
	case BondLover:
		return "intimate"
	case BondRejected:
		return "awkward_longing"
	default:
		switch {
		case level >= 50:
			return "in_love"
		case level >= 40:
			return "soulmates_friendship"
		case level >= 30:
			return "best_friend"
		case level >= 20:
			return "close_friend"
		case level >= 10:
			return "friendly"
		case level >= 5:
			return "acquaintance"
		default:
			return "stranger"
		}
	}
}

func (b *UserBotBond) BeforeUpdate(tx *gorm.DB) error {
	if tx.Statement.Changed("BondStatus") {
		var oldStatus string
		if b.BondStatus != nil {
			oldStatus = *b.BondStatus
		} else {
			oldStatus = BondNormal
		}
		_ = oldStatus
	}
	return nil
}
