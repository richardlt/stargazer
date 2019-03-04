package crawler

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type object map[string]interface{}

type repository struct {
	ID   primitive.ObjectID `bson:"_id" json:"-"`
	Path string             `bson:"path" json:"path"`
	Data object             `bson:"data" json:"data"`
}

type stargazer struct {
	ID             primitive.ObjectID `bson:"_id" json:"-"`
	RepositoryID   primitive.ObjectID `bson:"_repository_id" json:"-"`
	RepositoryPath string             `bson:"repository_path" json:"-"`
	Page           int64              `bson:"page" json:"page"`
	LastPage       bool               `bson:"last_page" json:"last_page"`
	Data           object             `bson:"data" json:"data"`
}

type user struct {
	ID            primitive.ObjectID `bson:"_id" json:"-"`
	Expire        time.Time          `bson:"expire" json:"expire"`
	Login         string             `bson:"login" json:"login"`
	Data          object             `bson:"data" json:"data"`
	Organizations []object           `bson:"organizations" json:"organizations"`
}

type measure struct {
	Date  time.Time `bson:"date"`
	Page  int64     `bson:"page"`
	Count int64     `bson:"count"`
}
