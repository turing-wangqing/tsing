package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	officalaws "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awslambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/turingvideo/broadway/databases/sql/pg"
	"github.com/turingvideo/broadway/handlers/export"
	"github.com/turingvideo/turing-common/log"
	"github.com/turingvideo/turing-common/sql"
)

const execChannel string = "lambda"

var handler *export.Exporter
var svc *awslambda.Lambda
var childArn string

func init() {
	awsRegion := os.Getenv("AWS_Region")
	childArn = os.Getenv("CHILD_LAMBDA_ARN")
	pgUserName := os.Getenv("PG_USERNAME")
	pgPwd := os.Getenv("PG_USERPWD")
	pgDBName := os.Getenv("PG_DBNAME")
	pgDBHost := os.Getenv("PG_HOST")

	logger := log.Logger("export")
	handler = &export.Exporter{}
	handler.SetLogger(&logger)
	if sql, err := pg.NewManager(sql.Config{
		DBAddress:  pgDBHost,
		DBName:     pgDBName,
		DBUsername: pgUserName,
		DBPassword: pgPwd,
	}, sql.PGConnectionConfig{
		ConnectionPoolSize: 10,
	}); err != nil {
		fmt.Println("sql init fail.", err)
		return
	} else {
		handler.SQL = sql
	}

	sess, err := session.NewSession(&officalaws.Config{
		Region: officalaws.String(awsRegion),
	})
	if err != nil {
		fmt.Println("lambda client init fail.", err)
		return
	}
	svc = awslambda.New(sess)
	return
}

func check() error {
	if handler == nil || handler.SQL == nil {
		return fmt.Errorf("init fail")
	}
	return nil
}

func execute() error {
	if err := check(); err != nil {
		return err
	} else {
		fmt.Println("init success")
	}
	if err := handler.CreateBeatJobs(execChannel); err != nil {
		fmt.Print("create beat jobs fail.", err)
		return err
	}
	jobs, err := handler.GetBeatJobs(execChannel)
	if err != nil {
		fmt.Print("get execute jobs fail.", err)
		return err
	}

	for _, job := range jobs {
		body, err := json.Marshal(job)
		if err != nil {
			fmt.Println("serialize job fail.", err, job.Id)
			continue
		}
		fmt.Println("invoke lambda.", childArn, string(body))
		out, err := svc.Invoke(&awslambda.InvokeInput{
			FunctionName:   officalaws.String(childArn),
			InvocationType: officalaws.String("Event"),
			Payload:        body,
		})
		if err != nil {
			fmt.Println("job id failure:", job.Id, out, err)
		} else {
			fmt.Println("job id success:", job.Id)
		}
	}
	return nil
}

func main() {
	lambda.Start(execute)
}
