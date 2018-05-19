// Package state contains a thread safe state struct that stores information
// about the networkd
package state

import (
	"errors"
	"io/ioutil"
	"log"
	"path"
	"strings"
	"sync"

	"github.com/gladiusio/gladius-utils/config"
)

// New returns a new state struct
func New() *State {
	state := &State{running: true, content: make(map[string]map[string]string), runChannel: make(chan bool)}
	state.LoadContentFromDisk()
	return state
}

type State struct {
	running    bool
	content    map[string]map[string]string
	runChannel chan (bool)
	mux        sync.Mutex
}

func (s *State) Content() map[string]map[string]string {
	s.mux.Lock()
	// Lock so only one goroutine at a time can access the map
	defer s.mux.Unlock()
	return s.content
}

func (s *State) SetContentRunState(runState bool) {
	s.mux.Lock()
	if s.running != runState {
		s.running = runState
		go func() { s.runChannel <- runState }()
	}
	s.mux.Unlock()
}

func (s *State) RunningStateChanged() chan (bool) {
	return s.runChannel
}

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

	m := make(map[string]map[string]string)

	for _, f := range files {
		website := f.Name()
		if f.IsDir() {
			contentFiles, err := ioutil.ReadDir(path.Join(filePath, website))
			if err != nil {
				log.Fatal("Error when reading content dir: ", err)
			}
			log.Print("Loading website: " + website)
			m[website] = make(map[string]string)
			for _, contentFile := range contentFiles {
				// Replace "%2f" with "/" and ".json" with ""
				replacer := strings.NewReplacer("%2f", "/", "%2F", "/", ".html", "")
				contentName := contentFile.Name()

				// Create a route name for the mapping
				routeName := replacer.Replace(contentName)

				// Pull the file
				b, err := ioutil.ReadFile(path.Join(filePath, website, contentName))
				if err != nil {
					log.Fatal(err)
				}
				log.Print("Loaded route: " + routeName)
				m[website][routeName] = string(b)
			}
		}
	}
	s.mux.Lock()
	s.content = m
	s.mux.Unlock()
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
