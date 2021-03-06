package crawler

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/richardlt/stargazer/config"
	"github.com/richardlt/stargazer/crawler/github"
	"github.com/richardlt/stargazer/database"
)

func Start(cfg config.Crawler) error {
	logrus.SetLevel(cfg.LogLevel)

	// init database
	client, err := mongo.NewClient(options.Client().ApplyURI(cfg.MgoURI))
	if err != nil {
		return errors.WithStack(err)
	}
	if err := client.Connect(context.Background()); err != nil {
		return errors.WithStack(err)
	}

	mgoClient := NewMongoClient(client.Database("stargazer"))
	if err := mgoClient.Init(); err != nil {
		return err
	}

	pgClient, err := database.New(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pgClient.Close()

	ghClient := github.NewClient(cfg.GHToken)

	go func() {
		logrus.Info("main: start main repository scanner")
		for {
			if err := execMainRepositoryRoutine(mgoClient, ghClient, cfg.MainRepository, cfg.UserExpirationDelay); err != nil {
				logrus.Errorf("%+v", err)
			}
			logrus.Infof("main: main repository scanner routine waiting %ds\n", cfg.MainRepositoryScanDelay)
			time.Sleep(time.Duration(cfg.MainRepositoryScanDelay) * time.Second)
		}
	}()

	go func() {
		logrus.Info("main: start task repository scanner")
		for {
			if err := execTaskRepositoriesRoutine(pgClient, mgoClient, ghClient, cfg); err != nil {
				logrus.Errorf("%+v", err)
			}
			logrus.Infof("main: task repository scanner routine waiting %ds\n", cfg.TaskRepositoryScanDelay)
			time.Sleep(time.Duration(cfg.TaskRepositoryScanDelay) * time.Second)
		}
	}()

	startDate := time.Now()
	lastDate := startDate
	for {
		now := time.Now()
		logrus.Infof("main: now is %s running since %s", now.UTC().String(), now.Sub(startDate).String())
		logrus.Infof("main: GH request count is %d for current hour", ghClient.GetRequestCount())

		// reset GH request count if hour changed
		if now.Hour() != lastDate.Hour() {
			ghClient.ResetRequestCount()
		}
		lastDate = now
		time.Sleep(time.Minute)
	}
}
