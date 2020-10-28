package crawler_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/richardlt/stargazer/config"
	"github.com/richardlt/stargazer/crawler"
	"github.com/richardlt/stargazer/crawler/mock_github"
	"github.com/richardlt/stargazer/database"
)

func TestCheckTaskRepositoryRoutine(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	pg, err := database.New("postgres://stargazer:stargazer@localhost:5432/stargazer?sslmode=disable")
	require.NoError(t, err)

	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	require.NoError(t, err)
	require.NoError(t, client.Connect(context.TODO()))
	mgo := crawler.NewMongoClient(client.Database("stargazer"))
	require.NoError(t, mgo.Init())

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	ghClient := mock_github.NewMockClient(ctrl)

	invalid, err := crawler.CheckTaskRepositoryRoutine(pg, mgo, ghClient, config.Crawler{}, database.Entry{Repository: "ownerrepo"})
	require.True(t, invalid)
	require.Error(t, err)
	require.Equal(t, "invalid repository path ownerrepo", err.Error())

	invalid, err = crawler.CheckTaskRepositoryRoutine(pg, mgo, ghClient, config.Crawler{
		TaskRepositoryExclusions: []string{"owner/repo"},
	}, database.Entry{Repository: "owner/repo"})
	require.True(t, invalid)
	require.Error(t, err)
	require.Equal(t, "exluded repository owner/repo", err.Error())
}
