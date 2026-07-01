package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type Category struct {
	ID        uint    `gorm:"primaryKey;autoIncrement"`
	Name      string  `gorm:"type:varchar(255);not null"`
	Slug      string  `gorm:"type:varchar(255);uniqueIndex;not null"`
	LogoURL   *string `gorm:"type:varchar(255)"`
	Pinned    bool    `gorm:"type:tinyint(1);default:0"`
	Featured  bool    `gorm:"type:tinyint(1);default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time

	Bots []Bot `gorm:"foreignKey:CategorySlug;references:Slug" json:"-"`
}

func (c *Category) BeforeCreate(tx *gorm.DB) error {
	if c.Slug == "" {
		c.Slug = strings.ToLower(strings.ReplaceAll(c.Name, " ", "-"))
	}
	return nil
}

func (c *Category) BeforeSave(tx *gorm.DB) error {
	if c.Pinned && c.Featured {
		return ErrPinnedAndFeatured
	}
	return nil
}

var ErrPinnedAndFeatured = &CategoryError{"pinned and featured cannot both be true"}

type CategoryError struct {
	msg string
}

func (e *CategoryError) Error() string {
	return e.msg
}
