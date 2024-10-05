package binman

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"

	db "github.com/rjbrown57/binman/pkg/db"
	"github.com/rjbrown57/binman/pkg/gh"
	"github.com/rjbrown57/binman/pkg/gl"
	log "github.com/rjbrown57/binman/pkg/logging"
)

/*
All binman work is done by implementations of the Action interface. Work is ordered depending on user request and then executed sequentially.
The work is divided into several stages
get
  * Collect data to act on. Currently this is only github releases.
pre
  * Preperation and validation there is actually work to do
post
  * steps to perform after an asset has been downloaded
*/

type Action interface {
	execute() error
}

func (r *BinmanRelease) runActions() error {

	var err error

	for _, task := range r.actions {
		log.Debugf("Executing %s for %s", reflect.TypeOf(task), r.Repo)
		err = task.execute()
		if err != nil {
			switch err.(type) {
			case *ExcludeError:
				log.Debugf("%s is excluded for OS (%s), skipping", r.Repo, runtime.GOOS)
				return err
			case *NoUpdateError:
				log.Debugf("%s(%s) is up to date", r.Repo, r.Version)
				return err
			default:
				log.Debugf("Unable to complete action %s : %v", reflect.TypeOf(task), err)
				return err
			}
		}
	}

	return nil
}

// SetPreActions handles query and asset Selection
func (r *BinmanRelease) setPreActions(releasePath string, binPath string) []Action {

	var actions []Action

	actions = append(actions, r.AddReleaseExcludeAction())

	switch r.source.Apitype {
	case "gitlab":
		glClient := gl.GetGLClient(r.source.URL, r.source.Tokenvar)
		actions = append(actions, r.AddGetGLReleaseAction(glClient))
	case "github":
		ghClient := gh.GetGHCLient(r.source.URL, r.source.Tokenvar)
		// TODO checking limits over and over is not optimal
		gh.ShowLimits(ghClient)
		if err := gh.CheckLimits(ghClient); err != nil {
			log.Fatalf("Unable to check limits against GH api")
		}

		actions = append(actions, r.AddGetGHReleaseAction(ghClient))
	case "binman":
		actions = append(actions, r.AddGetBinmanReleaseAction())
	}

	// If we have a nil DbChan + downloadChan then we will only populate
	// then we are likely being used as a library to get data from GH/GL.
	// So we populate []BimanRelease array and end early
	if r.dbChan == nil && r.downloadChan == nil {
		return append(actions, r.AddEndWorkAction())
	}

	// If publishPath is already set we are doing a direct repo download and don't need to set a release path
	// Direct repo actions should be moved to their own command
	if r.PublishPath == "" {
		actions = append(actions, r.AddReleaseStatusAction(releasePath))
	}

	// If watchSync is false then skip all remaining steps
	if !r.watchSync && r.watchExposeMetrics {
		return append(actions, r.AddEndWorkAction())
	}

	// If PostOnly is true, we don't need to select an asset
	if !r.PostOnly {
		actions = append(actions,
			// The SetUrlAction finds the approriate asset to download
			r.AddSetUrlAction(),
		)
	}

	// Add remaining preDownload actions
	actions = append(actions,
		r.AddSetArtifactPathAction(releasePath, binPath),
		r.AddSetPostActions(),
	)

	log.Debugf("Performing %s pre actions for %s", r.displayActions(&actions), r.Repo)

	return actions

}

type SetPostActions struct {
	r *BinmanRelease
}

func (r *BinmanRelease) AddSetPostActions() Action {
	return &SetPostActions{
		r,
	}
}

func (action *SetPostActions) execute() error {
	action.r.actions = action.r.setPostActions()
	return nil
}

// getPostActions will arrange all final work after we have selected an asset
func (r *BinmanRelease) setPostActions() []Action {

	var actions []Action

	if !r.PostOnly {
		actions = append(actions, r.AddDownloadAction())

		// If we are set to download only stop all postCommands
		if r.DownloadOnly {
			actions = append(actions, r.AddSetOsActions())
			return actions
		}

		// If we are not set to download only, set the rest of the post processing actions
		switch findfType(r.filepath) {
		case "tar":
			actions = append(actions, r.AddExtractAction())
			if r.CleanupArchive {
				actions = append(actions, r.AddCleanArchive())
			}
		case "zip":
			actions = append(actions, r.AddExtractAction())
			if r.CleanupArchive {
				actions = append(actions, r.AddCleanArchive())
			}
		case "default":
		}

		actions = append(actions, r.AddFindTargetAction(),
			r.AddMakeExecuteableAction(),
			r.AddWriteRelNotesAction())
	}

	actions = append(actions, r.AddSetOsActions())

	log.Debugf("Performing %s Post actions for %s", r.displayActions(&actions), r.Repo)

	return actions

}

type SetOsActions struct {
	r *BinmanRelease
}

func (r *BinmanRelease) AddSetOsActions() Action {
	return &SetOsActions{
		r,
	}
}

func (action *SetOsActions) execute() error {
	action.r.actions = action.r.setOsCommands()
	return nil
}

func (r *BinmanRelease) setOsCommands() []Action {

	var actions []Action

	if r.UpxConfig.Enabled == "true" {
		// Merge any user args with upx
		args := []string{r.ArtifactPath}
		args = append(args, r.UpxConfig.Args...)

		UpxCommand := PostCommand{
			Command: "upx",
			Args:    args,
		}

		r.PostCommands = append([]PostCommand{UpxCommand}, r.PostCommands...)
	}

	// Add post commands defined by user if specified
	for index := range r.PostCommands {
		actions = append(actions, r.AddOsCommandAction(index))
	}

	actions = append(actions, r.AddSetFinalActions())

	log.Debugf("Performing %s OS commands for %s", r.displayActions(&actions), r.Repo)
	return actions
}

type SetFinalActions struct {
	r *BinmanRelease
}

func (r *BinmanRelease) AddSetFinalActions() Action {
	return &SetFinalActions{
		r,
	}
}

func (action *SetFinalActions) execute() error {
	action.r.actions = action.r.setFinalActions()
	return nil
}

// setFinalActions assuming that all previous post and OS related actions have been successful perform final actions
// (like linking the binary to the new release)
func (r *BinmanRelease) setFinalActions() []Action {

	// If PostOnly or DownloadOnly we only need EndWorkAction
	if r.PostOnly || r.DownloadOnly {
		return []Action{r.AddEndWorkAction()}
	}

	return []Action{r.AddLinkFileAction(), r.AddUpdateDbAction(), r.AddEndWorkAction()}
}

type UpdateDbAction struct {
	r *BinmanRelease
}

func (r *BinmanRelease) AddUpdateDbAction() Action {
	return &UpdateDbAction{
		r,
	}
}

func (action *UpdateDbAction) execute() error {

	action.r.dwg.Add(1)

	var rwg sync.WaitGroup

	dbMsg := db.DbMsg{
		Operation:  "write",
		Key:        fmt.Sprintf("%s/%s/%s/data", action.r.SourceIdentifier, action.r.Repo, action.r.Version),
		ReturnChan: make(chan db.DBResponse, 1),
		ReturnWg:   &rwg,
		Data:       dataToBytes(action.r.getDataMap()),
	}

	m := dbMsg.Send(action.r.dbChan)
	return m.Err
}

type EndWorkAction struct {
	r *BinmanRelease
}

func (r *BinmanRelease) AddEndWorkAction() Action {
	return &EndWorkAction{
		r,
	}
}

func (action *EndWorkAction) execute() error {
	action.r.actions = nil
	return nil
}
