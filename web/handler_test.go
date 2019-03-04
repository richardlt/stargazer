package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/richardlt/stargazer/config"
	"github.com/richardlt/stargazer/database"
)

func newTestServer(t *testing.T) (*mux.Router, *database.DB) {
	logrus.SetLevel(logrus.DebugLevel)

	db, err := database.New(config.Database{
		Host:     "localhost",
		Port:     5432,
		SSL:      false,
		Name:     "stargazer",
		User:     "stargazer",
		Password: "stargazer",
	})
	require.NoError(t, err)

	s := &Server{db: db}
	require.NoError(t, s.initRouter())

	return s.router, s.db
}

func Test_repositoryPageHandler_firstRequest(t *testing.T) {
	testStart := time.Now()

	r, db := newTestServer(t)

	require.NoError(t, db.Delete("richardlt/stargazer"))

	req, err := http.NewRequest("GET", "/richardlt/stargazer", nil)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	entry, err := db.Get("richardlt/stargazer")
	require.NoError(t, err)

	assert.Equal(t, database.StatusRequested, entry.Status)
	assert.True(t, testStart.Before(entry.LastRequestedAt))
}

func Test_repositoryPageHandler_secondRequest(t *testing.T) {
	r, db := newTestServer(t)

	require.NoError(t, db.Delete("richardlt/stargazer"))
	existingEntry := database.Entry{
		Repository: "richardlt/stargazer",
		Status:     database.StatusGenerated,
	}
	require.NoError(t, db.Create(&existingEntry))
	assert.True(t, existingEntry.LastRequestedAt.Equal(existingEntry.LastGeneratedAt))

	time.Sleep(time.Millisecond * 50)

	req, err := http.NewRequest("GET", "/richardlt/stargazer", nil)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	entry, err := db.Get("richardlt/stargazer")
	require.NoError(t, err)

	assert.Equal(t, database.StatusGenerated, entry.Status)
	assert.True(t, entry.LastGeneratedAt.Equal(existingEntry.LastGeneratedAt))
	assert.True(t, entry.LastRequestedAt.After(entry.LastGeneratedAt))
}

func Test_repositoryPageHandler_refreshRequest(t *testing.T) {
	r, db := newTestServer(t)

	require.NoError(t, db.Delete("richardlt/stargazer"))
	existingEntry := database.Entry{
		Repository:      "richardlt/stargazer",
		Status:          database.StatusGenerated,
		LastGeneratedAt: time.Now().Add(-25 * time.Hour),
		LastRequestedAt: time.Now(),
	}
	require.NoError(t, db.Create(&existingEntry))

	req, err := http.NewRequest("GET", "/richardlt/stargazer", nil)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	entry, err := db.Get("richardlt/stargazer")
	require.NoError(t, err)

	assert.Equal(t, database.StatusRequested, entry.Status)
}
