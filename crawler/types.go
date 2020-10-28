package crawler

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/richardlt/stargazer/crawler/github"
)

type repository struct {
	ID   primitive.ObjectID `bson:"_id" json:"-"`
	Path string             `bson:"path" json:"path"`
	Data github.Repository  `bson:"data" json:"data"`
}

type stargazer struct {
	ID             primitive.ObjectID `bson:"_id" json:"-"`
	RepositoryID   primitive.ObjectID `bson:"_repository_id" json:"-"`
	RepositoryPath string             `bson:"repository_path" json:"-"`
	Page           int64              `bson:"page" json:"page"`
	LastPage       bool               `bson:"last_page" json:"last_page"`
	Data           github.Stargazer   `bson:"data" json:"data"`
}

type user struct {
	ID            primitive.ObjectID    `bson:"_id" json:"-"`
	Expire        time.Time             `bson:"expire" json:"expire"`
	Login         string                `bson:"login" json:"login"`
	Data          github.User           `bson:"data" json:"data"`
	Organizations []github.Organization `bson:"organizations" json:"organizations"`
}

type measure struct {
	Date  time.Time `bson:"date"`
	Page  int64     `bson:"page"`
	Count int64     `bson:"count"`
}
