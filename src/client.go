package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v2"
)

func client() {

	var globalErr bool

	// Get config, get data from it
	f, err := ioutil.ReadFile("./sharpcd.yml")
	var con config
	err = yaml.Unmarshal(f, &con)
	handle(err, "Failed to read and extract sharpcd.yml")

	// POST to sharpcd server for each task
	for id, task := range con.Tasks {

		// Make ID Lower Case
		id := strings.ToLower(id)
		id = strings.ReplaceAll(id, " ", "_")

		payload := postData{
			ID:         id,
			Name:       task.Name,
			Type:       task.Type,
			GitURL:     task.GitURL,
			Command:    task.Command,
			Compose:    task.Compose,
			Enviroment: getEnviroment(task.Envfile),
			Registry:   task.Registry,
			Secret:     getSec()}

		// Make POST request and let user know if successful
		body, code := post(payload, task.SharpURL)
		if code == statCode.Accepted {
			fmt.Printf("Task [%s] succesfully sent!\n", task.Name)
			fmt.Println("=================")
			err = postCommChecks(task, id)
			if err != nil {
				globalErr = true
			}
			fmt.Println("")
			fmt.Println("")
		} else {
			fmt.Println(body.Message)
			fmt.Printf("Task %s Failed!\n\n", task.Name)
		}
	}

	if globalErr {
		fmt.Println("At least one task Failed!")
		os.Exit(1)
	}
}

// Makes POST Request adn reads response
func post(payload postData, url string) (response, int) {
	// Create POST request with JSON
	jsonStr, err := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	handle(err, "Failed to create request")

	// Create client
	// Allow self-signed certs
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	// Do Request
	resp, err := client.Do(req)
	handle(err, "Failed to do POST request")
	defer resp.Body.Close()

	// Read Body and Status
	body, err := ioutil.ReadAll(resp.Body)
	handle(err, "Failed to read body of response")

	var respBody response
	err = json.Unmarshal(body, &respBody)
	handle(err, "Failed to convert repsonse to JSON")

	return respBody, resp.StatusCode
}

// Get the enviromental vars from a env file
func getEnviroment(loc string) map[string]string {

	var env map[string]string

	// If no envfile specified end function
	if len(loc) == 0 {
		return env
	}

	// Get env contents
	env, err := godotenv.Read(loc)
	handle(err, "Failed to Load .env")
	return env
}

func postCommChecks(t task, id string) error {
	jobURL := t.SharpURL + "/api/job/" + id
	logsURL := t.SharpURL + "/api/logs/" + id
	payload := postData{
		Secret: getSec()}
	buildingTriggered := false
	runningTriggered := false
	counter := 0

	fmt.Println("Waiting on server response...")
	for {
		resp, code := post(payload, jobURL)
		if code != statCode.Accepted {
			log.Fatal("Something went wrong using the API!")
			return errors.New("Bad API")
		}

		if resp.Job == nil {
			continue
		}
		job := resp.Job

		stopped := job.Status == jobStatus.Stopped && runningTriggered
		errored := job.Status == jobStatus.Errored
		building := job.Status == jobStatus.Building && !buildingTriggered
		running := job.Status == jobStatus.Running && !runningTriggered

		if building {
			buildingTriggered = true
			fmt.Println("The Task is now building a job")
		}

		if running {
			runningTriggered = true
			fmt.Println("Task Has Successfully started running!")
			fmt.Println("Making sure it does not stop unexpectedly...")
		}

		if errored || stopped {
			fmt.Println("Task stopped running!")
			fmt.Println("Error Message: " + job.ErrMsg)

			logResp, code := post(payload, logsURL)
			if code != statCode.Accepted {
				log.Fatal("Something went wrong using the API!")
				return errors.New("Bad API")
			}
			file := logResp.Message

			if file != "" {
				fmt.Println("Logs File:")
				fmt.Println(file)
			}

			return errors.New("Bad Task")
		}

		if counter > 7 {
			fmt.Println("Task has started Properly!")
			return nil
		}

		if runningTriggered {
			counter++
		}

		time.Sleep(1 * time.Second)
	}
}
