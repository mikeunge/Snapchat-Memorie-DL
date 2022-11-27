package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type IDLObject struct {
	Date  string `json:"Date"`
	MediaType string `json:"Media Type"`
	DownloadLink  string `json:"Download Link"`
}

type IMedia struct {
	Media []IDLObject `json:"Saved Media"`
}

type IConfig struct {
	Workers uint `json:"Workers"`
	Tries uint `json:"Tries"`
	Root string `json:"Root"`
	VideoPath string `json:"VideoPath"`
	ImagePath string `json:"ImagePath"`
	TimeZone string `json:"TimeZone"`
	LogFile string `json:"LogFile"`
}

var logger log.Logger
var config IConfig

func checkErrorAndExit(err error, exitCode int) {
	if err != nil {
		logger.Printf("[ERRO] %+v\n", err)
		fmt.Fprintf(os.Stderr, "[ERRO] %+v\n", err)
		os.Exit(exitCode)
	}
}

func createFolderIfNotExist(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.Mkdir(path, 0755)
		if err != nil {
			return fmt.Errorf("[ERRO] could not create folder: %s.\n\nError: %+v", path, err)
		}
	}
	return nil
}

func loadConfig(path string) error {
	data, err := os.Open(path)
	if err != nil {
		return err
	}
	defer data.Close()
	bytes, _ := ioutil.ReadAll(data)
	json.Unmarshal(bytes, &config)
	return nil
}

func init() {
	err := loadConfig("./config.json"); checkErrorAndExit(err, 1)

	// set timezone - used for date manipulation
	os.Setenv("TZ", config.TimeZone)

	// setup the logger
	logfile, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666); checkErrorAndExit(err, -1)
	logger.SetOutput(logfile)

	// create folders where we download the videos/images to
	err = createFolderIfNotExist(fmt.Sprintf("%s/%s", config.Root, "")); checkErrorAndExit(err, 2)
	err = createFolderIfNotExist(fmt.Sprintf("%s/%s", config.Root, config.ImagePath)); checkErrorAndExit(err, 2)
	err = createFolderIfNotExist(fmt.Sprintf("%s/%s", config.Root, config.VideoPath)); checkErrorAndExit(err, 2)
}

// The worker(s) invoke the save function with the given parameters
func worker(channel chan IDLObject, channelId chan uint, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	for obj := range channel {
		id := <-channelId
		err := download(id, &obj); fmt.Print(err)
	}
}

func makeFilepath(id, try uint, data *IDLObject) (string, error) {
	var extenstion, folder string
	filename := fmt.Sprintf("%s %s-%d", strings.Split(data.Date, " ")[0], strings.Split(data.Date, " ")[1], try)
	switch {
	case data.MediaType == "Image":
		folder = config.ImagePath 
		extenstion = "jpg"
	case data.MediaType == "Video":
		folder = config.VideoPath
		extenstion = "mp4"
	default:
		return "", fmt.Errorf("[WARN] %d - media type unknown, skipping", id)
	}
	return fmt.Sprintf("%s/%s/%s.%s", config.Root, folder, filename, extenstion), nil
}

func changeFileTime(id uint, path, filetime string) error {
	newTime := fmt.Sprintf("%s %s", strings.Split(filetime, " ")[0], strings.Split(filetime, " ")[1])
	modTime, err := time.Parse("2006-01-02 15:04:05", newTime)
	if err != nil {
		return fmt.Errorf("[WARN] %d - could not parse time", id)
	}
	return os.Chtimes(path, time.Now().Local(), modTime)
}

func download(id uint, obj *IDLObject) error {
	logger.Printf("[INFO] %d - starting download - date: %s - media: %s", id, obj.Date, obj.MediaType)
	res, err := http.Post(obj.DownloadLink, "application/x-www-form-urlencoded", nil)
	if err != nil || res.StatusCode != 200 {
		return fmt.Errorf("[ERRO] %d - SNAPCHAT - response status code is not correct for element, response: %d", id, res.StatusCode)
	}
	defer res.Body.Close()

	/**
	 * Snapchat hide the actual image link behind their own url.
	 * All the memories are stored on aws buckets, the aws url gets returned from the first url (POST) request.
	 *
	 * We get the first response, create a new byte buffer, feed it the recieved data from the body and construct our actual image url.
	 * From the 'new' url we now can download the actual media.
	 */
	buf := new(bytes.Buffer)
	buf.ReadFrom(res.Body)
	url := buf.String()
	res, err = http.Get(url)
	if err != nil || res.StatusCode != 200 {
		return fmt.Errorf("[ERRO] %d - AWS - response error occured for element", id)
	}
	defer res.Body.Close()
	dir, err := makeFilepath(id, 0, obj)
	if err != nil {
		return err
	}

	// make sure the filename does not already exist
	for i := uint(1); i <= config.Tries; i++ {
		if _, err := os.Stat(dir); err == nil {
			logger.Printf("[WARN] %d - filename exists, generating new name - try: %d", id, i)
			dir, err = makeFilepath(id, i, obj)
			if err != nil {
				return err
			}
		} else {
			break
		}
	}

	// create the file and write data to it
	file, err := os.Create(dir)
	if err != nil {
		return fmt.Errorf("[ERRO] %d - could not create file: %s", id, dir)
	}
	defer file.Close()
	if _, err = io.Copy(file, res.Body); err != nil {
		return fmt.Errorf("[ERRO] %d - could not write data to file: %s", id, dir)
	}

	// change the modified/access time
	if err = changeFileTime(id, dir, obj.Date); err != nil {
		logger.Printf("[WARN] %d - %+v", id, err)
	}
	logger.Printf("[INFO] %d - %s successfully saved: %s\n", id, obj.MediaType, dir)
	return nil
}

func main() {
	const memoriesPath = "json/memories_history.json"
	data, err := os.Open(memoriesPath); checkErrorAndExit(err, 1)
	defer data.Close()

	var media IMedia
	bytes, _ := ioutil.ReadAll(data)
	json.Unmarshal(bytes, &media)

	// setup the worker channels and the waitgroup
	channel := make(chan IDLObject)
	channelId := make(chan uint)
	wg := new(sync.WaitGroup)

	// create the workers, provide the channels and assign them to the waitgroup
	for i := uint(0); i < config.Workers; i++ {
		wg.Add(1)
		go worker(channel, channelId, wg)
	}

	// provide the workers with work
	logger.Printf("Data successfully read from: %s ... starting to download %d elements\n\n", memoriesPath, len(media.Media))
	for id, obj := range media.Media {
		channel <- obj
		channelId <- uint(id)
	}

	close(channel)
	close(channelId)
	wg.Wait() // wait till the work is done

	fmt.Fprintf(os.Stdout, "\nDone\n")
}