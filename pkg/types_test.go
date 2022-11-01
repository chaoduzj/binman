package binman

import (
	"fmt"
	"os"
	"runtime"
	"testing"
)

const mergeConfig = `
config:
  releasepath:  #path to keep fetched releases. $HOME/binMan is the default
  tokenvar: #environment variable that contains github token
  upx: #Compress binaries with upx
    enabled: false
    args: [] # arrary of args for upx
releases:
  - repo: anchore/syft
  - repo: anchore/grype
`

const dedupConfig = `
config:
  releasepath:  #path to keep fetched releases. $HOME/binMan is the default
  tokenvar: #environment variable that contains github token
  upx: #Compress binaries with upx
    enabled: false
    args: [] # arrary of args for upx
releases:
  - repo: rjbrown57/binman
  - repo: rjbrown57/binman
    releasefilename:  binman_darwin_amd64 
  - repo: rjbrown57/binman
`

const testConfig = `
config:
  releasepath: thereleasepath
  tokenvar: thetoken
  upx:
    enabled: true
    args: []
releases:
  - repo: rjbrown57/binman
`

const testConfigEmptyVals = `
config:
  releasepath: 
  tokenvar:
  upx:
    enabled: true
    args: []
releases:
  - repo: rjbrown57/binman
`

func TestDeduplicate(t *testing.T) {

	d, err := os.MkdirTemp(os.TempDir(), "binmantest")
	if err != nil {
		t.Fatalf("unable to make temp dir %s", d)
	}

	defer os.RemoveAll(d)

	configPath := fmt.Sprintf("%s/config", d)

	writeStringtoFile(configPath, dedupConfig)
	if err != nil {
		t.Fatalf("failed to write test config to %s", configPath)
	}

	c := newGHBMConfig(configPath)
	c.deDuplicate()

	if len(c.Releases) != 2 {
		t.Fatal("failed to dedeuplicate release array")
	}
}

func TestSetDefaults(t *testing.T) {
	d, err := os.MkdirTemp(os.TempDir(), "binmantest")
	if err != nil {
		t.Fatalf("unable to make temp dir %s", d)
	}

	defer os.RemoveAll(d)

	configPath := fmt.Sprintf("%s/config", d)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Unable to detected user home directory %s", err)
	}

	var tests = []struct {
		config              string
		expectedOs          string
		expectedArch        string
		expectedReleasePath string
		expectedTokenVar    string
	}{
		{testConfig, "linux", "amd64", "thereleasepath", "thetoken"},
		{testConfigEmptyVals, "linux", "amd64", homeDir + "/" + "binMan", "none"},
	}

	for _, test := range tests {
		writeStringtoFile(configPath, test.config)
		if err != nil {
			t.Fatalf("failed to write test config to %s", configPath)
		}
		c := newGHBMConfig(configPath)
		c.setDefaults()

		// test the defaults
		if c.Defaults.Arch != test.expectedArch || c.Defaults.Os != test.expectedOs {
			t.Fatalf("Expected %s,%s got %s,%s", c.Defaults.Arch, c.Defaults.Os, test.expectedArch, test.expectedOs)
		}

		if c.Config.TokenVar != test.expectedTokenVar {
			t.Fatalf("Expected %s got %s", test.expectedTokenVar, c.Config.TokenVar)
		}

		if c.Config.ReleasePath != test.expectedReleasePath {
			t.Fatalf("Expected %s got %s", test.expectedReleasePath, c.Config.ReleasePath)
		}
	}
}

const testConfigPopulateTest = `
config:
  releasepath: "/tmp/"
  tokenvar:
  upx:
    enabled: true
    args: []
releases:
  - repo: rjbrown57/binman
  - repo: rjbrown57/binextractor 
    upx:
      enabled: true
      args: ["-k", "-v"]
  - repo: rjbrown57/lp
    upx:
      enabled: false
`

func TestPopulateReleases(t *testing.T) {

	d, err := os.MkdirTemp(os.TempDir(), "binmantest")
	if err != nil {
		t.Fatalf("unable to make temp dir %s", d)
	}

	defer os.RemoveAll(d)

	configPath := fmt.Sprintf("%s/config", d)

	writeStringtoFile(configPath, testConfigPopulateTest)
	if err != nil {
		t.Fatalf("failed to write test config to %s", configPath)
	}

	testUpxConfigTrue := UpxConfig{
		Enabled: "true",
	}

	testUpxConfigFalse := UpxConfig{
		Enabled: "false",
	}

	// Releases are marshalled in the reverse of the order set in the config. So we reverse the config order here.
	testRelSlice := []BinmanRelease{
		{
			Repo:         "rjbrown57/binman",
			Org:          "rjbrown57",
			Project:      "binman",
			Os:           "linux",
			Arch:         "amd64",
			CheckSum:     false,
			DownloadOnly: false,
			UpxConfig:    testUpxConfigTrue,
		},
		{
			Repo:         "rjbrown57/binextractor",
			Org:          "rjbrown57",
			Project:      "extractor",
			Os:           "linux",
			Arch:         "amd64",
			CheckSum:     false,
			DownloadOnly: false,
			UpxConfig: UpxConfig{
				Enabled: "true",
				Args:    []string{"-k", "-v"},
			},
		},
		{
			Repo:         "rjbrown57/lp",
			Org:          "rjbrown57",
			Project:      "lp",
			Os:           "linux",
			Arch:         "amd64",
			CheckSum:     false,
			DownloadOnly: false,
			UpxConfig:    testUpxConfigFalse,
		},
	}

	got := newGHBMConfig(configPath)

	expected := &GHBMConfig{
		Config: BinmanConfig{

			ReleasePath: "/tmp/",
			TokenVar:    "none",
			UpxConfig:   testUpxConfigTrue,
		},
		Releases: testRelSlice,
		Defaults: BinmanDefaults{
			Os:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
	}

	got.setDefaults()
	got.populateReleases()

	for k := range got.Releases {

		if got.Releases[k].Repo != expected.Releases[k].Repo {
			t.Fatalf("\n Got %+v != \n Expected %+v", got.Releases[k], expected.Releases[k])
		}

		if got.Releases[k].Arch != expected.Releases[k].Arch {
			t.Fatalf("\n Got %+v != \n Expected %+v", got.Releases[k], expected.Releases[k])
		}

		if got.Releases[k].Os != expected.Releases[k].Os {
			t.Fatalf("\n Got %+v != \n Expected %+v", got.Releases[k], expected.Releases[k])
		}

		if got.Releases[k].UpxConfig.Enabled != expected.Releases[k].UpxConfig.Enabled {
			t.Fatalf("\n Got %+v != \n Expected %+v", got.Releases[k], expected.Releases[k])
		}

		if len(got.Releases[k].UpxConfig.Args) != len(expected.Releases[k].UpxConfig.Args) {
			t.Fatalf("\n Got %+v != \n Expected %+v", got.Releases[k], expected.Releases[k])
		}

	}
}
