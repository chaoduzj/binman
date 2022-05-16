package binman

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v2"
)

// GHBMConfigfile contains Global Config Options
type GHBMConfigFile struct {
	ReleasePath string `yaml:"releasepath"`        //path to download/link releases from github
	TokenVar    string `yaml:"tokenvar,omitempty"` //Github Auth Token
}

// GHBMDefaults contains default config options. If a value is unset in releases array these will be used.
type GHBMDefaults struct {
	Os       string `yaml:"os,omitempty"`       //OS architechrue to look for
	Arch     string `yaml:"arch,omitempty"`     //OS architechrue to look for
	CheckSum bool   `yaml:"checksum,omitempty"` // Not used not, in theory validate checksums
	FileType string `yaml:"filetype,omitempty"` // Filetype to find assets by. Typically set to tar.gz
	Version  string `yaml:"version,omitempty"`  // Stub Version to look for
}

// GHBMRelease contains info on specifc releases to hunt for
type GHBMRelease struct {
	Os              string `yaml:"os,omitempty"`
	Arch            string `yaml:"arch,omitempty"`
	CheckSum        bool   `yaml:"checkSum,omitempty"`
	FileName        string `yaml:"filename,omitempty"` // The file within the release you want
	FileType        string `yaml:"filetype,omitempty"`
	ReleaseFileName string `yaml:"releasefilename,omitempty"` // Specifc Release filename to look for. This is useful if a project publishes a binary and not a tarball.
	Repo            string `yaml:"repo"`                      // The specific repo name in github. e.g achore/syft
	Org             string // Will be provided by constuctor
	Project         string // Will be provided by constuctor
	PublishPath     string // Path Release will be set up at
	ArtifactPath    string // Will be set by GHBMRelease.setPaths
	LinkPath        string // Will be set by GHBMRelease.setPaths
	Version         string `yaml:"version,omitempty"` // Stub
}

// Helper method to set paths for a requested release object
func (r *GHBMRelease) setPaths(ReleasePath string, tag string) {

	// Trim trailing / if user provided
	if strings.HasSuffix(ReleasePath, "/") {
		ReleasePath = strings.TrimSuffix(ReleasePath, "/")
	}
	r.PublishPath = fmt.Sprintf("%s/repos/%s/%s/%s", ReleasePath, r.Org, r.Project, tag)

	// If a binary is specified by ReleaseFileName use it for source and project for destination
	// else if it's a tar but we have specified the inside file use filename for source and destination
	// else it's a tar and we want default
	if r.ReleaseFileName != "" {
		r.ArtifactPath = fmt.Sprintf("%s/%s", r.PublishPath, r.ReleaseFileName)
		r.LinkPath = fmt.Sprintf("%s/%s", ReleasePath, r.Project)
		log.Debugf("ReleaseFilenName set %s->%s\n", r.ArtifactPath, r.LinkPath)
	} else if r.FileName != "" {
		r.ArtifactPath = fmt.Sprintf("%s/%s", r.PublishPath, r.FileName)
		r.LinkPath = fmt.Sprintf("%s/%s", ReleasePath, filepath.Base(r.FileName))
		log.Debugf("Tar with Filename set %s -> %s\n", r.ArtifactPath, r.LinkPath)
	} else {
		r.ArtifactPath = fmt.Sprintf("%s/%s", r.PublishPath, r.Project)
		r.LinkPath = fmt.Sprintf("%s/%s", ReleasePath, r.Project)
		log.Debugf("Default Extraction %s->%s\n", r.ArtifactPath, r.LinkPath)
	}

}

// Type that rolls up the above types into one happy family
type GHBMConfig struct {
	Config   GHBMConfigFile `yaml:"config"`
	Defaults GHBMDefaults   `yaml:"defaults"`
	Releases []GHBMRelease  `yaml:"releases"`
}

func newGHBMConfig(configPath string) *GHBMConfig {
	config := &GHBMConfig{}
	mustUnmarshalYaml(configPath, config)
	config.setDefaults()
	return config
}

// setReleaseDefaults will populate defaults, and required values
func (config *GHBMConfig) setDefaults() {

	if config.Config.TokenVar == "" {
		log.Warn("tokenvar is not set at config.tokenvar using anonymous authentication. Please be aware you can quickly be rate limited by github. Instructions here https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token")
		config.Config.TokenVar = "none"
	}

	log.Debugf("OS = %s Arch = %s", runtime.GOOS, runtime.GOARCH)

	for k, _ := range config.Releases {

		config.Releases[k].Org, config.Releases[k].Project = getOR(config.Releases[k].Repo)

		if config.Releases[k].Os == "" {
			config.Releases[k].Os = strings.ToLower(runtime.GOOS)
		}

		if config.Releases[k].Arch == "" {
			config.Releases[k].Arch = strings.ToLower(runtime.GOARCH)
		}

		if config.Releases[k].FileType == "" {
			config.Releases[k].FileType = config.Defaults.FileType
		}

	}
}

// Add an in default values for most fields :)
func mustUnmarshalYaml(configPath string, v interface{}) {
	yamlFile, err := ioutil.ReadFile(filepath.Clean(configPath))
	if err != nil {
		log.Fatalf("err opening %s   #%v\n", configPath, err)
		os.Exit(1)
	}
	err = yaml.Unmarshal(yamlFile, v)
	if err != nil {
		log.Fatalf("unmarhsal error   #%v\n", err)
		os.Exit(1)
	}
}
