package config

import "github.com/sirupsen/logrus"

type Common struct {
	LogLevel logrus.Level
}

type Crawler struct {
	Common
	GHToken                         string
	MgoURI                          string
	UserExpirationDelay             int64
	MainRepository                  string
	MainRepositoryScanDelay         int64
	TaskRepositoryScanDelay         int64
	TaskRepositoryMaxStargazerPages int64
	TaskRepositoryExclusions        []string
	DatabaseURL                     string
}

type Web struct {
	Common
	Port            int64
	DatabaseURL     string
	RegenerateDelay int64
	MainRepository  string
	MaxEntriesCount int64
}
