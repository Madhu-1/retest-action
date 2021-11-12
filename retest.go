package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var (
	retry         = flag.Int("maxretry", 5, "Maximum retry of retest of a test in a Pull Request")
	exemptlabel   = flag.String("exemptlabel", "DNM", "If the PullRequest is having this label, retest will be skipped")
	requiredlabel = flag.String("requiredlabel", "", "If the PullRequest is having this label, retest will be tried for failed tests")
	githubToken   = os.Getenv("GITHUB_TOKEN")
	owner, repo   = func() (string, string) {
		if os.Getenv("GITHUB_REPOSITORY") != "" {
			if len(strings.Split(os.Getenv("GITHUB_REPOSITORY"), "/")) == 2 {
				return strings.Split(os.Getenv("GITHUB_REPOSITORY"), "/")[0], strings.Split(os.Getenv("GITHUB_REPOSITORY"), "/")[1]
			}

		}
		return "", ""
	}()
)

func main() {

	flag.Parse()
	for _, e := range os.Environ() {
		log.Printf("the visible env variables %v\n", e)
	}
	if *requiredlabel == "" {
		log.Fatal("requiredlabels are not set")
	}

	if githubToken == "" {
		log.Fatal("GITHUB_TOKEN is not set")
	}

	if owner == "" || repo == "" {
		log.Fatal("GITHUB_REPOSITORY is not set")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	opt := &github.PullRequestListOptions{}
	req, _, err := client.PullRequests.List(context.TODO(), owner, repo, opt)
	if err != nil {
		log.Fatalf("failed to list pull requests %v\n", err)
	}
	for _, re := range req {
		if *re.State == "open" {
			prNumber := re.GetNumber()
			log.Printf("PR with ID %d with Title %q is open\n", prNumber, re.GetTitle())
			for _, r := range re.Labels {
				fmt.Println("found label", r.GetName())
				// check if label is exempt
				if strings.EqualFold(*exemptlabel, r.GetName()) {
					continue
				}
				// check if label is matching
				if !strings.EqualFold(*requiredlabel, r.GetName()) {
					continue
				}

				rs, _, err := client.Repositories.ListStatuses(context.TODO(), owner, repo, re.GetHead().GetSHA(), &github.ListOptions{})
				if err != nil {
					log.Printf("failed to list status %v\n", err)
					continue
				}

				creq, _, err := client.Issues.ListComments(context.Background(), owner, repo, prNumber, &github.IssueListCommentsOptions{})
				if err != nil {
					log.Printf("failed to list comments %v\n", err)
				}

				// check if retest limit is  reached

				for _, r := range rs {
					if r.GetState() == "failed" {
						log.Printf("found failed test %s\n", r.GetContext())

						retestCount := 0
						msg := fmt.Sprintf("/retest %s", r.GetContext())
						for _, pc := range creq {
							if pc.GetBody() == msg {
								retestCount += 1
							}
						}
						if retestCount >= *retry {
							log.Printf("Pull Requested %d: %q reached  maximum attempt. skipping retest %v\n", prNumber, r.GetContext(), retestCount)
							continue
						}
						comment := &github.IssueComment{

							Body: github.String(msg),
						}
						_, _, err := client.Issues.CreateComment(context.Background(), owner, repo, prNumber, comment)
						if err != nil {
							log.Printf("failed to create comment %v\n", err)
						}
						//Post comment with target URL for retesting
						msg = fmt.Sprintf("@%s %s test failed. Logs are available at %s for debugging", re.GetUser(), r.GetContext(), r.GetTargetURL())
						comment.Body = github.String(msg)
						_, _, err = client.Issues.CreateComment(context.Background(), owner, repo, prNumber, comment)
						if err != nil {
							log.Printf("failed to create comment %v\n", err)
						}
					}
				}
			}
		}
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
