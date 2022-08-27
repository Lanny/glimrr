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

type GLMRData struct {
	Title        string
	CreatedAt    string `json:"created_at"`
	State        string
	TargetBranch string `json:"target_branch"`
	SourceBranch string `json:"source_branch"`
	Changes      []GLChangeData
}

type GLInstance struct {
	apiUrl string
}

func (gl *GLInstance) FetchMR(pid int, mrid int) (*GLMRData, error) {
	client := &http.Client{}

	url := fmt.Sprintf("%s/v4/projects/%d/merge_requests/%d/changes", strings.TrimSuffix(gl.apiUrl, "/"), pid, mrid)
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

	var parsedData GLMRData
	json.Unmarshal(body, &parsedData)

	return &parsedData, nil
}

func (gl *GLInstance) FetchFileContents(pid int, path string, ref string) (*string, error) {
	client := &http.Client{}

	url := fmt.Sprintf("%s/v4/projects/%d/repository/files/%s/raw?ref=%s", strings.TrimSuffix(gl.apiUrl, "/"), pid, url.QueryEscape(path), url.QueryEscape(ref))
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
	bodyAsStr := string(body)

	return &bodyAsStr, nil
}
