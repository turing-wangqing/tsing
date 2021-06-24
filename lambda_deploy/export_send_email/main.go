package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/turingvideo/broadway/email"
	"github.com/turingvideo/broadway/handlers/export"
	"github.com/turingvideo/turing-common/log"
	"github.com/turingvideo/turing-common/model"
)

type lambdaMsg struct {
	FileUrl string
	Params  model.TaskParams
}

const (
	defaultExportDir = "/tmp"
)

func getFilePath(fileUrl string) (string, error) {
	u, err := url.Parse(fileUrl)
	if err != nil {
		return "", err
	}
	ss := strings.Split(u.Path, "/")
	return path.Join(defaultExportDir, ss[len(ss)-1]), nil
}

func execute(msg lambdaMsg) error {
	emailFrom := os.Getenv("EMAIL_FROM")
	emailHost := os.Getenv("EMAIL_HOST")
	emailPort := os.Getenv("EMAIL_PORT")
	emailUserName := os.Getenv("EMAIL_USERNAME")
	emailPwd := os.Getenv("EMAIL_PASSWORD")

	port, _ := strconv.Atoi(emailPort)
	handler := export.Exporter{}
	logger := log.Logger("export")
	handler.SetLogger(&logger)

	emailClient := email.NewSTMPClient(&email.Config{
		From:     emailFrom,
		Host:     emailHost,
		Port:     port,
		UserName: emailUserName,
		Password: emailPwd,
	})
	handler.Email = emailClient

	res, err := http.Get(msg.FileUrl)
	if err != nil {
		return fmt.Errorf("failed to get s3.%s,%v", msg.FileUrl, err)
	}
	body, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to read res body.%s,%v", msg.FileUrl, err)
	}
	filePath, err := getFilePath(msg.FileUrl)
	if err != nil {
		return fmt.Errorf("create file path fail.%v,%v", msg.FileUrl, err)
	}
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("open file fail.%v,%v", filePath, err)
	}
	if _, err := file.Write(body); err != nil {
		return fmt.Errorf("failed to write body into file.%s,%v", msg.FileUrl, err)
	}
	if err := handler.NotifyEmail(filePath, msg.FileUrl, &msg.Params); err != nil {
		_ = os.Remove(filePath)
		return err
	}
	_ = os.Remove(filePath)
	return nil

}

func main() {
	lambda.Start(execute)
}
