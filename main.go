package main

import (
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"

	"github.com/richardlt/stargazer/config"
	"github.com/richardlt/stargazer/crawler"
	"github.com/richardlt/stargazer/web"
)

func main() {
	app := cli.NewApp()

	globalFlags := []cli.Flag{
		&cli.StringFlag{
			Name:    "pg-url",
			Value:   "postgres://stargazer:stargazer@localhost:5432/stargazer?sslmode=disable",
			Usage:   "Postgres database URL",
			EnvVars: []string{"STARGAZER_PG_URL", "DATABASE_URL"},
		},
		&cli.StringFlag{
			Name:    "log-level",
			Value:   "info",
			Usage:   "[panic fatal error warning info debug]",
			EnvVars: []string{"STARGAZER_LOG_LEVEL"},
		},
		&cli.StringFlag{
			Name:    "main-repository",
			Value:   "richardlt/stargazer",
			Usage:   "Set the path for main repository.",
			EnvVars: []string{"STARGAZER_MAIN_REPOSITORY"},
		},
		&cli.Int64Flag{
			Name:    "task-repository-org-contributors-to-check",
			Value:   10,
			Usage:   "Set the count of organization contributors to includes when checking for start on main repository.",
			EnvVars: []string{"STARGAZER_TASK_REPOSITORY_ORG_CONTRIBUTORS_TO_CHECK"},
		},
	}

	app.Commands = []*cli.Command{
		{
			Name: "crawler",
			Flags: append(globalFlags,
				&cli.StringFlag{
					Name:    "gh-token",
					Value:   "secret",
					Usage:   "Github api token",
					EnvVars: []string{"STARGAZER_GH_TOKEN"},
				},
				&cli.StringFlag{
					Name:    "mgo-uri",
					Value:   "mongodb://localhost:27017",
					Usage:   "Mongo database URI",
					EnvVars: []string{"STARGAZER_MGO_URI"},
				},
				&cli.Int64Flag{
					Name:    "user-expiration-delay",
					Value:   3600,
					Usage:   "Set expiration delay for users in seconds (0 means no expiration).",
					EnvVars: []string{"STARGAZER_USER_EXPIRATION_DELAY"},
				},
				&cli.Int64Flag{
					Name:    "main-repository-scan-delay",
					Value:   30,
					Usage:   "Set the delay for main repository scanner in seconds.",
					EnvVars: []string{"STARGAZER_MAIN_REPOSITORY_SCAN_DELAY"},
				},
				&cli.Int64Flag{
					Name:    "task-repository-scan-delay",
					Value:   30,
					Usage:   "Set the delay for task repository scanner in seconds.",
					EnvVars: []string{"STARGAZER_TASK_REPOSITORY_SCAN_DELAY"},
				},
				&cli.Int64Flag{
					Name:    "task-repository-max-stargazer-pages",
					Value:   10,
					Usage:   "Set the maximum stargazer pages to load for a repository.",
					EnvVars: []string{"STARGAZER_TASK_REPOSITORY_MAX_STARGAZER_PAGES"},
				},
				&cli.StringSliceFlag{
					Name:    "task-repository-exclusions",
					Value:   cli.NewStringSlice("richardlt/stargazer"),
					Usage:   "Set the repositories that you want to exclude from computing.",
					EnvVars: []string{"STARGAZER_TASK_REPOSITORY_EXCLUSIONS"},
				},
			),
			Action: func(c *cli.Context) error {
				level, err := logrus.ParseLevel(c.String("log-level"))
				if err != nil {
					return errors.Wrap(err, "invalid given log level")
				}

				return crawler.Start(config.Crawler{
					Common: config.Common{
						LogLevel:                             level,
						DatabaseURL:                          c.String("pg-url"),
						MainRepository:                       c.String("main-repository"),
						TaskRepositoryOrgContributorsToCheck: c.Int64("task-repository-org-contributors-to-check"),
					},
					MgoURI:                          c.String("mgo-uri"),
					GHToken:                         c.String("gh-token"),
					UserExpirationDelay:             c.Int64("user-expiration-delay"),
					MainRepositoryScanDelay:         c.Int64("main-repository-scan-delay"),
					TaskRepositoryScanDelay:         c.Int64("task-repository-scan-delay"),
					TaskRepositoryMaxStargazerPages: c.Int64("task-repository-max-stargazer-pages"),
					TaskRepositoryExclusions:        c.StringSlice("task-repository-exclusions"),
				})
			},
		},
		{
			Name: "web",
			Flags: append(globalFlags,
				&cli.Int64Flag{
					Name:    "port",
					Value:   8080,
					Usage:   "Stargazer webserver port",
					EnvVars: []string{"STARGAZER_PORT", "PORT"},
				},
				&cli.Int64Flag{
					Name:    "regenerate-delay",
					Value:   3600 * 24,
					Usage:   "Set the delay for stats regenaration in seconds.",
					EnvVars: []string{"STARGAZER_REGENERATE_DELAY"},
				},
				&cli.Int64Flag{
					Name:    "max-entries-count",
					Value:   100,
					Usage:   "Set the max count of entries to store in database.",
					EnvVars: []string{"STARGAZER_MAX_ENTRIES_COUNT"},
				},
			),
			Action: func(c *cli.Context) error {
				level, err := logrus.ParseLevel(c.String("log-level"))
				if err != nil {
					return errors.WithStack(err)
				}

				return web.Start(config.Web{
					Common: config.Common{
						LogLevel:                             level,
						DatabaseURL:                          c.String("pg-url"),
						MainRepository:                       c.String("main-repository"),
						TaskRepositoryOrgContributorsToCheck: c.Int64("task-repository-org-contributors-to-check"),
					},
					Port:            c.Int64("port"),
					RegenerateDelay: c.Int64("regenerate-delay"),
					MaxEntriesCount: c.Int64("max-entries-count"),
				})
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Errorf("%+v", err)
	}
}
