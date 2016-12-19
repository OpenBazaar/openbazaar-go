package test

import (
	"net/http"
	"os"
	"path"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

// NewAPIConfig returns a new config object for the API tests
func NewAPIConfig() (*repo.APIConfig, error) {
	apiConfig, err := repo.GetAPIConfig(path.Join(GetRepoPath(), "config"))
	if err != nil {
		return nil, err
	}
	apiConfig.Authenticated = true
	apiConfig.Username = "test"
	apiConfig.Password = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08" // sha256("test")
	corsOrigin := "example.com"
	apiConfig.CORS = &corsOrigin
	apiConfig.HTTPHeaders = map[string][]string{
		"X-Foo": []string{"Trade Free or Die"},
	}

	return apiConfig, nil
}

// GetRepoPath returns the repo path to use for tests
// It should be considered volitile and may be destroyed at any time
func GetRepoPath() string {
	return getEnvString("OPENBAZAAR_TEST_REPO_PATH", "/tmp/openbazaar-test")
}

// GetPassword returns a static mneumonic to use
func GetPassword() string {
	return getEnvString("OPENBAZAAR_TEST_PASSWORD", "correct horse battery staple")
}

// GetAuthCookie returns a pointer to a test authentication cookie
func GetAuthCookie() *http.Cookie {
	return &http.Cookie{
		Name:  "OpenBazaar_Auth_Cookie",
		Value: "supersecret",
	}
}

func getEnvString(key string, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}
