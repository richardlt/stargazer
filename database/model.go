package database

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
)

type Status string

const (
	StatusRequested Status = "requested"
	StatusGenerated Status = "generated"
)

type Entry struct {
	ID              uint      `gorm:"column:id;primary_key"`
	CreatedAt       time.Time `gorm:"column:created_at;DEFAULT:CURRENT_TIMESTAMP"`
	UpdatedAt       time.Time `gorm:"column:updated_at;DEFAULT:CURRENT_TIMESTAMP"`
	Repository      string    `gorm:"column:repository;type:varchar(255);unique_index"`
	LastGeneratedAt time.Time `gorm:"column:last_generated_at;DEFAULT:CURRENT_TIMESTAMP"`
	LastRequestedAt time.Time `gorm:"column:last_requested_at;DEFAULT:CURRENT_TIMESTAMP"`
	Status          Status    `gorm:"column:status"`
	Stats           Stats     `gorm:"column:stats;type:JSONB"`
}

type Stats struct {
	Evolution  []Measure   `json:"evolution,omitempty"`
	PerDays    []Measure   `json:"per_days,omitempty"`
	Last10     []Stargazer `json:"last_10,omitempty"`
	CountStars int64       `json:"count_stars,omitempty"`
}

func (s *Stats) Scan(src interface{}) error {
	source, ok := src.([]byte)
	if !ok {
		return errors.New("type assertion .([]byte) failed")
	}
	return errors.Wrap(json.Unmarshal(source, s), "cannot unmarshal Stats")
}

func (s Stats) Value() (driver.Value, error) {
	j, err := json.Marshal(s)
	return j, errors.Wrap(err, "cannot marshal Stats")
}

type Measure struct {
	Date  time.Time `json:"date"`
	Count int64     `json:"count"`
}

type Stargazer struct {
	Name string `json:"by"`
}
