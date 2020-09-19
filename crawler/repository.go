package crawler

import (
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/richardlt/stargazer/database"
)

func execMainRepositoryRoutine(dbClient *databaseClient, ghClient *githubClient, repo string, userExpirationDelay int64) error {
	logrus.Infof("execMainRepositoryRoutine: get main repository %s from Github", repo)

	o, err := ghClient.getRepository(repo)
	if err != nil {
		return err
	}

	githubStargazersCount := int(o["stargazers_count"].(float64))

	logrus.Infof("execMainRepositoryRoutine: get repository %s from database", repo)
	r, err := dbClient.getRepository(repo)
	if err != nil {
		return err
	}

	repoExists := r != nil

	if !repoExists {
		r = &repository{
			Path: repo,
			Data: o,
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
			r.Data = o
			if err := dbClient.updateRepository(r); err != nil {
				return err
			}
		}

		logrus.Info("execMainRepositoryRoutine: load stargazers from Github")
		os, err := ghClient.getRepositoryStargazer(r.Path)
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
			rawUser := ss[i].Data["user"].(object)
			login := rawUser["login"].(string)

			u, err := dbClient.getUser(login)
			if err != nil {
				return err
			}
			needSave := u == nil || (u.Expire.Before(time.Now()) && userExpirationDelay > 0)
			if needSave {
				logrus.Infof("execMainRepositoryRoutine: get user %s from Github (%d/%d)", login, i+1, len(ss))
				o, err := ghClient.getUser(login)
				if err != nil {
					return err
				}
				os, err := ghClient.getUserOrganizations(login)
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

func execTaskRepositoryRoutine(pgClient *database.DB, mgoClient *databaseClient, ghClient *githubClient, mainRepo string, maxStargazerPageToScan int64, mainRepoPath string, exclusions []string) error {
	es, err := pgClient.GetAllWithStatus(database.StatusRequested)
	if err != nil {
		return err
	}

	for _, e := range es {
		logrus.Infof("execTaskRepositoryRoutine: starting scan for repository %s", e.Repository)

		// Check if repository was not excluded
		var excluded bool
		for i := range exclusions {
			if exclusions[i] == e.Repository {
				excluded = true
				break
			}
		}
		if excluded {
			logrus.Warnf("execTaskRepositoryRoutine: delete exluded repository %s", e.Repository)
			if err := pgClient.Delete(e.Repository); err != nil {
				return err
			}
			continue
		}

		rs := strings.Split(e.Repository, "/")
		if len(rs) != 2 {
			logrus.Warnf("execTaskRepositoryRoutine: invalid repository path %s", e.Repository)
			if err := pgClient.Delete(e.Repository); err != nil {
				return err
			}
			continue
		}
		owner := rs[0]

		// Check that the repository owner starred the main repository
		// For organization repository, first check that one stargazer of the main repository is in the organization
		exists, err := mgoClient.existsOneOfRepositoryStargazer(mainRepo, owner)
		if err != nil {
			return err
		}
		if !exists {
			logrus.Debugf("execTaskRepositoryRoutine: no stargazer found on main repo for %s", e.Repository)
			if err := pgClient.Delete(e.Repository); err != nil {
				return err
			}
			continue
		}

		// Load the repository from GH
		o, err := ghClient.getRepository(e.Repository)
		if err != nil {
			logrus.Debugf("execTaskRepositoryRoutine: repository not found on GH %s", e.Repository)
			if err := pgClient.Delete(e.Repository); err != nil {
				return err
			}
			continue
		}
		repoOwner := o["owner"].(map[string]interface{})
		repoOwnerType := repoOwner["type"].(string)

		if repoOwnerType == "Organization" {
			logrus.Debugf("execTaskRepositoryRoutine: repository owner is an organization, checking contributors for %s", e.Repository)
			contributors, err := ghClient.getRepositoryConributors(e.Repository)
			if err != nil || len(contributors) == 0 {
				logrus.Debugf("execTaskRepositoryRoutine: repository contributors not found on GH %s", e.Repository)
				if err := pgClient.Delete(e.Repository); err != nil {
					return err
				}
				continue
			}
			logins := make([]string, len(contributors))
			for i := 0; i < 3; i++ {
				logins[i] = contributors[i]["login"].(string)
			}

			// For organization repository we check that one of the top three contributors starred the main repository
			exists, err := mgoClient.existsOneOfRepositoryStargazer(mainRepo, logins...)
			if err != nil {
				return err
			}
			if !exists {
				logrus.Debugf("execTaskRepositoryRoutine: no contributors stargazer found on main repo for %s", e.Repository)
				if err := pgClient.Delete(e.Repository); err != nil {
					return err
				}
				continue
			}
		}

		logrus.Debugf("execTaskRepositoryRoutine: starting compute stats for repo for %s", e.Repository)

		// Load stargazer for repo
		if err := loadStargazerForRepo(mgoClient, ghClient, o, maxStargazerPageToScan, mainRepoPath); err != nil {
			return err
		}

		e.Stats.CountStars = int64(o["stargazers_count"].(float64))

		// Compute evolution stats
		msPage, err := mgoClient.getRepoStarCountPerDaysAndPage(e.Repository)
		if err != nil {
			return err
		}
		e.Stats.Evolution = nil
		if len(msPage) > 0 {
			previousPage := int64(1)
			count := int64(0)
			for i := range msPage {
				if previousPage < msPage[i].Page {
					pageGap := msPage[i].Page - previousPage
					if pageGap > 1 {
						count += (pageGap - 1) * 100
					}
				}
				previousPage = msPage[i].Page
				count += msPage[i].Count
				e.Stats.Evolution = append(e.Stats.Evolution, database.Measure{Date: msPage[i].Date, Count: count})
			}
			e.Stats.Evolution = append(e.Stats.Evolution, database.Measure{Date: time.Now(), Count: e.Stats.CountStars})
		}

		// Compute count per days stats
		ms, err := mgoClient.getRepoStarCountPerDays(e.Repository)
		if err != nil {
			return err
		}
		e.Stats.PerDays = nil
		if len(ms) > 0 {
			for i := 30; i > 0; i-- {
				if len(ms) >= i {
					m := ms[len(ms)-i]
					e.Stats.PerDays = append(e.Stats.PerDays, database.Measure{Date: m.Date, Count: m.Count})
				}
			}
		}

		// Set last stargazers
		ss, err := mgoClient.getLast10Stargazers(e.Repository)
		if err != nil {
			return err
		}

		e.Stats.Last10 = make([]database.Stargazer, 10)
		for i := range ss {
			rawUser := ss[i].Data["user"].(object)
			login := rawUser["login"].(string)
			e.Stats.Last10[i] = database.Stargazer{Name: login}
		}

		logrus.Debugf("execTaskRepositoryRoutine: end computing stats for repo for %s", e.Repository)

		e.Status = database.StatusGenerated
		e.LastGeneratedAt = time.Now()
		if err := pgClient.Update(&e); err != nil {
			return err
		}
	}

	return nil
}
