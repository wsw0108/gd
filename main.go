package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v39/github"
	"go.uber.org/multierr"
	"golang.org/x/oauth2"
)

var (
	owner    string
	repo     string
	branch   string
	repoPath string
	dir      string
)

func init() {
	flag.StringVar(&owner, "owner", "", "repo owner")
	flag.StringVar(&repo, "repo", "", "repo name")
	flag.StringVar(&branch, "branch", "master", "repo branch")
	flag.StringVar(&repoPath, "path", "", "repo path(directory/file)")
	flag.StringVar(&dir, "dir", "", "local directory")
	flag.Parse()
}

type file struct {
	path string
	url  string
}

func rawURL(owner string, repo string, sha string, path string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, sha, path)
}

func download(client *http.Client, file file, dir string) (err error) {
	resp, err := client.Get(file.url)
	if err != nil {
		return
	}
	defer func() {
		err = multierr.Append(err, resp.Body.Close())
	}()
	name := filepath.Join(dir, file.path)
	err = os.MkdirAll(filepath.Dir(name), 0755)
	if err != nil {
		return
	}
	w, err := os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	if err != nil {
		return
	}
	defer func() {
		err = multierr.Append(err, w.Close())
	}()
	_, err = io.Copy(w, resp.Body)
	return
}

func main() {
	if owner == "" || repo == "" {
		flag.Usage()
		os.Exit(-1)
	}

	token, ok := os.LookupEnv("GITHUB_TOKEN")
	if !ok {
		fmt.Println("Please set 'GITHUB_TOKEN' first.")
		os.Exit(-1)
	}

	ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	tree, _, err := client.Git.GetTree(ctx, owner, repo, branch, true)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	var ftype string
	var fpath string
	if repoPath != "" {
		for _, entry := range tree.Entries {
			epath := entry.GetPath()
			if epath == repoPath {
				ftype = entry.GetType()
				fpath = epath
				break
			}
		}
		if fpath == "" {
			fmt.Printf("can not find %s in repo\n", repoPath)
			os.Exit(-1)
		}
	}

	var files []file
	for _, entry := range tree.Entries {
		etype := entry.GetType()
		if etype != "blob" {
			continue
		}
		epath := entry.GetPath()
		if fpath == "" {
			files = append(files, file{
				path: epath,
				url:  rawURL(owner, repo, branch, epath),
			})
			continue
		}
		var prefix string
		if ftype == "tree" {
			prefix = fpath + "/"
		} else {
			prefix = fpath
		}
		if strings.HasPrefix(epath, prefix) {
			files = append(files, file{
				path: epath,
				url:  rawURL(owner, repo, branch, epath),
			})
		}
	}

	for _, file := range files {
		if err = download(client.Client(), file, dir); err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
	}
}
