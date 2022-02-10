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

type DLObject struct {
	Date  string `json:"Date"`
	Media string `json:"Media Type"`
	Link  string `json:"Download Link"`
}

type Media struct {
	SavedMedia []DLObject `json:"Saved Media"`
}

const (
	workers  = 1
	baseDir  = "."
	videoDir = "videos"
	imageDir = "images"
	timeZone = "Europe/Vienna"
)

var logger log.Logger

func init() {
	os.Setenv("TZ", timeZone)
	logfile, e := os.OpenFile("dl_log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if e != nil {
		fmt.Fprintf(os.Stderr, "[ERRO] could not setup logger.\n\nError: %+v\n", e)
		os.Exit(-1)
	}
	logger.SetOutput(logfile)

	imgFolder := fmt.Sprintf("%s/%s", baseDir, imageDir)
	vidFolder := fmt.Sprintf("%s/%s", baseDir, videoDir)
	if _, err := os.Stat(imgFolder); os.IsNotExist(err) {
		err := os.Mkdir(imgFolder, 0755)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERRO] could not create folder: %s.\n\nError: %+v\n", imgFolder, e)
			os.Exit(2)
		}
	}

	if _, err := os.Stat(vidFolder); os.IsNotExist(err) {
		err := os.Mkdir(vidFolder, 0755)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERRO] could not create folder: %s.\n\nError: %+v\n", vidFolder, e)
			os.Exit(2)
		}
	}
}

func main() {
	var jsonFilePath = "json/memories_history.json"
	jsonFile, e := os.Open(jsonFilePath)
	if e != nil {
		fmt.Fprintf(os.Stderr, "[ERRO] could not read data from file: %s\n\nError: %+v\n", jsonFilePath, e)
		os.Exit(1)
	}
	defer jsonFile.Close()

	var media Media
	byteValue, _ := ioutil.ReadAll(jsonFile)
	json.Unmarshal(byteValue, &media)

	c := make(chan DLObject)
	iC := make(chan int)
	wg := new(sync.WaitGroup)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker(c, iC, wg)
	}

	fmt.Fprintf(os.Stdout, "[INFO] data read... starting to download %d elements\n", len(media.SavedMedia))
	for id, obj := range media.SavedMedia {
		c <- obj
		iC <- id
	}

	close(c)
	wg.Wait()
}

// The worker(s) invoke the save function with the given parameter
func worker(wChan chan DLObject, idChan chan int, wg *sync.WaitGroup) {
	defer wg.Done()

	for obj := range wChan {
		e := save(<-idChan, &obj)
		if e != nil {
			fmt.Println(e)
		}
	}
}

func makeFilepath(id int, meta *DLObject) (string, error) {
	var folder string
	var extenstion string
	var filename string
	var dir string

	filename = strings.Split(meta.Date, " ")[0] + " " + strings.Split(meta.Date, " ")[1]
	if meta.Media == "Image" {
		folder = imageDir
		extenstion = "jpg"
	} else if meta.Media == "Video" {
		folder = videoDir
		extenstion = "mp4"
	} else {
		e := fmt.Errorf("[WARN] %d - media type unknown, skipping\n", id)
		logger.Println(e)
		return dir, e
	}
	dir = fmt.Sprintf("%s/%s/%s.%s", baseDir, folder, filename, extenstion)
	return dir, nil
}

func changeFileTime(id int, path, t string) error {
	layout := "2006-01-02 15:04:05"
	t = fmt.Sprintf("%s %s", strings.Split(t, " ")[0], strings.Split(t, " ")[1])
	modTime, err := time.Parse(layout, t)
	if err != nil {
		e := fmt.Errorf("[WARN] %d - could not parse time\n", id)
		logger.Println(fmt.Sprintf("[WARN] %d - %+v", id, e))
		return e
	}
	return os.Chtimes(path, time.Now().Local(), modTime)
}

func save(id int, obj *DLObject) error {
	fmt.Fprintf(os.Stdout, "[INFO] %d - starting download\n", id)
	response, e := http.Post(obj.Link, "application/x-www-form-urlencoded", nil)
	if e != nil {
		err := fmt.Errorf("[ERRO] %d - response error occured for element\n", id)
		fmt.Fprintf(os.Stderr, "[ERRO] %d - response error occured for element\n", id)
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		e := fmt.Errorf("[ERRO] %d - response status code is not correct for element\n", id)
		logger.Println(fmt.Sprintf("[ERRO] %d - %+v", id, e))
		return e
	}

	/**
	 * Snapchat is a bit weird, they hide the actual image link behind their own url.
	 * All the memories are stored on aws buckets, the aws url gets returned from the first url (POST) request.
	 *
	 * We get the first response, create a new byte buffer, feed it the recieved data from the body and construct our actual url.
	 * From the 'new' url we now can download the actual media.
	 */
	buf := new(bytes.Buffer)
	buf.ReadFrom(response.Body)
	url := buf.String()
	res, e := http.Get(url)
	if e != nil {
		err := fmt.Errorf("[ERRO] %d - response error occured for element\n", id)
		logger.Println(fmt.Sprintf("[ERRO] %d - %+v", id, e))
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		e := fmt.Errorf("[ERRO] %d - response status code is not correct for element\n", id)
		logger.Println(fmt.Sprintf("[ERRO] %d - %+v", id, e))
		return e
	}
	fmt.Fprintf(os.Stdout, "[INFO] %d - download finished\n", id)

	dir, e := makeFilepath(id, obj)
	if e != nil {
		return e
	}

	file, e := os.Create(dir)
	if e != nil {
		err := fmt.Errorf("[ERRO] %d - could not create file: %s\n", id, dir)
		logger.Println(fmt.Sprintf("[ERRO] %d - %+v", id, e))
		return err
	}
	defer file.Close()
	fmt.Fprintf(os.Stdout, "[INFO] %d - file created: %s\n", id, dir)

	fmt.Fprintf(os.Stdout, "[INFO] %d - writing %s data to: %s\n", id, obj.Media, dir)
	_, e = io.Copy(file, res.Body)
	if e != nil {
		err := fmt.Errorf("[ERRO] %d - could not write to file: %s\n", id, dir)
		logger.Println(fmt.Sprintf("[ERRO] %d - %+v", id, e))
		return err
	}

	e = changeFileTime(id, dir, obj.Date)
	if e != nil {
		fmt.Fprintf(os.Stdout, "[WARN] %d - could not set modified/access time\n", id)
		logger.Println(fmt.Sprintf("[WARN] %d - %+v", id, e))
	}

	fmt.Fprintf(os.Stdout, "[INFO] %d - %s successfully saved: %s\n", id, obj.Media, dir)
	return nil
}
