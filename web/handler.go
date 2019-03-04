package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/richardlt/stargazer/database"
)

func (s *Server) homeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.ts.ExecuteTemplate(w, "home", map[string]interface{}{}); err != nil {
			logrus.Errorf("%+v", errors.WithStack(err))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
}

func (s *Server) repositoryPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		organization := vars["organization"]
		repository := vars["repository"]

		if organization == "" || repository == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		repoPath := organization + "/" + repository

		e, err := s.db.Get(repoPath)
		if err != nil && errors.Cause(err) != gorm.ErrRecordNotFound {
			logrus.Errorf("%+v", errors.WithStack(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if e == nil {
			entriesCount, err := s.db.Count()
			if err != nil {
				logrus.Errorf("%+v", errors.WithStack(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if entriesCount >= s.maxEntriesCount {
				logrus.Warnf("%+v", errors.WithStack(fmt.Errorf("max entries count reached %d/%d", entriesCount, s.maxEntriesCount)))
				w.WriteHeader(http.StatusNotFound)
				return
			}

			e = &database.Entry{
				Repository: repoPath,
				Status:     database.StatusRequested,
			}
			if err := s.db.Create(e); err != nil {
				logrus.Errorf("%+v", errors.WithStack(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			logrus.Debugf("New entry created for repository: %s", repoPath)
		} else {
			e.LastRequestedAt = time.Now()

			// If stats expired, chnage the status to requested
			canRefresh := s.regenerateDelay == 0 || e.LastRequestedAt.Sub(e.LastGeneratedAt) > time.Duration(s.regenerateDelay)*time.Second
			if e.Status == database.StatusGenerated && canRefresh {
				e.Status = database.StatusRequested
			}

			if err := s.db.Update(e); err != nil {
				logrus.Errorf("%+v", errors.WithStack(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			logrus.Debugf("Entry updated for repository: %s", repoPath)
		}

		buf, err := json.Marshal(e.Stats)
		if err != nil {
			logrus.Errorf("%+v", errors.WithStack(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := s.ts.ExecuteTemplate(w, "repository", map[string]interface{}{
			"main_repository":          s.mainRepository,
			"entry":                    *e,
			"stats_json":               string(buf),
			"last_generated_at_string": e.LastGeneratedAt.UTC().Format(time.RFC822),
			"regenerate_delay_human":   (time.Duration(s.regenerateDelay) * time.Second).String(),
		}); err != nil {
			logrus.Errorf("%+v", errors.WithStack(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
