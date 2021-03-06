package builder

import (
	"encoding/gob"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	fp "path/filepath"
)

var (
	ErrTooMany         = errors.New("too many revisions in that work item")
	ErrUnknownWorkType = errors.New("unknown work type")
)

func init() {
	gob.Register(build{})
}

var exeSuffix = func() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}()

type WorkType int

const (
	WorkTypePackage WorkType = iota
	WorkTypeWorkspace
	WorkTypeGoinstall
)

//Work represents an item of work to be completed by the builder
type Work interface {
	Revisions() (rev []string)
	VCS() (v VCS)
	RepoPath() (path string)
	ImportPath() (path string)
	WorkType() (t WorkType)
}

type Build interface {
	Error() error
	Paths() []string
	Revision() string
	Cleanup() error
}

type build struct {
	Ps   []string
	base string
	Err  error
	Rev  string
}

func (b build) Revision() string {
	return b.Rev
}

func (b build) Error() error {
	return b.Err
}

func (b build) Paths() []string {
	return b.Ps
}

func (b build) Cleanup() (err error) {
	if b.base != "" {
		err = os.RemoveAll(b.base)
	}
	return
}

func (bui *build) appendPath(pack string) {
	//what the go tool does from inspecting the source
	_, elem := path.Split(pack)
	name := elem + ".test" + exeSuffix
	path := fp.Join(bui.base, name)

	//make sure that binary exists before we add it to the paths. it may
	//not exist if there are no test files, so only add it if something
	//is there.
	if _, err := os.Stat(path); err == nil {
		bui.Ps = append(bui.Ps, path)
	}
}

var _ Build = build{}

type environ struct {
	gopath  string
	srcDir  string
	tmpRepo string
	vcs     VCS
}

func (e environ) Cleanup() {
	if e.gopath != "" {
		os.RemoveAll(e.gopath)
	}
	if e.tmpRepo != "" {
		os.RemoveAll(e.tmpRepo)
	}
}

func (e environ) CleanGopath() {
	//clean bin/pkg directories from the gopath
	if e.gopath != "" {
		os.RemoveAll(fp.Join(e.gopath, "pkg"))
		os.RemoveAll(fp.Join(e.gopath, "bin"))
		os.RemoveAll(fp.Join(e.gopath, "src"))
	}
}

func newEnviron(w Work) (e environ, err error) {
	e.vcs = w.VCS()

	e.gopath, err = ioutil.TempDir("", "gopath")
	if err != nil {
		return
	}

	e.tmpRepo, err = ioutil.TempDir("", "tmpRepo")
	if err != nil {
		return
	}

	switch w.WorkType() {
	case WorkTypePackage:
		e.srcDir = fp.Join(e.gopath, "src", w.ImportPath())
	case WorkTypeWorkspace:
		e.srcDir = e.gopath
	default:
		err = ErrUnknownWorkType
	}

	return
}

func CreateBuilds(w Work) (items []Build, err error) {
	//check if we have a goget thing here
	if w.WorkType() == WorkTypeGoinstall {
		var item build
		item, err = createGoinstallBuild(w)
		if err == nil {
			items = append(items, item)
		}
		return
	}

	if len(w.Revisions()) > 5 {
		err = ErrTooMany
		return
	}

	//create a new environment for the work
	e, err := newEnviron(w)
	defer e.Cleanup()
	if err != nil {
		return
	}

	//grab the build items
	items, err = createBuilds(w, e)
	return
}

func createBuilds(w Work, e environ) (res []Build, err error) {
	//clone the repo to a temporary location for checkout/copying
	err = e.vcs.Clone(w.RepoPath(), e.tmpRepo)
	if err != nil {
		return
	}

	for _, rev := range w.Revisions() {
		//create the build binaries for this revision
		bui := createBuild(rev, e)

		//if we didn't create any binaries, don't keep the dump directory around
		if len(bui.Ps) == 0 {
			bui.Cleanup()
			bui.base = ""
		}

		//add the build into our result list
		res = append(res, bui)

		//clean up the gopath from the last build
		e.CleanGopath()
	}

	return
}

func createBuild(rev string, e environ) (bui build) {
	var packs, testpacks []string
	bui.Rev = rev

	//make a new directory for the builds of this revision
	bui.base, bui.Err = ioutil.TempDir("", rev)
	if bui.Err != nil {
		return
	}

	//checkout the revision we need
	bui.Err = e.vcs.Checkout(e.tmpRepo, rev)
	if bui.Err != nil {
		return
	}

	//copy the repo to the srcDir
	bui.Err = copy(e.tmpRepo+string(fp.Separator)+".", e.srcDir)
	if bui.Err != nil {
		return
	}

	//figure out what packages need to be built
	packs, testpacks, bui.Err = list(e.gopath)
	if bui.Err != nil {
		return
	}

	merged := make([]string, 0, len(packs)+len(testpacks))
	merged = append(merged, packs...)
	merged = append(merged, testpacks...)

	//run a get to build deps
	bui.Err = get(e.gopath, false, merged...)
	if bui.Err != nil {
		return
	}

	//build the binaries and move them to a temporary directory
	for _, pack := range packs {
		bui.Err = testbuild(e.gopath, pack, bui.base)
		if bui.Err != nil {
			return
		}

		bui.appendPath(pack)
	}

	return
}

func createGoinstallBuild(w Work) (bui build, err error) {
	bui.Rev = "Latest"

	pack := w.ImportPath()

	//make a new directory for the build
	bui.base, bui.Err = ioutil.TempDir("", hash(pack))
	if bui.Err != nil {
		return
	}

	bui.Err = get(GOPATH, true, pack)
	if bui.Err != nil {
		return
	}

	//find all the deps for the tests and build those
	var testpacks []string
	_, testpacks, bui.Err = listPackage(GOPATH, pack)
	if bui.Err != nil {
		return
	}

	bui.Err = get(GOPATH, true, testpacks...)
	if bui.Err != nil {
		return
	}

	bui.Err = testbuild(GOPATH, pack, bui.base)
	if bui.Err != nil {
		return
	}

	bui.appendPath(pack)
	return
}
