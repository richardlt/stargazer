package crawler

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/richardlt/stargazer/crawler/github"
)

func execMainRepositoryRoutine(dbClient *DatabaseClient, ghClient github.Client, repo string, userExpirationDelay int64) error {
	logrus.Infof("execMainRepositoryRoutine: get main repository %s from Github", repo)

	ghRepo, err := ghClient.GetRepository(repo)
	if err != nil {
		return err
	}

	githubStargazersCount := ghRepo.StargazersCount

	logrus.Infof("execMainRepositoryRoutine: get repository %s from database", repo)
	r, err := dbClient.getRepository(repo)
	if err != nil {
		return err
	}

	repoExists := r != nil

	if !repoExists {
		r = &repository{
			Path: repo,
			Data: ghRepo,
		}

		logrus.Infof("execMainRepositoryRoutine: create repository %s in database", repo)
		if err := dbClient.insertRepository(r); err != nil {
			return err
		}
	}

	databaseStargazersCount, err := dbClient.countStargazers(r.ID)
	if err != nil {
		return err
	}

	logrus.Infof("execMainRepositoryRoutine: found %d stargazers from GH for repo %s and %d in database", githubStargazersCount, repo, databaseStargazersCount)
	change := !repoExists || githubStargazersCount != databaseStargazersCount

	// if counts are different then reload all stargazers
	if change {
		if repoExists {
			logrus.Infof("execMainRepositoryRoutine: update repository %s in database", r.Path)
			r.Data = ghRepo
			if err := dbClient.updateRepository(r); err != nil {
				return err
			}
		}

		logrus.Info("execMainRepositoryRoutine: load stargazers from Github")
		os, err := ghClient.GetRepositoryStargazer(r.Path)
		if err != nil {
			return err
		}

		logrus.Infof("execMainRepositoryRoutine: delete all stargazers for repository %s in database", r.Path)
		if err := dbClient.deleteStargazers(r.ID); err != nil {
			return err
		}

		ss := make([]stargazer, len(os))
		for i := range os {
			ss[i].RepositoryID = r.ID
			ss[i].RepositoryPath = r.Path
			ss[i].Data = os[i]
		}

		logrus.Infof("execMainRepositoryRoutine: insert %d stargazers for repository %s in database", len(ss), r.Path)
		if err := dbClient.insertStargazers(ss); err != nil {
			return err
		}

		ss, err = dbClient.getStargazers(repo)
		if err != nil {
			return err
		}

		// Refresh data for all user that starred the main repository
		logrus.Infof("execMainRepositoryRoutine: iterate over %d stargazers", len(ss))
		for i := range ss {
			login := ss[i].Data.User.Login

			u, err := dbClient.getUser(login)
			if err != nil {
				return err
			}
			needSave := u == nil || (u.Expire.Before(time.Now()) && userExpirationDelay > 0)
			if needSave {
				logrus.Infof("execMainRepositoryRoutine: get user %s from Github (%d/%d)", login, i+1, len(ss))
				o, err := ghClient.GetUser(login)
				if err != nil {
					return err
				}
				os, err := ghClient.GetUserOrganizations(login)
				if err != nil {
					return err
				}

				expire := time.Now().Add(time.Second * time.Duration(userExpirationDelay))
				if u == nil {
					logrus.Infof("execMainRepositoryRoutine: insert user %s in database", login)
					if err := dbClient.insertUser(&user{
						Expire:        expire,
						Login:         login,
						Data:          o,
						Organizations: os,
					}); err != nil {
						return err
					}
				} else {
					u.Expire = expire
					u.Data = o
					u.Organizations = os
					logrus.Infof("execMainRepositoryRoutine: update user %s in database", login)
					if err := dbClient.updateUser(u); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}
