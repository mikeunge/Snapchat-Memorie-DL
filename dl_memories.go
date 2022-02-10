package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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

func save(id int, obj DLObject) error {
	id = id + 1
	fmt.Fprintf(os.Stdout, "[INFO] %d - starting download\n", id)
	response, e := http.Post(obj.Link, "application/x-www-form-urlencoded", nil)
	if e != nil {
		err := fmt.Errorf("[ERRO] %d - response error occured for element\n", id)
		logger.Println(fmt.Sprintf("[ERRO] %d - %+v", id, e))
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		err := fmt.Errorf("[ERRO] %d - response status code is not correct for element\n", id)
		logger.Println(fmt.Sprintf("[ERRO] %d - %+v", id, e))
		return err
	}

	var url = fmt.Sprintf("%+v", response.Body)
	fmt.Fprintf(os.Stdout, "[INFO] %d - returned url: %s\n", id, url)
	res, e := http.Post(url, "application/x-www-form-urlencoded", nil)
	if e != nil {
		err := fmt.Errorf("[ERRO] %d - response error occured for element\n", id)
		logger.Println(fmt.Sprintf("[ERRO] %d - %+v", id, e))
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		err := fmt.Errorf("[ERRO] %d - response status code is not correct for element\n", id)
		logger.Println(fmt.Sprintf("[ERRO] %d - %+v", id, e))
		return err
	}
	fmt.Fprintf(os.Stdout, "[INFO] %d - download finished\n", id)

	var baseDir string
	var extenstion string
	if obj.Media == "Image" {
		baseDir = "images/"
		extenstion = ".jpg"
	} else if obj.Media == "Video" {
		baseDir = "videos/"
		extenstion = ".mp4"
	} else {
		err := fmt.Errorf("[WARN] %d - media type unknown, skipping\n", id)
		logger.Println(err)
		return err
	}

	var dir = fmt.Sprintf("%s%s - %d.%s", baseDir, obj.Date, id, extenstion)
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

func init() {
	logfile, e := os.OpenFile("dl_log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if e != nil {
		fmt.Fprintf(os.Stderr, "[ERRO] could not setup logger.\n\nError: %+v\n", e)
		os.Exit(-1)
	}
	logger.SetOutput(logfile)
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

	fmt.Fprintf(os.Stdout, "[INFO] data read... starting to download %d elements\n", len(media.SavedMedia))
	for i := 0; i < 5; i++ {
		e = save(i, media.SavedMedia[i])
		fmt.Println(e)
	}
}
