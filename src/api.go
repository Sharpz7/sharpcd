package main

import (
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// Checks which API section is required and
// runs the appropreiate command
func getAPIData(r *http.Request, resp *response) error {
	var err error

	path := strings.Split(r.URL.Path[5:], "/")
	switch path[0] {
	case "jobs":
		resp.Jobs = allJobs.List
		return nil
	case "job":
		resp.Job, err = getJobs(path[1])
		return err
	case "logs":
		resp.Message, err = getLogs(path[1])
		return err
	}

	return nil
}

// Gets the logs from the task ID's file
func getLogs(path string) (string, error) {
	logs := folder.Logs + path + "/info.log"
	file, err := os.Open(logs)
	if err != nil {
		return "", err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func getJobs(path string) (*taskJob, error) {
	var emptyJob *taskJob

	for _, job := range allJobs.List {
		if job.ID == path {
			err := checkJobStatus(job)
			return job, err
		}
	}

	return emptyJob, errors.New("job not found")
}

func checkJobStatus(job *taskJob) error {
	logs, err := getLogs(job.ID)
	if strings.Contains(logs, "exited with code") {
		job.Status = jobStatus.Stopped
		job.ErrMsg = "A Container exited unexpectedly"
	}

	return err
}
