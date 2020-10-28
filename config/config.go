package config

import "github.com/sirupsen/logrus"

type Common struct {
	LogLevel                             logrus.Level
	DatabaseURL                          string
	MainRepository                       string
	TaskRepositoryOrgContributorsToCheck int64
}

type Crawler struct {
	Common
	GHToken                         string
	MgoURI                          string
	UserExpirationDelay             int64
	MainRepositoryScanDelay         int64
	TaskRepositoryScanDelay         int64
	TaskRepositoryMaxStargazerPages int64
	TaskRepositoryExclusions        []string
}

type Web struct {
	Common
	Port            int64
	RegenerateDelay int64
	MaxEntriesCount int64
}
