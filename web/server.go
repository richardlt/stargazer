package web

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/richardlt/stargazer/database"
)

type Server struct {
	router          *mux.Router
	db              *database.DB
	regenerateDelay int64
	mainRepository  string
	maxEntriesCount int64
	ts              *template.Template
}

func (s *Server) initRouter() error {
	var err error
	s.ts, err = template.New("stargazer").ParseFiles(
		"./templates/home.html",
		"./templates/repository.html",
	)
	if err != nil {
		return err
	}

	r := mux.NewRouter()
	r.HandleFunc("/{organization}/{repository}", s.repositoryPageHandler())
	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "./favicon.ico") })
	r.NotFoundHandler = s.homeHandler()

	s.router = r

	return nil
}

func (s *Server) Close() {
	s.db.Close()
}

func (s *Server) Start(port int64) error {
	logrus.Infof("Starting webserver at :%d", port)
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      s.router,
	}
	return errors.WithStack(srv.ListenAndServe())
}
