package crawler

import (
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/richardlt/stargazer/config"
	"github.com/richardlt/stargazer/crawler/github"
	"github.com/richardlt/stargazer/database"
)

func execTaskRepositoriesRoutine(pgClient *database.DB, mgoClient *DatabaseClient, ghClient github.Client, cfg config.Crawler) error {
	es, err := pgClient.GetAllWithStatus(database.StatusRequested)
	if err != nil {
		return err
	}

	for _, e := range es {
		invalid, err := CheckTaskRepositoryRoutine(pgClient, mgoClient, ghClient, cfg, e)
		if invalid {
			logrus.Infof("execTaskRepositoriesRoutine: delete entry for %s: %v", e.Repository, err)
			return pgClient.Delete(e.Repository)
		} else if err != nil {
			return err
		}

		// Load stargazer for repo
		if err := LoadStargazerForRepo(mgoClient, ghClient, cfg, e); err != nil {
			return err
		}

		if err := ComputeTaskRepositoryRoutine(pgClient, mgoClient, e); err != nil {
			return err
		}
	}

	return nil
}

func CheckTaskRepositoryRoutine(pgClient *database.DB, mgoClient *DatabaseClient, ghClient github.Client, cfg config.Crawler, e database.Entry) (bool, error) {
	logrus.Infof("execTaskRepositoryRoutine: starting scan for repository %s", e.Repository)

	// Check that repository path is valid
	rs := strings.Split(e.Repository, "/")
	if len(rs) != 2 {
		return true, errors.Errorf("invalid repository path %s", e.Repository)
	}
	owner := rs[0]

	// Check if repository was not excluded
	var excluded bool
	for i := range cfg.TaskRepositoryExclusions {
		if strings.ToLower(cfg.TaskRepositoryExclusions[i]) == strings.ToLower(e.Repository) {
			excluded = true
			break
		}
	}
	if excluded {
		return true, errors.Errorf("exluded repository %s", e.Repository)
	}

	// Check that the repository owner starred the main repository
	// For organization repository, first check that one stargazer of the main repository is in the organization
	exists, err := mgoClient.existsOneOfRepositoryStargazer(cfg.MainRepository, owner)
	if err != nil {
		return false, err
	}
	if !exists {
		return true, errors.Errorf("no stargazer found on main repo for %s", e.Repository)
	}

	// Load the repository from GH
	ghRepo, err := ghClient.GetRepository(e.Repository)
	if err != nil {
		return true, errors.Errorf("repository not found on GH %s", e.Repository)
	}

	if ghRepo.Owner.Type == "Organization" {
		logrus.Debugf("execTaskRepositoryRoutine: repository owner is an organization, checking contributors for %s", e.Repository)
		contributors, err := ghClient.GetRepositoryConributors(ghRepo.FullName)
		if err != nil || len(contributors) == 0 {
			return true, errors.Errorf("repository contributors not found on GH %s", e.Repository)
		}
		logins := make([]string, len(contributors))
		for i := 0; i < int(cfg.TaskRepositoryOrgContributorsToCheck); i++ {
			logins[i] = contributors[i].Login
		}

		// For organization repository we check that one of the top contributors starred the main repository
		exists, err := mgoClient.existsOneOfRepositoryStargazer(cfg.MainRepository, logins...)
		if err != nil {
			return false, err
		}
		if !exists {
			return true, errors.Errorf("no contributors stargazer found on main repo for %s", e.Repository)
		}
	}

	logrus.Debugf("stargazer routine: get repository %s from database", e.Repository)
	r, err := mgoClient.getRepository(e.Repository)
	if err != nil {
		return false, err
	}
	if r == nil {
		logrus.Debugf("stargazer routine: create repository %s in database", e.Repository)
		return false, mgoClient.insertRepository(&repository{
			Path: e.Repository,
			Data: ghRepo,
		})
	}
	logrus.Debugf("stargazer routine: update repository %s in database", e.Repository)
	r.Data = ghRepo
	return false, mgoClient.updateRepository(r)
}

func LoadStargazerForRepo(mgoClient *DatabaseClient, ghClient github.Client, cfg config.Crawler, e database.Entry) error {
	r, err := mgoClient.getRepository(e.Repository)
	if err != nil {
		return err
	}

	expectedPageCount := int64((r.Data.StargazersCount / 100) + 1)
	if expectedPageCount > 400 { // GH limit on page count is 400
		expectedPageCount = 400
	}
	logrus.Infof("stargazer routine: load stargazers for repo %s from Github from %d pages expected", r.Path, expectedPageCount)
	getPage := func(path string, page, expectedPageCount int64) ([]stargazer, error) {
		logrus.Infof("stargazer routine: load stargazers page %d for repo %s from Github", page, r.Path)
		os, err := ghClient.GetRepositoryStargazerPage(r.Path, page)
		if err != nil {
			return nil, err
		}
		ss := make([]stargazer, len(os))
		for i := range os {
			ss[i].RepositoryID = r.ID
			ss[i].RepositoryPath = r.Path
			ss[i].Page = page
			ss[i].LastPage = page == expectedPageCount
			ss[i].Data = os[i]
		}
		return ss, nil
	}

	ss := make([]stargazer, 0, cfg.TaskRepositoryMaxStargazerPages*100)

	if expectedPageCount > cfg.TaskRepositoryMaxStargazerPages && r.Path != cfg.MainRepository {
		var page int64
		for i := cfg.TaskRepositoryMaxStargazerPages; i > 0; i-- {
			if i == cfg.TaskRepositoryMaxStargazerPages {
				page = expectedPageCount
			} else if i > 1 {
				page -= int64(float64(page) / float64(i))
			} else {
				page = 1
			}
			os, err := getPage(r.Path, page, expectedPageCount)
			if err != nil {
				return err
			}
			ss = append(ss, os...)
		}
	} else {
		for i := int64(1); i <= expectedPageCount; i++ {
			os, err := getPage(r.Path, i, expectedPageCount)
			if err != nil {
				return err
			}
			ss = append(ss, os...)
		}
	}

	logrus.Infof("stargazer routine: delete all stargazers for repository %s in database", r.Path)
	if err := mgoClient.deleteStargazers(r.ID); err != nil {
		return err
	}

	logrus.Infof("stargazer routine: insert %d stargazers for repository %s in database", len(ss), r.Path)
	if err := mgoClient.insertStargazers(ss); err != nil {
		return err
	}

	return nil
}

func ComputeTaskRepositoryRoutine(pgClient *database.DB, mgoClient *DatabaseClient, e database.Entry) error {
	logrus.Debugf("execTaskRepositoryRoutine: starting compute stats for repo for %s", e.Repository)
	r, err := mgoClient.getRepository(e.Repository)
	if err != nil {
		return err
	}

	e.Stats.CountStars = r.Data.StargazersCount

	// Compute evolution stats
	msPage, err := mgoClient.getRepoStarCountPerDaysAndPage(r.Path)
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
	ms, err := mgoClient.getRepoStarCountPerDays(r.Path)
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
	ss, err := mgoClient.getLast10Stargazers(r.Path)
	if err != nil {
		return err
	}

	e.Stats.Last10 = make([]database.Stargazer, 10)
	for i := range ss {
		e.Stats.Last10[i] = database.Stargazer{Name: ss[i].Data.User.Login}
	}

	logrus.Debugf("execTaskRepositoryRoutine: end computing stats for repo for %s", e.Repository)

	e.Status = database.StatusGenerated
	e.LastGeneratedAt = time.Now()
	return pgClient.Update(&e)
}
