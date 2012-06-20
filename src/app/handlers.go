package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"worker"
)

//our basic handle index that demonstrates how to get data from the context
//inside a template
func handle_index(w http.ResponseWriter, req *http.Request, ctx *Context) {
	if req.URL.Path != "/" {
		perform_status(w, ctx, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-type", "text/html")
	ws, err := worker.GetRecentWork(ctx.Context, 10)
	if err != nil {
		internal_error(w, req, ctx, err)
		return
	}
	ctx.Set("Recent", ws)
	base_execute(w, ctx, tmpl_root("blocks", "index.block"))
}

func handle_build_info(w http.ResponseWriter, req *http.Request, ctx *Context) {
	id := req.FormValue(":id")
	if id == "" {
		perform_status(w, ctx, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	wk, err := worker.GetWorkFromBuild(ctx.Context, id)
	if err != nil {
		internal_error(w, req, ctx, err)
		return
	}
	var bd *worker.Build
	for _, b := range wk.Builds {
		if b.ID == id {
			bd = b
			break
		}
	}
	if bd == nil {
		internal_error(w, req, ctx, fmt.Errorf("%s: queryed but not found", id))
		return
	}
	ctx.Set("Build", bd)
	base_execute(w, ctx, tmpl_root("blocks", "build.block"))
}

func handle_how(w http.ResponseWriter, req *http.Request, ctx *Context) {
	w.Header().Set("Content-type", "text/html")
	ctx.Meta.SubNav = navList{
		&navBase{"Info", "#info", nil},
		&navBase{"Github", "#github", nil},
		&navBase{"Bitbucket", "#bitbucket", nil},
		&navBase{"Google Code", "#google", nil},
		&navBase{"General", "#general", nil},
	}
	base_execute(w, ctx, tmpl_root("blocks", "how.block"))
}

func send_json(w http.ResponseWriter, val interface{}) (err error) {
	w.Header().Set("Content-type", "application/json")
	enc := json.NewEncoder(w)
	err = enc.Encode(val)
	return
}

func handle_recent_json(w http.ResponseWriter, req *http.Request, ctx *Context) {
	ws, err := worker.GetRecentWork(ctx.Context, 10)
	if err != nil {
		internal_error(w, req, ctx, err)
		return
	}
	if err := send_json(w, ws); err != nil {
		log.Println("error responding with json:", err)
	}
}

func handle_work_json(w http.ResponseWriter, req *http.Request, ctx *Context) {
	mw, err := worker.CurrentWork(ctx.Context)
	if err != nil {
		internal_error(w, req, ctx, err)
		return
	}
	if err := send_json(w, mw); err != nil {
		log.Println("error responding with json:", err)
	}
}
