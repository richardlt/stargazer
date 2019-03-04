package crawler

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	ghBaseURL = "https://api.github.com"
)

type githubClient struct {
	mutex        sync.RWMutex
	RequestCount int64
	token        string
}

func (c *githubClient) resetRequestCount() {
	c.mutex.Lock()
	c.RequestCount = 0
	c.mutex.Unlock()
}

func (c *githubClient) getRequestCount() int64 {
	c.mutex.RLock()
	v := c.RequestCount
	c.mutex.RUnlock()
	return v
}

func (c *githubClient) get(url string, modifiers ...func(req *http.Request)) ([]byte, error) {
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

func (c *githubClient) getPaginate(url string, modifiers ...func(req *http.Request)) ([]object, error) {
	var res []object

	page := 1
	for {
		logrus.Debugf("getPaginate: load page %d from %s\n", page, url)

		buf, err := c.get(fmt.Sprintf("%s?page=%d&per_page=100", url, page), modifiers...)
		if err != nil {
			return nil, err
		}

		var os []object
		if err := json.Unmarshal(buf, &os); err != nil {
			return nil, errors.WithStack(err)
		}
		res = append(res, os...)

		if len(os) < 100 {
			break
		}
		page++
	}

	return res, nil
}

func (c *githubClient) getRepository(path string) (object, error) {
	var o object

	buf, err := c.get(fmt.Sprintf("%s/repos/%s", ghBaseURL, path))
	if err != nil {
		return o, err
	}

	if err := json.Unmarshal(buf, &o); err != nil {
		return o, errors.WithStack(err)
	}

	return o, nil
}

func (c *githubClient) getRepositoryStargazer(path string) ([]object, error) {
	os, err := c.getPaginate(
		fmt.Sprintf("%s/repos/%s/stargazers", ghBaseURL, path),
		func(req *http.Request) { req.Header.Add("Accept", "application/vnd.github.v3.star+json") },
	)
	if err != nil {
		return nil, err
	}
	return os, nil
}

func (c *githubClient) getRepositoryStargazerPage(path string, page int64) ([]object, error) {
	buf, err := c.get(
		fmt.Sprintf("%s/repos/%s/stargazers?page=%d&per_page=100", ghBaseURL, path, page),
		func(req *http.Request) { req.Header.Add("Accept", "application/vnd.github.v3.star+json") },
	)
	if err != nil {
		return nil, err
	}

	var os []object
	if err := json.Unmarshal(buf, &os); err != nil {
		return nil, errors.WithStack(err)
	}

	return os, nil
}

func (c *githubClient) getUser(login string) (object, error) {
	var o object

	buf, err := c.get(fmt.Sprintf("%s/users/%s", ghBaseURL, login))
	if err != nil {
		return o, err
	}

	if err := json.Unmarshal(buf, &o); err != nil {
		return o, errors.WithStack(err)
	}

	return o, nil
}

func (c *githubClient) getUserOrganizations(login string) ([]object, error) {
	os, err := c.getPaginate(fmt.Sprintf("%s/users/%s/orgs", ghBaseURL, login))
	if err != nil {
		return nil, err
	}
	return os, nil
}
