package state

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

func (s *State) startContentFileWatcher() {
	// creates a new file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Can't add watcher to content directory")
		defer watcher.Close()
	}
	done := make(chan bool)
	go func() {
		for {
			select {
			// watch for events
			case event := <-watcher.Events:
				if event.Op&fsnotify.Create == fsnotify.Create ||
					event.Op&fsnotify.Remove == fsnotify.Remove ||
					event.Op&fsnotify.Rename == fsnotify.Rename {
					s.loadContentFromDisk()
				}

			// watch for errors
			case watchErr := <-watcher.Errors:
				if watchErr != nil {
					log.WithFields(log.Fields{
						"error": watchErr,
					}).Error("Error watching content direcory")
				}
			}
		}
	}()

	// out of the box fsnotify can watch a single file, or a single directory
	filePath, err := getContentDir()
	fmt.Println(filePath)
	if err != nil {
		log.Fatal("Error getting content dir", err)
	}
	files, err := ioutil.ReadDir(filePath)
	if err != nil {
		log.Fatal("Error when reading content dir: ", err)
	}
	for _, f := range files {
		website := f.Name()
		if f.IsDir() {
			if err := watcher.Add(path.Join(filePath, website)); err != nil {
				log.WithFields(log.Fields{
					"error":     err,
					"directory": path.Join(filePath, website),
				}).Error("Can't add watcher to website directory")
			}
		}
	}

	<-done
}

// LoadContentFromDisk loads the content from the disk and stores it in the state
func (s *State) loadContentFromDisk() {
	filePath, err := getContentDir()
	if err != nil {
		log.Fatal("Error getting content dir", err)
	}

	files, err := ioutil.ReadDir(filePath)
	if err != nil {
		log.Fatal("Error when reading content dir: ", err)
	}
	// map websites
	cs := &contentStore{make(map[string]*websiteContent)}

	for _, f := range files {
		website := f.Name()
		if f.IsDir() {
			// Create a website store
			wc := cs.createWebsite(website)

			// Get all of the files for that website
			websiteFiles, err := ioutil.ReadDir(path.Join(filePath, website))
			if err != nil {
				log.Fatal("Error when reading content dir: ", err)
			}
			log.WithFields(log.Fields{
				"website": website,
			}).Debug("Loading website: " + website)
			for _, websiteFile := range websiteFiles {
				// Ignore subdirecories
				if !websiteFile.IsDir() {
					fileName := websiteFile.Name()

					// Pull the file
					b, err := ioutil.ReadFile(path.Join(filePath, website, fileName))
					if err != nil {
						log.WithFields(log.Fields{
							"err":       err,
							"file_name": fileName,
						}).Fatal("Error loading asset")
					}
					// Create the asset in the website content
					wc.createAsset(fileName, []byte(b))
					log.WithFields(log.Fields{
						"asset_name": fileName,
					}).Debug("Loaded new asset")

				}
			}
		}
	}
	s.mux.Lock()
	s.content = cs

	// Tell the controld about our new content
	err = s.p2p.UpdateField("disk_content", cs.getContentList()...)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err.Error(),
		}).Warn("Error updating disk content")
	}
	s.mux.Unlock()
}

func (s *State) startContentSyncWatcher() {
	// Get the files we have on disk now
	s.loadContentFromDisk()
	go s.startContentFileWatcher()

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
						log.Fatal("Can't find content dir")
						return
					}

					contentURL := contentLocations[r.Intn(len(contentLocations))]

					// Create a filepath location from the content name
					toDownload := filepath.Join(append([]string{contentDir}, strings.Split(contentName, "/")...)...)

					// Pass in the name so we can verify the hash (filename is the hash)
					err = downloadFile(toDownload, contentURL, contentName)
					if err != nil {
						log.WithFields(log.Fields{
							"url":      contentURL,
							"filename": contentName,
							"path":     toDownload,
							"err":      err.Error(),
						}).Warn("Error downloading file from peer")
					}
				}
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

	if fmt.Sprintf("%X", h.Sum(nil)) != name {
		out.Close()
		os.Remove(filepath)
		return errors.New("incomming file from peer did not match expected hash")
	}

	log.WithFields(log.Fields{
		"url":      url,
		"filename": name,
		"path":     filepath,
	}).Debug("A new file was downloaded from a peer")
	return nil
}
