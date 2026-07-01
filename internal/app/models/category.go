package models

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Category struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Slug      string    `gorm:"uniqueIndex" json:"slug"`
	LogoUrl   *string   `json:"logo_url"`
	Pinned    bool      `json:"pinned"`
	Featured  bool      `json:"featured"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Bots []Bot `gorm:"foreignKey:CategorySlug;references:Slug" json:"-"`
}

func (Category) TableName() string {
	return "categories"
}

// BeforeSave hook untuk validasi dan slug
func (c *Category) BeforeSave(tx *gorm.DB) (err error) {
	if c.Slug == "" {
		// generate slug sederhana
		lower := strings.ToLower(c.Name)
		c.Slug = strings.ReplaceAll(lower, " ", "-")
	}

	if c.Pinned && c.Featured {
		return errors.New("Pinned dan Featured tidak bisa aktif bersamaan.")
	}

	return nil
}
