package binman

import (
	"context"
	"regexp"

	"github.com/google/go-github/v48/github"
	log "github.com/rjbrown57/binman/pkg/logging"
)

type GetGHTagAction struct {
	r        *BinmanRelease
	ghClient *github.Client
}

func (r *BinmanRelease) AddGetGHTagAction(ghClient *github.Client) Action {
	return &GetGHTagAction{
		r,
		ghClient,
	}
}

func (action *GetGHTagAction) execute() error {

	ctx := context.Background()

	log.Debugf("Querying github api for tag list for %s", action.r.Repo)

	opt := &github.ListOptions{
		PerPage: 50,
	}

	// get all pages of results

	// This should be moved to it's own function when proven
	var alltags []*github.RepositoryTag
	for {

		tag, resp, err := action.ghClient.Repositories.ListTags(ctx, action.r.org, action.r.project, opt)
		if err != nil {
			return err
		}

		alltags = append(alltags, tag...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	if action.r.TagRegex != "" {
		var filteredtags []*github.RepositoryTag

		log.Debugf("tagregex defined as %s", action.r.TagRegex)
		tr := regexp.MustCompile(action.r.TagRegex)

		for _, tag := range alltags {
			if tr.MatchString(tag.GetName()) {
				filteredtags = append(filteredtags, tag)
			}
		}
		alltags = filteredtags
	}

	// If the tags are semvers 0 will be the lexical latest
	version := alltags[0].GetName()
	log.Debugf("Selected tag %s", alltags[0].GetName())

	// Create a release and add our tag
	// TODO Refactor to avoid this requirement, since this is a bit confusing
	// TODO Refactor use of pointer here
	action.r.githubData = &github.RepositoryRelease{
		TagName: &version,
	}

	return nil
}

type GetGHLatestReleaseAction struct {
	r        *BinmanRelease
	ghClient *github.Client
}

func (r *BinmanRelease) AddGetGHLatestReleaseAction(ghClient *github.Client) Action {
	return &GetGHLatestReleaseAction{
		r,
		ghClient,
	}
}

func (action *GetGHLatestReleaseAction) execute() error {

	var err error

	ctx := context.Background()

	log.Debugf("Querying github api for latest release of %s", action.r.Repo)
	// https://docs.github.com/en/rest/releases/releases#get-the-latest-release
	action.r.githubData, _, err = action.ghClient.Repositories.GetLatestRelease(ctx, action.r.org, action.r.project)

	return err
}

type GetGHReleaseByTagsAction struct {
	r        *BinmanRelease
	ghClient *github.Client
}

func (r *BinmanRelease) AddGetGHReleaseByTagsAction(ghClient *github.Client) Action {
	return &GetGHReleaseByTagsAction{
		r,
		ghClient,
	}
}

func (action *GetGHReleaseByTagsAction) execute() error {

	var err error

	ctx := context.Background()

	log.Debugf("Querying github api for %s release of %s", action.r.Version, action.r.Repo)
	// https://docs.github.com/en/rest/releases/releases#get-the-latest-release
	action.r.githubData, _, err = action.ghClient.Repositories.GetReleaseByTag(ctx, action.r.org, action.r.project, action.r.Version)

	return err
}
