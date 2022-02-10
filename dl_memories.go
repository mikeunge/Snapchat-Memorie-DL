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
)

type DLObject struct {
	Date  string `json:"Date"`
	Media string `json:"Media Type"`
	Link  string `json:"Download Link"`
}

type Media struct {
	SavedMedia []DLObject `json:"Saved Media"`
}

var logger log.Logger

func getMeta(id int, meta *DLObject) (string, string, string, error) {
	var baseDir string
	var extenstion string
	var name string

	name = strings.Split(meta.Date, " ")[0] + " " + strings.Split(meta.Date, " ")[1]
	if meta.Media == "Image" {
		baseDir = "images/"
		extenstion = ".jpg"
	} else if meta.Media == "Video" {
		baseDir = "videos/"
		extenstion = ".mp4"
	} else {
		e := fmt.Errorf("[WARN] %d - media type unknown, skipping\n", id)
		logger.Println(e)
		return baseDir, extenstion, name, e
	}

	return baseDir, extenstion, name, nil
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

	baseDir, extension, name, e := getMeta(id, obj)
	if e != nil {
		return e
	}

	var dir = fmt.Sprintf("%s%d - %s.%s", baseDir, id, name, extension)
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
	fmt.Fprintf(os.Stdout, "[INFO] %d - %s successfully saved: %s\n", id, obj.Media, dir)
	return nil
}

func worker(wChan chan DLObject, idChan chan int, wg *sync.WaitGroup) {
	defer wg.Done()

	for obj := range wChan {
		e := save(<-idChan, &obj)
		if e != nil {
			fmt.Println(e)
		}
	}
}

func init() {
	logfile, e := os.OpenFile("dl_log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if e != nil {
		fmt.Fprintf(os.Stderr, "[ERRO] could not setup logger.\n\nError: %+v\n", e)
		os.Exit(-1)
	}
	logger.SetOutput(logfile)
}

func main() {
	const workers = 100
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
