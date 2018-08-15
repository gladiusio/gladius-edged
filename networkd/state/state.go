// Package state contains a thread safe state struct that stores information
// about the networkd
package state

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gladiusio/gladius-utils/config"
)

// New returns a new state struct
func New(version string) *State {
	state := &State{running: true, content: make(map[string]([2](map[string][]byte))), runChannel: make(chan bool), version: version}
	state.startContentSyncWatcher()
	return state
}

// State is a thread safe struct for keeping information about the networkd
type State struct {
	running    bool
	content    map[string]([2](map[string][]byte)) // A map of website to an array of maps, the first being page content, the second being assets
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

func (s *State) Info() string {
	s.mux.Lock()
	defer s.mux.Unlock()

	status := &status{Running: s.running, Version: s.version}

	jsonString, _ := json.Marshal(status)
	return string(jsonString)
}

// LoadContentFromDisk loads the content from the disk and stores it in the state
func (s *State) loadContentFromDisk() {
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

func (s *State) startContentSyncWatcher() {
	// Get the files we have on disk now
	s.loadContentFromDisk()

	/* If there is new content we need, sleep for a random time then ask which
	nodes have it in the network, then download it from a random one. This allows
	a semi random	propogation so we can minimize individal load on nodes.*/
	go func() {
		for {
			time.Sleep(2 * time.Second)       // Sleep to give the controld a break
			siteContent := s.getContentList() // Fetch what we have on disk in a format that's understood by the controld
			contentNeeded := getNeededFromControld(siteContent)

			if len(contentNeeded) > 0 {
				r := rand.New(rand.NewSource(time.Now().Unix()))
				time.Sleep(time.Duration(r.Intn(10)) * time.Second) // Random sleep allow better propogation

				for _, nc := range getContentLocationsFromControld(contentNeeded) {
					contentLocations := nc.contentLocations
					contentName := nc.contentName

					contentDir, err := getContentDir()
					if err != nil {
						log.Println("Can't find content dir")
						return
					}

					contentURL := contentLocations[r.Intn(len(contentLocations))]

					// Create a filepath location from the content name
					toDownload := filepath.Join(append([]string{contentDir}, strings.Split(contentName, "/")...)...)

					// Pass in the name so we can verify the hash (filename is the hash)
					downloadFile(toDownload, contentURL, contentName)
				}
				s.loadContentFromDisk()
			}
		}
	}()
}

// downloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func downloadFile(filepath, url, name string) error {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	// Check the hash of the file
	h := sha256.New()
	if _, err := io.Copy(h, out); err != nil {
		log.Fatal(err)
	}

	if fmt.Sprintf("%x", h.Sum(nil)) != name {
		out.Close()
		os.Remove(filepath)
		return errors.New("incomming file from peer did not match expected hash")
	}

	return nil
}

type networkContent struct {
	contentName      string
	contentLocations []string
}

// getNeededFromControld asks the controld what we need
func getNeededFromControld(contentOnDisk []string) []string {
	return []string{}
}

// getContentLocationsFromControld gets a list of lists to the links of files
func getContentLocationsFromControld(contentNeeded []string) []*networkContent {
	return []*networkContent{&networkContent{}}
}

// getContentList returns a list of the content we have on disk in the format of:
// <website name>/<asset or content>/<fileName>
func (s *State) getContentList() []string {
	s.mux.Lock()
	defer s.mux.Unlock()

	contentList := make([]string, 0)

	for websiteName, websiteData := range s.content {
		for routeName := range websiteData[0] {
			contentList = append(contentList, strings.Join([]string{websiteName, "content", routeName}, "/"))
		}
		for assetName := range websiteData[1] {
			contentList = append(contentList, strings.Join([]string{websiteName, "asset", assetName}, "/"))
		}

	}

	return contentList
}

func getContentDir() (string, error) {
	contentDir := config.GetString("ContentDirectory")
	if contentDir == "" {
		return contentDir, errors.New("No content directory specified")
	}
	return contentDir, nil
}
