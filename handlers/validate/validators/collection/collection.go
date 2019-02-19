package collection

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/bitrise-io/go-utils/pathutil"

	"github.com/bitrise-io/bitrise-steplib/handlers/validate/steplib"
	"github.com/bitrise-io/go-utils/command"
)

const testBitriseYML = `format_version: "4"
default_step_lib_source: file://./

workflows:
  test:
    steps:
    - script:
        inputs:
        - content: echo "successful"
`

// Validator is matching for the validator interface
type Validator struct{}

// IsSkippable sets the validation task to skip failures
func (v *Validator) IsSkippable() bool {
	return false
}

func getTestableCLIVersionDownloadURLs() ([]string, error) {
	resp, err := http.Get("https://api.github.com/repos/bitrise-io/bitrise/releases?per_page=20")
	if err != nil {
		return nil, err
	}

	var releases []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	if err := resp.Body.Close(); err != nil {
		return nil, err
	}

	os := "Darwin"
	arch := "x86_64"

	if runtime.GOOS != "darwin" {
		os = "Linux"
	}

	var releaseTags []string
	for _, release := range releases {
		releaseTags = append(releaseTags, fmt.Sprintf("https://github.com/bitrise-io/bitrise/releases/download/%s/bitrise-%s-%s", release.TagName, os, arch))
	}

	return releaseTags, nil
}

func setupBinary(url string) (string, error) {
	tmpPath, err := pathutil.NormalizedOSTempDirPath("cli_version_test")
	if err != nil {
		return "", err
	}

	tmpBinaryPath := filepath.Join(tmpPath, "bitrise")

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	binaryFile, err := os.Create(tmpBinaryPath)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(binaryFile, resp.Body); err != nil {
		return "", err
	}

	if err := binaryFile.Close(); err != nil {
		return "", err
	}

	if err := resp.Body.Close(); err != nil {
		return "", err
	}

	if err := os.Chmod(tmpBinaryPath, 0777); err != nil {
		return "", err
	}

	return tmpBinaryPath, nil
}

// Validate is the logic handler of the task
func (v *Validator) Validate(sl steplib.StepLib) error {
	cliVersionURLs, err := getTestableCLIVersionDownloadURLs()
	if err != nil {
		return err
	}

	for _, cliVersionURL := range cliVersionURLs {
		fmt.Println(" - downloading and running:", cliVersionURL)

		tmpBinaryPath, err := setupBinary(cliVersionURL)
		if err != nil {
			return err
		}

		out, err := command.New(tmpBinaryPath, "run", "--config-base64", base64.StdEncoding.EncodeToString([]byte(testBitriseYML)), "test").RunAndReturnTrimmedCombinedOutput()
		if err != nil {
			return fmt.Errorf(" - CLI run failed, output:\n%s\n\nerror: %s", out, err)
		}
	}
	return nil
}

// String will return a short summary of the validator task
func (v *Validator) String() string {
	return "Ensure CLI is working properly with the new collection"
}
