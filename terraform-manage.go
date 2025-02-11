package main

// Running imports for the AWS sdk

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Creating const these are variables that can not change

// These are gotten from the local env and will need to be set before - this will need to be changed to so that there are checks for them and we only need on of the tfvars configuration

var (
	S3Bucket         = os.Getenv("S3_BUCKET")
	S3Path           = os.Getenv("S3_PATH")
	DevTFVars        = os.Getenv("DEV_TFVARS")
	StagingTFVars    = os.Getenv("STAGING_TFVARS")
	ProdTFVars       = os.Getenv("PROD_TFVARS")
	DrTFVars         = os.Getenv("DR_TFVARS")
	ManagementTFVars = os.Getenv("MANAGEMENT_TFVARS")
)

// This is how it gets the env variable and sets them equal to variable to be used later

func getConfig() (aws.Config, error) {
	profile := os.Getenv("AWS_PROFILE")
	region := os.Getenv("AWS_REGION")
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	sessionToken := os.Getenv("AWS_SESSION_TOKEN")

	// This checks for if the profile is empty AND the access keys are empty - it only needs one - this checkes if the access key or the secret key is empty

	if profile == "" && (accessKey == "" || secretKey == "") {
		return aws.Config{}, fmt.Errorf("AWS_PROFILE environment variable or AWS access key and secret key are not set")
	}

	// it does need the region to be there so if that is empty it throws an error

	if region == "" {
		return aws.Config{}, fmt.Errorf("AWS_REGION environment variable is not set")
	}

	// this creates a new config instance and somethign to hold the error so that we can check if it is empty

	var cfg aws.Config
	var err error

	// This is seeing if there is a profile or credentials that are passed through. If there is a problem on either it will fail all together

	if profile != "" {
		cfg, err = config.LoadDefaultConfig(
			context.TODO(),
			config.WithSharedConfigProfile(profile),
			config.WithRegion(region),
		)
	} else {
		cfg, err = config.LoadDefaultConfig(
			context.TODO(),
			config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
					SessionToken:    sessionToken,
				}, nil
			})),
			config.WithRegion(region),
		)
	}

	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %v", err)
	}

	return cfg, nil
}

// This is the function for uploading the tfvars

func uploadTFVars(fileName string) error {
	fmt.Printf("Uploading %s to S3...\n", fileName)
	cfg, err := getConfig()
	if err != nil {
		return err
	}

	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("failed to open file %q, %v", fileName, err)
	}
	defer file.Close()

	s3Client := s3.NewFromConfig(cfg)
	uploader := manager.NewUploader(s3Client)
	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(S3Bucket),
		Key:    aws.String(S3Path + fileName),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file, %v", err)
	}
	fmt.Printf("Successfully uploaded %s to %s\n", fileName, S3Bucket)
	return nil
}

// function for donwloading tfvars

func downloadTFVars(fileName string) error {
	fmt.Printf("Downloading %s from S3...\n", fileName)
	cfg, err := getConfig()
	if err != nil {
		return err
	}

	s3Client := s3.NewFromConfig(cfg)
	downloader := manager.NewDownloader(s3Client)
	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create file %q, %v", fileName, err)
	}
	defer file.Close()

	numBytes, err := downloader.Download(context.TODO(), file, &s3.GetObjectInput{
		Bucket: aws.String(S3Bucket),
		Key:    aws.String(S3Path + fileName),
	})
	if err != nil {
		return fmt.Errorf("failed to download file, %v", err)
	}
	fmt.Printf("Successfully downloaded %s (%d bytes)\n", fileName, numBytes)
	return nil
}

//function for applying

func terraformApply(tfvarsFile string) error {
	tfvarsFilePath, err := filepath.Abs(tfvarsFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of tfvars file: %v", err)
	}

	cmd := exec.Command("terraform", "apply", "-var-file", tfvarsFilePath, "-auto-approve")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to apply Terraform configuration: %v", err)
	}

	return nil
}

//function for planning

func terraformPlan(tfvarsFile string, planFile string) error {
	tfvarsFilePath, err := filepath.Abs(tfvarsFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of tfvars file: %v", err)
	}

	planFilePath, err := filepath.Abs(planFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of plan file: %v", err)
	}

	cmd := exec.Command("terraform", "plan", "-var-file", tfvarsFilePath, "-out", planFilePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create Terraform plan: %v", err)
	}

	return nil
}

// entry point

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run script.go {upload|download|plan|apply} {dev|staging|prod|dr} [plan-file (for plan command)]")
		os.Exit(1)
	}

	operation := os.Args[1]
	environment := os.Args[2]

	fileMapping := map[string]string{
		"dev":        DevTFVars,
		"staging":    StagingTFVars,
		"prod":       ProdTFVars,
		"dr":         DrTFVars,
		"management": ManagementTFVars,
	}

	fileName, exists := fileMapping[environment]
	if !exists {
		fmt.Println("Invalid environment specified.")
		os.Exit(1)
	}

	var err error
	switch operation {
	case "upload":
		err = uploadTFVars(fileName)
	case "download":
		err = downloadTFVars(fileName)
	case "plan":
		if len(os.Args) != 4 {
			fmt.Println("Usage for plan: go run script.go plan {dev|staging|prod|dr} {plan-file}")
			os.Exit(1)
		}
		planFile := os.Args[3]
		err = terraformPlan(fileName, planFile)
	case "apply":
		err = terraformApply(fileName)
	default:
		fmt.Printf("Unknown command %s\n", operation)
		os.Exit(1)
	}

	if err != nil {
		log.Fatalf("Operation failed: %v\n", err)
	}
}
