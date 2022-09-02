// Copyright 2022 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

type graph map[string][]string

// GetValidNameFromFolder returns a valid kubernetes name for a folder
func GetValidNameFromFolder(folder string) (string, error) {
	dir, err := filepath.Abs(folder)
	if err != nil {
		return "", fmt.Errorf("error inferring name: %s", err)
	}
	if folder == ".okteto" {
		dir = filepath.Dir(dir)
	}

	name := filepath.Base(dir)
	name = strings.ToLower(name)
	name = ValidKubeNameRegex.ReplaceAllString(name, "-")
	name = strings.TrimPrefix(name, "-")
	name = strings.TrimSuffix(name, "-")
	oktetoLog.Infof("autogenerated name: %s", name)
	return name, nil
}

//GetValidNameFromFolder returns a valid kubernetes name for a folder
func GetValidNameFromGitRepo(folder string) (string, error) {
	repo, err := GetRepositoryURL(folder)
	if err != nil {
		return "", err
	}
	name := TranslateURLToName(repo)
	return name, nil
}

func TranslateURLToName(repo string) string {
	repoName := findRepoName(repo)

	if strings.HasSuffix(repoName, ".git") {
		repoName = repoName[:strings.LastIndex(repoName, ".git")]
	}
	name := ValidKubeNameRegex.ReplaceAllString(repoName, "-")
	return name
}
func findRepoName(repo string) string {
	possibleName := strings.ToLower(repo[strings.LastIndex(repo, "/")+1:])
	if possibleName == "" {
		possibleName = repo
		nthTrim := strings.Count(repo, "/")
		for i := 0; i < nthTrim-1; i++ {
			possibleName = strings.ToLower(possibleName[strings.Index(possibleName, "/")+1:])
		}
		possibleName = possibleName[:len(possibleName)-1]
	}
	return possibleName
}
func GetRepositoryURL(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("failed to analyze git repo: %w", err)
	}

	origin, err := repo.Remote("origin")
	if err != nil {
		if err != git.ErrRemoteNotFound {
			return "", fmt.Errorf("failed to get the git repo's remote configuration: %w", err)
		}
	}

	if origin != nil {
		return origin.Config().URLs[0], nil
	}

	remotes, err := repo.Remotes()
	if err != nil {
		return "", fmt.Errorf("failed to get git repo's remote information: %w", err)
	}

	if len(remotes) == 0 {
		return "", fmt.Errorf("git repo doesn't have any remote")
	}

	return remotes[0].Config().URLs[0], nil
}

func getDependentCyclic(g graph) []string {
	visited := make(map[string]bool)
	stack := make(map[string]bool)
	cycle := make([]string, 0)
	for svcName := range g {
		if dfs(g, svcName, visited, stack) {
			for svc, isInStack := range stack {
				if isInStack {
					cycle = append(cycle, svc)
				}
			}
			return cycle
		}
	}
	return cycle
}

func getDependentNodes(g graph, startingNodes []string) []string {
	initialLength := len(startingNodes)
	svcsToDeploySet := map[string]bool{}
	for _, svc := range startingNodes {
		svcsToDeploySet[svc] = true
	}
	for _, svcToDeploy := range startingNodes {
		for _, dependentSvc := range g[svcToDeploy] {
			if _, ok := svcsToDeploySet[dependentSvc]; ok {
				continue
			}
			startingNodes = append(startingNodes, dependentSvc)
			svcsToDeploySet[dependentSvc] = true
		}
	}
	if initialLength != len(startingNodes) {
		return getDependentNodes(g, startingNodes)
	}
	return startingNodes
}

func dfs(g graph, svcName string, visited, stack map[string]bool) bool {
	isVisited := visited[svcName]
	if !isVisited {
		visited[svcName] = true
		stack[svcName] = true

		svc := g[svcName]
		for _, dependentSvc := range svc {
			if !visited[dependentSvc] && dfs(g, dependentSvc, visited, stack) {
				return true
			} else if value, ok := stack[dependentSvc]; ok && value {
				return true
			}
		}
	}
	stack[svcName] = false
	return false
}

func pathExistsAndDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func getListDiff(l1, l2 []string) []string {
	var (
		longerList  []string
		shorterList []string
	)
	if len(l1) < len(l2) {
		shorterList = l1
		longerList = l2

	} else {
		shorterList = l2
		longerList = l1
	}

	shorterListSet := map[string]bool{}
	for _, svc := range shorterList {
		shorterListSet[svc] = true
	}
	added := []string{}
	for _, svcName := range longerList {
		if _, ok := shorterListSet[svcName]; ok {
			added = append(added, svcName)
		}
	}
	return added
}
