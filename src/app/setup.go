package main

import (
	"builder"
	"log"
	"setup"
	"sync"
)

//run the setup to run the work
func run_setup() {
	log.Println("running setup...")
	//ensure we have the go tool and vcs in parallel
	var group sync.WaitGroup
	group.Add(2)

	//check the go tool
	go func() {
		if err := setup.EnsureTool(); err != nil {
			log.Fatal(err)
		}
		log.Println("tooling complete")
		group.Done()
	}()

	//check for hg + bzr
	go func() {
		if err := setup.EnsureVCS(); err != nil {
			log.Fatal(err)
		}
		log.Println("vcs complete")
		group.Done()
	}()

	group.Wait()

	//setup the builder to know where GOROOT is set
	builder.GOROOT = setup.GOROOT

	log.Println("setup complete. running queue")
	go runQueue()
}