package delta

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
)

var (
	md5Validator = regexp.MustCompile("(?i)^[a-f0-9]{32}$")
)

// LastSubmissionLicense retrieves and stores last inventory successful submission license key hash.
type LastSubmissionLicense interface {
	HasChanged(string) (bool, error)
	Update(string) error
}

// LastSubmissionLicenseFileStore persists last successful submission license key hash.
type LastSubmissionLicenseFileStore struct {
	readFile  func(path string) (string, error)
	writeFile func(content string, path string) error
	// file holds the file path storing the data
	filePath   string
	licenseMD5 string
}

// NewLastSubmissionLicenseFileStore creates a new LastSubmissionLicenseFileStore storing data in file.
func NewLastSubmissionLicenseFileStore(dataDir, fileName string) LastSubmissionLicense {
	return &LastSubmissionLicenseFileStore{
		readFile:  readLicenseHashFromFileFn,
		writeFile: writeLicenseHashToFileFn,
		filePath:  filepath.Join(dataDir, lastLicenseHashFolder, helpers.SanitizeFileName(fileName)),
	}
}

// NewLastSubmissionLicenseInMemory will store the license hash only in memory.
func NewLastSubmissionLicenseInMemory() LastSubmissionLicense {
	return &LastSubmissionLicenseFileStore{
		readFile: func(path string) (string, error) {
			return "", nil
		},
		writeFile: func(content string, path string) error {
			return nil
		},
	}
}

// load will get the license hash from the disk.
func (l *LastSubmissionLicenseFileStore) load() error {
	var err error
	if l.licenseMD5 == "" {
		l.licenseMD5, err = l.readFile(l.filePath)
	}

	return err
}

var pslog = log.WithComponent("PatchSender")

// UpdateEntityID will store the entityID on memory and disk.
func (l *LastSubmissionLicenseFileStore) Update(license string) error {
	l.licenseMD5 = l.md5(license)
	return l.writeFile(l.licenseMD5, l.filePath)
}

// HasChanged returns whether the License had been changed.
func (l *LastSubmissionLicenseFileStore) HasChanged(license string) (bool, error) {
	err := l.load()
	// If license file not found.
	if err == nil && l.licenseMD5 == "" {
		err = l.Update(license)
		return false, err
	}

	return l.licenseMD5 != l.md5(license), err
}

func (l *LastSubmissionLicenseFileStore) md5(license string) string {
	hash := md5.New()
	hash.Write([]byte(license))
	return hex.EncodeToString(hash.Sum(nil))
}

func readLicenseHashFromFileFn(filePath string) (string, error) {
	// Check if there is an already stored value on disk.
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", nil
	}

	buf, err := ioutil.ReadFile(filePath)

	if err != nil {
		return "", fmt.Errorf("cannot read file persisted license hash, file: '%s', error: %v", filePath, err)
	}

	match := md5Validator.Match(buf)
	if !match {
		return "", fmt.Errorf("expected licence has to be an MD5 format, content: %s", buf)
	}
	return string(buf), nil
}

func writeLicenseHashToFileFn(content string, filePath string) error {
	dir := filepath.Dir(filePath)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if mkDirErr := os.MkdirAll(dir, DATA_DIR_MODE); mkDirErr != nil {
			return fmt.Errorf("cannot persist license hash, agent data directory: '%s' does not exist and cannot be created: %v",
				dir, mkDirErr)
		}
	}

	return ioutil.WriteFile(filePath, []byte(content), DATA_FILE_MODE)
}
