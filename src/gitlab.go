package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type GLChangeData struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	Diff        string
	NewFile     bool `json:"new_file"`
	RenamedFile bool `json:"renamed_file"`
	DeletedFile bool `json:"deleted_file"`
}

type GLDiffRefs struct {
	BaseSHA  string `json:"base_sha"`
	HeadSHA  string `json:"head_sha"`
	StartSHA string `json:"start_sha"`
}

type GLAuthor struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

type GLPosition struct {
	BaseSHA      string `json:"base_sha"`
	HeadSHA      string `json:"head_sha"`
	StartSHA     string `json:"start_sha"`
	PositionType string `json:"position_type"`
	OldLine      int    `json:"old_line"`
	NewLine      int    `json:"new_line"`
	NewPath      string `json:"new_path"`
	OldPath      string `json:"old_path"`
}

type GLNote struct {
	Id        int        `json:"id"`
	Author    GLAuthor   `json:"author"`
	Type      string     `json:"type"`
	Body      string     `json:"body"`
	CreatedAt string     `json:"created_at"`
	UpdatedAt string     `json:"updated_at"`
	Position  GLPosition `json:"position"`
}

type GLDiscussion struct {
	Id    string   `json:"id"`
	Notes []GLNote `json:"notes"`
}

type GLMRData struct {
	Id           int            `json:"id"`
	Iid          int            `json:"iid"`
	ProjectId    int            `json:"project_id"`
	Title        string         `json:"title"`
	CreatedAt    string         `json:"created_at"`
	State        string         `json:"state"`
	TargetBranch string         `json:"target_branch"`
	SourceBranch string         `json:"source_branch"`
	Changes      []GLChangeData `json:"changes"`
	DiffRefs     GLDiffRefs     `json:"diff_refs"`
	Discussions  []GLDiscussion
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
		req.Header.Add("PRIVATE-TOKEN", os.Getenv("GLIMRR_TOKEN"))
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			ln("Response body for GET request to %s:\n```\n%s\n```", url, string(body))
			return nil, fmt.Errorf("Request to %s failed with status code %d", url, resp.StatusCode)
		}

		gl.cache[url] = body
		serializedCache, err := json.Marshal(gl.cache)
		os.WriteFile("glimrrCache.json", serializedCache, 0644)

		return body, nil
	}
}

func (gl *GLInstance) postForm(url string, form url.Values) ([]byte, error) {
	client := &http.Client{}

	ln("Posting to URL: %s", url)
	req, err := http.NewRequest("POST", url, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Add("PRIVATE-TOKEN", os.Getenv("GLIMRR_TOKEN"))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		ln("Response body for POST request to %s:\n```\n%s\n```", url, string(body))
		return nil, fmt.Errorf("Request to %s failed with status code %d", url, resp.StatusCode)
	}

	return body, nil
}

func (gl *GLInstance) Init() {
	seralizedCache, err := os.ReadFile("glimrrCache.json")
	if err != nil {
		ln("Unable to restore cache, creating empty.")
		ln("Error: %s", err.Error())
		gl.cache = make(map[string]([]byte))
	} else {
		jankLog("Restoring cache from file.\n")
		json.Unmarshal(seralizedCache, &gl.cache)
	}
}

func (gl *GLInstance) FetchMR(pid int, mrid int) (*GLMRData, error) {
	var parsedData GLMRData

	url := fmt.Sprintf("%s/v4/projects/%d/merge_requests/%d/changes", strings.TrimSuffix(gl.apiUrl, "/"), pid, mrid)
	body, err := gl.get(url)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(body, &parsedData)

	url = fmt.Sprintf("%s/v4/projects/%d/merge_requests/%d/discussions", strings.TrimSuffix(gl.apiUrl, "/"), pid, mrid)
	body, err = gl.get(url)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(body, &(parsedData.Discussions))

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

func (gl *GLInstance) CreateComment(comment GLNote, mr GLMRData) (GLDiscussion, error) {
	var discussion GLDiscussion

	form := url.Values{}
	form.Add("body", comment.Body)
	form.Add("position[position_type]", "text")
	form.Add("position[base_sha]", mr.DiffRefs.BaseSHA)
	form.Add("position[head_sha]", mr.DiffRefs.HeadSHA)
	form.Add("position[start_sha]", mr.DiffRefs.StartSHA)
	form.Add("position[old_path]", comment.Position.OldPath)
	form.Add("position[new_path]", comment.Position.NewPath)

	if comment.Position.NewLine > 0 {
		form.Add("position[new_line]", fmt.Sprintf("%d", comment.Position.NewLine))
	}

	if comment.Position.OldLine > 0 {
		form.Add("position[old_line]", fmt.Sprintf("%d", comment.Position.OldLine))
	}

	url := fmt.Sprintf("%s/v4/projects/%d/merge_requests/%d/discussions", strings.TrimSuffix(gl.apiUrl, "/"), mr.ProjectId, mr.Iid)

	body, err := gl.postForm(url, form)
	if err != nil {
		return discussion, err
	}

	err = json.Unmarshal(body, &discussion)
	if err != nil {
		return discussion, err
	}

	return discussion, nil
}
