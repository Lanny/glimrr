package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"encoding/json"
	"net/http"
	"net/url"
)

type GLChangeData struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	Diff        string
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
}

type GLDiffRefs struct {
	BaseSHA  string `json:"base_sha"`
	HeadSHA  string `json:"head_sha"`
	StartSHA string `json:"start_sha"`
}

type GLMRData struct {
	Title        string
	CreatedAt    string `json:"created_at"`
	State        string
	TargetBranch string `json:"target_branch"`
	SourceBranch string `json:"source_branch"`
	Changes      []GLChangeData
	DiffRefs     GLDiffRefs `json:"diff_refs"`
}

type GLInstance struct {
	apiUrl string
	cache  map[string]([]byte)
}

func (gl *GLInstance) get(url string) ([]byte, error) {
	client := &http.Client{}

	jankLog(fmt.Sprintf("Requesting URL: %s\n", url))
	if cachedVal, present := gl.cache[url]; present {
		jankLog("Found in cache.\n")
		return cachedVal, nil
	} else {
		jankLog("Not found in cache, requesting.\n")
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Add("PRIVATE-TOKEN", os.Getenv("MRAAG_GL_TOKEN"))
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		gl.cache[url] = body
		serializedCache, err := json.Marshal(gl.cache)
		os.WriteFile("glimmrCache.json", serializedCache, 0644)

		return body, nil
	}
}

func (gl *GLInstance) Init() {
	seralizedCache, err := os.ReadFile("glimmrCache.json")
	if err != nil {
		jankLog("Unable to restore cache, creating empty.\n")
		gl.cache = make(map[string]([]byte))
	} else {
		jankLog("Restoring cache from file.\n")
		json.Unmarshal(seralizedCache, &gl.cache)
	}
}

func (gl *GLInstance) FetchMR(pid int, mrid int) (*GLMRData, error) {
	url := fmt.Sprintf("%s/v4/projects/%d/merge_requests/%d/changes", strings.TrimSuffix(gl.apiUrl, "/"), pid, mrid)
	body, err := gl.get(url)
	if err != nil {
		return nil, err
	}

	var parsedData GLMRData
	json.Unmarshal(body, &parsedData)

	return &parsedData, nil
}

func (gl *GLInstance) FetchFileContents(pid int, path string, ref string) (*string, error) {
	url := fmt.Sprintf("%s/v4/projects/%d/repository/files/%s/raw?ref=%s", strings.TrimSuffix(gl.apiUrl, "/"), pid, url.QueryEscape(path), url.QueryEscape(ref))
	body, err := gl.get(url)
	if err != nil {
		return nil, err
	}

	bodyAsStr := string(body)
	return &bodyAsStr, nil
}
