package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const ghBaseURL = "https://api.github.com"

type Client interface {
	GetRepository(path string) (Repository, error)
	GetRepositoryConributors(path string) ([]Contributor, error)
	GetRepositoryStargazer(path string) ([]Stargazer, error)
	GetRepositoryStargazerPage(path string, page int64) ([]Stargazer, error)
	GetUser(login string) (User, error)
	GetUserOrganizations(login string) ([]Organization, error)
	ResetRequestCount()
	GetRequestCount() int64
}

var _ Client = new(client)

func NewClient(token string) Client {
	return &client{token: token}
}

type client struct {
	mutex        sync.RWMutex
	RequestCount int64
	token        string
}

func (c *client) ResetRequestCount() {
	c.mutex.Lock()
	c.RequestCount = 0
	c.mutex.Unlock()
}

func (c *client) GetRequestCount() int64 {
	c.mutex.RLock()
	v := c.RequestCount
	c.mutex.RUnlock()
	return v
}

func (c *client) get(url string, modifiers ...func(req *http.Request)) (json.RawMessage, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for i := range modifiers {
		modifiers[i](req)
	}

	c.mutex.Lock()
	c.RequestCount++
	c.mutex.Unlock()

	req.Header.Add("Authorization", fmt.Sprintf("token %s", c.token))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("error request at %s with code %d: body=%s", url, res.StatusCode, string(buf)))
	}
	return buf, nil
}

func (c *client) getPaginate(url string, modifiers ...func(req *http.Request)) (json.RawMessage, error) {
	var res []interface{}

	page := 1
	for {
		logrus.Debugf("getPaginate: load page %d from %s\n", page, url)

		data, err := c.get(fmt.Sprintf("%s?page=%d&per_page=100", url, page), modifiers...)
		if err != nil {
			return nil, err
		}

		var is []interface{}
		if err := json.Unmarshal(data, &is); err != nil {
			return nil, errors.WithStack(err)
		}
		res = append(res, is...)
		if len(is) < 100 {
			break
		}
		page++
	}

	buf, err := json.Marshal(res)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return buf, nil
}

func (c *client) GetRepository(path string) (Repository, error) {
	var r Repository
	buf, err := c.get(fmt.Sprintf("%s/repos/%s", ghBaseURL, path))
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(buf, &r); err != nil {
		return r, errors.WithStack(err)
	}
	return r, nil
}

func (c *client) GetRepositoryConributors(path string) ([]Contributor, error) {
	buf, err := c.get(fmt.Sprintf("%s/repos/%s/contributors", ghBaseURL, path))
	if err != nil {
		return nil, err
	}
	var cs []Contributor
	if err := json.Unmarshal(buf, &cs); err != nil {
		return nil, errors.WithStack(err)
	}
	return cs, nil
}

func (c *client) GetRepositoryStargazer(path string) ([]Stargazer, error) {
	data, err := c.getPaginate(
		fmt.Sprintf("%s/repos/%s/stargazers", ghBaseURL, path),
		func(req *http.Request) { req.Header.Add("Accept", "application/vnd.github.v3.star+json") },
	)
	if err != nil {
		return nil, err
	}
	var ss []Stargazer
	if err := json.Unmarshal(data, &ss); err != nil {
		return nil, err
	}
	return ss, nil
}

func (c *client) GetRepositoryStargazerPage(path string, page int64) ([]Stargazer, error) {
	buf, err := c.get(
		fmt.Sprintf("%s/repos/%s/stargazers?page=%d&per_page=100", ghBaseURL, path, page),
		func(req *http.Request) { req.Header.Add("Accept", "application/vnd.github.v3.star+json") },
	)
	if err != nil {
		return nil, err
	}
	var ss []Stargazer
	if err := json.Unmarshal(buf, &ss); err != nil {
		return nil, errors.WithStack(err)
	}
	return ss, nil
}

func (c *client) GetUser(login string) (User, error) {
	var u User
	buf, err := c.get(fmt.Sprintf("%s/users/%s", ghBaseURL, login))
	if err != nil {
		return u, err
	}
	if err := json.Unmarshal(buf, &u); err != nil {
		return u, errors.WithStack(err)
	}
	return u, nil
}

func (c *client) GetUserOrganizations(login string) ([]Organization, error) {
	data, err := c.getPaginate(fmt.Sprintf("%s/users/%s/orgs", ghBaseURL, login))
	if err != nil {
		return nil, err
	}
	var os []Organization
	if err := json.Unmarshal(data, &os); err != nil {
		return nil, err
	}
	return os, nil
}
