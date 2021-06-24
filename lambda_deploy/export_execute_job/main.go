package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	officalaws "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awslambda "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/turingvideo/broadway/databases/nosql/mongo"
	"github.com/turingvideo/broadway/databases/sql/pg"
	"github.com/turingvideo/broadway/handlers/export"
	"github.com/turingvideo/turing-common/aws"
	"github.com/turingvideo/turing-common/log"
	"github.com/turingvideo/turing-common/model"
	"github.com/turingvideo/turing-common/sql"
)

var handler *export.Exporter

type lambdaMsg struct {
	FileUrl string
	Params  model.TaskParams
}

var childArn string
var svc *awslambda.Lambda

func init() {
	pgUserName := os.Getenv("PG_USERNAME")
	pgPwd := os.Getenv("PG_USERPWD")
	pgDBName := os.Getenv("PG_DBNAME")
	pgDBHost := os.Getenv("PG_HOST")
	mgUserName := os.Getenv("MG_USERNAME")
	mgPwd := os.Getenv("MG_USERPWD")
	mgDBName := os.Getenv("MG_DBNAME")
	mgDBHost := os.Getenv("MG_HOST")
	mgRetName := os.Getenv("MG_RETNAME")
	awsKeyID := os.Getenv("AWS_KEY_ID")
	awsKeySecret := os.Getenv("AWS_KEY_SECRET")
	awsBucket := os.Getenv("AWS_BUCKET")
	awsRegion := os.Getenv("AWS_Region")
	childArn = os.Getenv("CHILD_LAMBDA_ARN")

	if len(childArn) == 0 {
		fmt.Println("child arn fail. childArn is empty")
		return
	}

	sess, err := session.NewSession(&officalaws.Config{
		Region: officalaws.String(awsRegion),
	})
	if err != nil {
		fmt.Println("lambda client init fail.", err)
		return
	}
	svc = awslambda.New(sess)

	logger := log.Logger("export")
	handler = &export.Exporter{AwsBucket: awsBucket}
	handler.SetLogger(&logger)
	if s3Conn, err := aws.NewConnection(aws.S3Config{
		AccessKeyId:     awsKeyID,
		SecretAccessKey: awsKeySecret,
		Region:          awsRegion,
	}); err != nil {
		fmt.Println("s3 init fail.", err)
		return
	} else {
		handler.S3Conn = s3Conn
	}

	if nosql, err := mongo.NewManager(mongo.Config{
		Username:   mgUserName,
		Password:   mgPwd,
		Endpoint:   mgDBHost,
		ReplicaSet: mgRetName,
		Database:   mgDBName,
	}); err != nil {
		fmt.Println("nosql init fail.", err)
		return
	} else {
		handler.NoSQL = nosql
	}

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
	return
}

func check() error {
	if handler == nil || handler.SQL == nil || handler.NoSQL == nil || len(childArn) == 0 || handler.S3Conn == nil || svc == nil {
		return fmt.Errorf("init fail")
	}
	return nil
}

func execute(job model.BeatJob) error {
	if err := check(); err != nil {
		return err
	} else {
		fmt.Println("init success")
	}
	fileUrl, err := handler.BeatExportEvents(&job)
	if err != nil {
		return err
	}
	msg := lambdaMsg{FileUrl: fileUrl, Params: *job.TaskParams}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if out, err := svc.Invoke(&awslambda.InvokeInput{
		FunctionName:   officalaws.String(childArn),
		InvocationType: officalaws.String("Event"),
		Payload:        body,
	}); err != nil {
		fmt.Println("job id failure:", out, err)
	} else {
		fmt.Println("job id success:", out)
	}
	return nil
}

func main() {
	lambda.Start(execute)
}
