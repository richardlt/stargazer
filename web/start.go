package web

import (
	"github.com/sirupsen/logrus"

	"github.com/richardlt/stargazer/config"
	"github.com/richardlt/stargazer/database"
)

func Start(cfg config.Web) error {
	logrus.SetLevel(cfg.LogLevel)

	db, err := database.New(cfg.Database)
	if err != nil {
		return err
	}

	s := &Server{
		db:              db,
		regenerateDelay: cfg.RegenerateDelay,
		mainRepository:  cfg.MainRepository,
		maxEntriesCount: cfg.MaxEntriesCount,
	}
	if err := s.initRouter(); err != nil {
		return err
	}
	defer s.Close()

	return s.Start(cfg.Port)
}
