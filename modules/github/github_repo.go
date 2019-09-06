package github

import (
	"context"
	"fmt"
	"net/http"

	ghb "github.com/google/go-github/v26/github"
	"github.com/wtfutil/wtf/utils"
	"golang.org/x/oauth2"
)

// GithubRepo defines a new GithubRepo structure
type GithubRepo struct {
	apiKey    string
	baseURL   string
	uploadURL string

	Name         string
	Owner        string
	PullRequests []*ghb.PullRequest
	RemoteRepo   *ghb.Repository
	Err          error
}

// NewGithubRepo returns a new Github Repo with a name, owner, apiKey, baseURL and uploadURL
func NewGithubRepo(name, owner, apiKey, baseURL, uploadURL string) *GithubRepo {
	repo := GithubRepo{
		Name:  name,
		Owner: owner,

		apiKey:    apiKey,
		baseURL:   baseURL,
		uploadURL: uploadURL,
	}

	return &repo
}

// Open will open the GitHub Repo URL using the utils helper
func (repo *GithubRepo) Open() {
	utils.OpenFile(*repo.RemoteRepo.HTMLURL)
}

// Refresh reloads the github data via the Github API
func (repo *GithubRepo) Refresh() {
	prs, err := repo.loadPullRequests()
	repo.Err = err
	repo.PullRequests = prs
	if err != nil {
		return
	}
	remote, err := repo.loadRemoteRepository()
	repo.Err = err
	repo.RemoteRepo = remote
}

/* -------------------- Counts -------------------- */

// IssueCount return the total amount of issues as an int
func (repo *GithubRepo) IssueCount() int {
	if repo.RemoteRepo == nil {
		return 0
	}

	return *repo.RemoteRepo.OpenIssuesCount
}

// PullRequestCount returns the total amount of pull requests as an int
func (repo *GithubRepo) PullRequestCount() int {
	return len(repo.PullRequests)
}

// StarCount returns the total amount of stars this repo has gained as an int
func (repo *GithubRepo) StarCount() int {
	if repo.RemoteRepo == nil {
		return 0
	}

	return *repo.RemoteRepo.StargazersCount
}

/* -------------------- Unexported Functions -------------------- */

func (repo *GithubRepo) isGitHubEnterprise() bool {
	if len(repo.baseURL) > 0 {
		if len(repo.uploadURL) == 0 {
			repo.uploadURL = repo.baseURL
		}
		return true
	}
	return false
}

func (repo *GithubRepo) oauthClient() *http.Client {
	tokenService := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: repo.apiKey},
	)

	return oauth2.NewClient(context.Background(), tokenService)
}

func (repo *GithubRepo) githubClient() (*ghb.Client, error) {
	oauthClient := repo.oauthClient()

	if repo.isGitHubEnterprise() {
		return ghb.NewEnterpriseClient(repo.baseURL, repo.uploadURL, oauthClient)
	}

	return ghb.NewClient(oauthClient), nil
}

// myPullRequests returns a list of pull requests created by username on this repo
func (repo *GithubRepo) myPullRequests(username string, showStatus bool) []*ghb.PullRequest {
	prs := []*ghb.PullRequest{}

	for _, pr := range repo.PullRequests {
		user := *pr.User

		if *user.Login == username {
			prs = append(prs, pr)
		}
	}

	if showStatus {
		prs = repo.individualPRs(prs)
	}

	return prs
}

// individualPRs takes a list of pull requests (presumably returned from
// github.PullRequests.List) and fetches them individually to get more detailed
// status info on each. see: https://developer.github.com/v3/git/#checking-mergeability-of-pull-requests
func (repo *GithubRepo) individualPRs(prs []*ghb.PullRequest) []*ghb.PullRequest {
	github, err := repo.githubClient()
	if err != nil {
		return prs
	}

	var ret []*ghb.PullRequest
	for i := range prs {
		pr, _, err := github.PullRequests.Get(context.Background(), repo.Owner, repo.Name, prs[i].GetNumber())
		if err != nil {
			// worst case, just keep the original one
			ret = append(ret, prs[i])
		} else {
			ret = append(ret, pr)
		}
	}
	return ret
}

// myReviewRequests returns a list of pull requests for which username has been
// requested to do a code review
func (repo *GithubRepo) myReviewRequests(username string) []*ghb.PullRequest {
	prs := []*ghb.PullRequest{}

	for _, pr := range repo.PullRequests {
		for _, reviewer := range pr.RequestedReviewers {
			if *reviewer.Login == username {
				prs = append(prs, pr)
			}
		}
	}

	return prs
}

func (repo *GithubRepo) customIssueQuery(filter string, perPage int) *ghb.IssuesSearchResult {
	github, err := repo.githubClient()
	if err != nil {
		return nil
	}

	opts := &ghb.SearchOptions{}
	if perPage != 0 {
		opts.ListOptions.PerPage = perPage
	}

	prs, _, _ := github.Search.Issues(context.Background(), fmt.Sprintf("%s repo:%s/%s", filter, repo.Owner, repo.Name), opts)
	return prs
}

func (repo *GithubRepo) loadPullRequests() ([]*ghb.PullRequest, error) {
	github, err := repo.githubClient()
	if err != nil {
		return nil, err
	}

	opts := &ghb.PullRequestListOptions{}
	opts.ListOptions.PerPage = 100

	prs, _, err := github.PullRequests.List(context.Background(), repo.Owner, repo.Name, opts)

	if err != nil {
		return nil, err
	}

	return prs, nil
}

func (repo *GithubRepo) loadRemoteRepository() (*ghb.Repository, error) {
	github, err := repo.githubClient()

	if err != nil {
		return nil, err
	}

	repository, _, err := github.Repositories.Get(context.Background(), repo.Owner, repo.Name)

	if err != nil {
		return nil, err
	}

	return repository, nil
}
