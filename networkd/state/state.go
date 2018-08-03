// Package state contains a thread safe state struct that stores information
// about the networkd
package state

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"path"
	"strings"
	"sync"

	"github.com/gladiusio/gladius-utils/config"
)

// New returns a new state struct
func New(version string) *State {
	state := &State{running: true, content: make(map[string]([2](map[string][]byte))), runChannel: make(chan bool), version: version}
	state.LoadContentFromDisk()
	state.
	return state
}

// State is a thread safe struct for keeping information about the networkd
type State struct {
	running    bool
	content    map[string]([2](map[string][]byte))
	runChannel chan (bool)
	version    string
	mux        sync.Mutex
}

type status struct {
	Running bool
	Version string
}

// Content gets the current content in ram
func (s *State) GetPage(website, route string) []byte {
	s.mux.Lock()
	// Lock so only one goroutine at a time can access the map
	defer s.mux.Unlock()
	return s.content[website][0][route]
}

func (s *State) GetAsset(website, asset string) []byte {
	s.mux.Lock()
	// Lock so only one goroutine at a time can access the map
	defer s.mux.Unlock()
	return s.content[website][1][asset]
}

// SetContentRunState updates the the desired state of the networking
func (s *State) SetContentRunState(runState bool) {
	s.mux.Lock()
	if s.running != runState {
		s.running = runState
		go func() { s.runChannel <- runState }()
	}
	s.mux.Unlock()
}

// RunningStateChanged returns a channel that updates when the running state is
// changed
func (s *State) RunningStateChanged() chan (bool) {
	return s.runChannel
}

func (s *State) Info() string {
	s.mux.Lock()
	defer s.mux.Unlock()

	status := &status{Running: s.running, Version: s.version}

	jsonString, _ := json.Marshal(status)
	return string(jsonString)
}

// ShouldBeRunning returns the current desired run state of the networking
func (s *State) ShouldBeRunning() bool {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.running
}

// LoadContentFromDisk loads the content from the disk and stores it in the state
func (s *State) LoadContentFromDisk() {
	filePath, err := getContentDir()
	if err != nil {
		panic(err)
	}

	files, err := ioutil.ReadDir(filePath)
	if err != nil {
		log.Fatal("Error when reading content dir: ", err)
	}
	// map websites
	m := make(map[string]([2](map[string][]byte)))

	for _, f := range files {
		website := f.Name()
		if f.IsDir() { /* content */ /* assets */
			m[website] = [2]map[string][]byte{make(map[string][]byte), make(map[string][]byte)}

			contentFiles, err := ioutil.ReadDir(path.Join(filePath, website))
			if err != nil {
				log.Fatal("Error when reading content dir: ", err)
			}
			log.Print("Loading website: " + website)
			for _, contentFile := range contentFiles {
				// HTML for the page
				if !contentFile.IsDir() {
					// Replace "%2f" with "/"
					replacer := strings.NewReplacer("%2f", "/", "%2F", "/")
					contentName := contentFile.Name()

					// Create a route name for the mapping
					routeName := replacer.Replace(contentName)

					// Pull the file
					b, err := ioutil.ReadFile(path.Join(filePath, website, contentName))
					if err != nil {
						log.Fatal(err)
					}
					log.Print("Loaded route: " + routeName)
					m[website][0][routeName] = []byte(b)

					// All of the assets for the site
				} else if contentFile.Name() == "assets" {
					assets, err := ioutil.ReadDir(path.Join(filePath, website, "assets"))
					if err != nil {
						log.Fatal("Error when reading assets dir: ", err)
					}
					for _, asset := range assets {
						if !asset.IsDir() {
							// Pull the file
							b, err := ioutil.ReadFile(path.Join(filePath, website, "assets", asset.Name()))
							if err != nil {
								log.Fatal(err)
							}
							log.Print("Loaded asset: " + asset.Name())
							m[website][1][asset.Name()] = []byte(b)
						}
					}
				}
			}
		}
	}
	s.mux.Lock()
	s.content = m
	s.mux.Unlock()
}

func (s *State) SyncContentFromPool() {

}

func getContentDir() (string, error) {
	// TODO: Actually get correct filepath
	// TODO: Add configurable values from a config file
	contentDir := config.GetString("ContentDirectory")
	if contentDir == "" {
		return contentDir, errors.New("No content directory specified")
	}
	return contentDir, nil
}
