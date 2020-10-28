package github

import "time"

type Repository struct {
	StargazersCount int64 `bson:"stargazers_count" json:"stargazers_count"`
	Owner           struct {
		Type string `bson:"type" json:"type"`
	} `bson:"owner" json:"owner"`
	FullName string `bson:"full_name" json:"full_name"`
}

type Contributor struct {
	Login string `bson:"login" json:"login"`
}

type Stargazer struct {
	User struct {
		Login string `bson:"login" json:"login"`
	} `bson:"user" json:"user"`
	StarredAt time.Time `bson:"starred_at" json:"starred_at"`
}

type User struct {
	Login string `bson:"login" json:"login"`
}

type Organization struct {
	Login string `bson:"login" json:"login"`
}
