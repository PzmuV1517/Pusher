package blobrel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/andreibanu/pusher/internal/ghauth"
)

var api = "https://api.github.com/repos/" + ghauth.Repo

// Variant is which build of the library to fetch.
type Variant string

// The library builds. Competition carries no recording code at all.
const (
	Competition Variant = "blob-competition"

	Dev Variant = "blob-dev"
)

// AssetName is what CI attaches to a release.
func AssetName(v Variant, tag string) string {
	return fmt.Sprintf("%s-%s.aar", v, tag)
}

// MainBranch is what an unlabelled tag belongs to.
const MainBranch = "main"

// Branch names the work a tag came out of.
//
// blob carries that inside the tag rather than anywhere else: a stable release
// is v1.7.0, and branch work is v1.8.0-feedforward.1, where semver already
// makes anything after the hyphen a pre-release. The label up to its first dot
// is the branch, and the rest counts the builds from it.
//
// Reading it out of the tag rather than asking GitHub which branch a release
// targets is deliberate on blob's side: a tag build knows its own tag and
// little else, and a name that travels inside the tag cannot come apart from
// the artifacts built for it. Branches are also deleted once merged, while the
// releases made from them stay.
func Branch(tag string) string {
	_, label, found := strings.Cut(tag, "-")
	if !found || label == "" {
		return MainBranch
	}

	if name, _, found := strings.Cut(label, "."); found {
		return name
	}
	return label
}

// Release is one published release of the library.
type Release struct {
	Tag string

	// Branch is the work it came from, MainBranch for a stable release.
	Branch string

	// Prerelease is what GitHub was told, which for blob means the same thing
	// as a labelled tag. Kept separately because it is GitHub's answer rather
	// than one read out of the name.
	Prerelease bool
}

// Releases lists what has been published, newest first.
//
// Drafts are left out: they have no assets to fetch and nobody outside the
// repository can see them anyway.
func Releases(token string) ([]Release, error) {
	var payload []struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := get(token, api+"/releases?per_page=100", &payload); err != nil {
		return nil, err
	}

	out := make([]Release, 0, len(payload))
	for _, r := range payload {
		if r.Tag == "" || r.Draft {
			continue
		}
		out = append(out, Release{
			Tag:        r.Tag,
			Branch:     Branch(r.Tag),
			Prerelease: r.Prerelease,
		})
	}
	return out, nil
}

// Branches lists the work that has releases, main first and the rest in the
// order they were last published.
func Branches(token string) ([]string, error) {
	releases, err := Releases(token)
	if err != nil {
		return nil, err
	}
	return branchesIn(releases), nil
}

func branchesIn(releases []Release) []string {
	seen := map[string]bool{}
	out := []string{}

	for _, r := range releases {
		if seen[r.Branch] {
			continue
		}
		seen[r.Branch] = true
		out = append(out, r.Branch)
	}

	// main leads even when its newest release is older than a branch's, because
	// it is the one somebody returns to.
	for i, name := range out {
		if name == MainBranch {
			copy(out[1:i+1], out[:i])
			out[0] = MainBranch
			break
		}
	}

	return out
}

// LatestOn is the newest release published from one branch.
func LatestOn(token, branch string) (string, error) {
	releases, err := Releases(token)
	if err != nil {
		return "", err
	}
	return latestOn(releases, branch)
}

func latestOn(releases []Release, branch string) (string, error) {
	if branch == "" {
		branch = MainBranch
	}

	for _, r := range releases {
		if r.Branch == branch {
			return r.Tag, nil
		}
	}

	if branch == MainBranch {
		return "", fmt.Errorf("blob has no published releases")
	}
	return "", fmt.Errorf("blob has no releases from %s", branch)
}

// LatestTag returns the newest release from main.
func LatestTag(token string) (string, error) {
	return LatestOn(token, MainBranch)
}

// Tags lists published release tags on one branch, newest first.
func Tags(token, branch string) ([]string, error) {
	releases, err := Releases(token)
	if err != nil {
		return nil, err
	}

	tags := make([]string, 0, len(releases))
	for _, r := range releases {
		if r.Branch == branch {
			tags = append(tags, r.Tag)
		}
	}
	return tags, nil
}

type asset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// Fetch downloads the AAR for a variant at a tag.
func Fetch(token string, v Variant, tag string) ([]byte, error) {
	var release struct {
		Assets []asset `json:"assets"`
	}
	if err := get(token, api+"/releases/tags/"+tag, &release); err != nil {
		return nil, err
	}

	want := AssetName(v, tag)
	for _, a := range release.Assets {
		if a.Name == want {
			return download(token, a.ID)
		}
	}

	return nil, fmt.Errorf("release %s has no %s\navailable: %s",
		tag, want, names(release.Assets))
}

func names(assets []asset) string {
	if len(assets) == 0 {
		return "nothing attached"
	}
	out := make([]string, 0, len(assets))
	for _, a := range assets {
		out = append(out, a.Name)
	}
	return strings.Join(out, ", ")
}

// The asset endpoint redirects to object storage, which rejects the request if
// GitHub's Authorization header is still attached. Go forwards it by default.
func download(token string, id int64) ([]byte, error) {
	client := &http.Client{
		Timeout: 5 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			req.Header.Del("Authorization")
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	url := fmt.Sprintf("%s/releases/assets/%d", api, id)
	req, err := ghauth.Request(http.MethodGet, url, token)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot download the library: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("download was cut short: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("download was empty")
	}
	if !isZip(data) {

		return nil, fmt.Errorf("what came back is not an AAR")
	}
	return data, nil
}

func isZip(data []byte) bool {
	return len(data) > 4 && data[0] == 'P' && data[1] == 'K' &&
		(data[2] == 3 || data[2] == 5 || data[2] == 7)
}

func get(token, url string, into any) error {
	req, err := ghauth.Request(http.MethodGet, url, token)
	if err != nil {
		return err
	}

	resp, err := ghauth.Client(15 * time.Second).Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach GitHub: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return fmt.Errorf("not found, or this token cannot see %s", ghauth.Repo)
	default:
		return fmt.Errorf("GitHub returned %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		return fmt.Errorf("cannot read GitHub response: %w", err)
	}
	return nil
}
