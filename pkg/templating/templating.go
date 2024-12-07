package templating

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/go-sprout/sprout"
	"github.com/go-sprout/sprout/group/all"

	log "github.com/rjbrown57/binman/pkg/logging"
)

// Format strings for processing. Currently used by releaseFileName and DlUrl
func TemplateString(templateString string, dataMap map[string]interface{}) string {

	handler := sprout.New()
	handler.AddGroups(all.RegistryGroup())

	// For compatability with previous binman versions update %s to {{.}}
	templateString = strings.Replace(templateString, "%s", "{{.version}}", -1)

	// we need an io.Writer to capture the template output
	buf := new(bytes.Buffer)

	// https://github.com/Masterminds/sprout use sprout functions for extra templating functions
	tmpl, err := template.New("stringFormatter").Funcs(handler.Build()).Parse(templateString)
	if err != nil {
		log.Fatalf("unable to process template for %s", templateString)
	}

	err = tmpl.Execute(buf, dataMap)
	if err != nil {
		log.Fatalf("unable to process template for %s", templateString)
	}

	return buf.String()
}
