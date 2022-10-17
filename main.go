/* 
This Go program is using the Helm SDK to programmatically deploy the GitLab Runner Helm Chart to an AWS EKS Cluster.
The Helm Chart values are read from a values.yaml file.
The program should be optimised to be run in a GitLab CI pipeline.
Before deploying the chart, we need to do configure a Kubernetes service account 'gitlab-runner' for the purposes of assumingan IAM role 'GitLabRunnerRole'. 
To associate an IAM role with a Kubernetes service account, we can Use the aws cli method to create an IAM role and associate it with a Kubernetes service account.
*/

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// GitLabRunnerRoleName is the name of the IAM role to be created
	GitLabRunnerRoleName = "GitLabRunnerRole"
	// GitLabRunnerRolePolicyName is the name of the IAM role policy to be created
	GitLabRunnerRolePolicyName = "GitLabRunnerRolePolicy"
	// GitLabRunnerRolePolicyDocument is the policy document to be attached to the IAM role
	GitLabRunnerRolePolicyDocument = `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": [
					"ec2:*",
					"elasticloadbalancing:*",
					"autoscaling:*",
					"cloudwatch:*",
					"s3:*",
					"sns:*",
					"sqs:*",
					"rds:*",
					"route53:*",
					"iam:PassRole",
					"iam:GetRole",
					"iam:ListInstanceProfiles",
					"iam:ListRoles"
				],
				"Resource": "*"
			}
		]
	}`
	// GitLabRunnerRoleAssumeRolePolicyDocument is the policy document to be attached to the IAM role
	GitLabRunnerRoleAssumeRolePolicyDocument = `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {
					"Service": "eks.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
			}
		]
	}`
	// GitLabRunnerServiceAccountName is the name of the Kubernetes service account to be created
	GitLabRunnerServiceAccountName = "gitlab-runner"
	// GitLabRunnerServiceAccountAnnotation is the annotation to be attached to the Kubernetes service account
	GitLabRunnerServiceAccountAnnotation = "eks.amazonaws.com/role-arn"
	// GitLabRunnerServiceAccountAnnotationValue is the value of the annotation to be attached to the Kubernetes service account
	GitLabRunnerServiceAccountAnnotationValue = "arn:aws:iam::%s:role/%s"
	// GitLabRunnerHelmChartName is the name of the Helm chart to be deployed
	GitLabRunnerHelmChartName = "gitlab-runner"
	// GitLabRunnerHelmChartRepo is the repo of the Helm chart to be deployed
	GitLabRunnerHelmChartRepo = "https://charts.gitlab.io"
	// GitLabRunnerHelmChartVersion is the version of the Helm chart to be deployed
	GitLabRunnerHelmChartVersion = "0.1.0"
	// GitLabRunnerHelmChartValuesFile is the values file of the Helm chart to be deployed
	GitLabRunnerHelmChartValuesFile = "values.yaml"
	// GitLabRunnerHelmChartReleaseName is the release name of the Helm chart to be deployed
	GitLabRunnerHelmChartReleaseName = "gitlab-runner"
	// GitLabRunnerHelmChartNamespace is the namespace of the Helm chart to be deployed
	GitLabRunnerHelmChartNamespace = "gitlab-runner"
)

var (
	// GitLabRunnerRoleARN is the ARN of the IAM role to be created
	GitLabRunnerRoleARN string
	// GitLabRunnerRolePolicyARN is the ARN of the IAM role policy to be created
	GitLabRunnerRolePolicyARN string
	// GitLabRunnerServiceAccountARN is the ARN of the Kubernetes service account to be created
	GitLabRunnerServiceAccountARN string
)

func main() {
	// Create a new AWS session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("eu-west-1")},
	)
	if err != nil {
		logrus.Fatal(err)
	}

	// Create a new IAM client
	iamClient := iam.New(sess)

	// Create a new STS client
	stsClient := sts.New(sess)

	// Get the account ID
	accountID, err := getAccountID(stsClient)
	if err != nil {
		logrus.Fatal(err)
	}

	// Create the IAM role
	GitLabRunnerRoleARN, err = createRole(iamClient, accountID)
	if err != nil {
		logrus.Fatal(err)
	}

	// Create the IAM role policy
	GitLabRunnerRolePolicyARN, err = createRolePolicy(iamClient, accountID)
	if err != nil {
		logrus.Fatal(err)
	}

	// Attach the IAM role policy to the IAM role
	err = attachRolePolicy(iamClient, accountID)
	if err != nil {
		logrus.Fatal(err)
	}

	// Create the Kubernetes service account
	GitLabRunnerServiceAccountARN, err = createServiceAccount(accountID)
	if err != nil {
		logrus.Fatal(err)
	}

	// Deploy the Helm chart
	err = deployHelmChart()
	if err != nil {
		logrus.Fatal(err)
	}
}

func getAccountID(stsClient *sts.STS) (string, error) {
	// Get the account ID
	getCallerIdentityOutput, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return "", errors.Wrap(err, "failed to get caller identity")
	}

	// Return the account ID
	return *getCallerIdentityOutput.Account, nil
}

func createRole(iamClient *iam.IAM, accountID string) (string, error) {
	// Create the IAM role
	createRoleOutput, err := iamClient.CreateRole(&iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(GitLabRunnerRoleAssumeRolePolicyDocument),
		RoleName:                 aws.String(GitLabRunnerRoleName),
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to create role")
	}

	// Return the IAM role ARN
	return *createRoleOutput.Role.Arn, nil
}

func createRolePolicy(iamClient *iam.IAM, accountID string) (string, error) {
	// Create the IAM role policy
	createPolicyOutput, err := iamClient.CreatePolicy(&iam.CreatePolicyInput{
		Description: aws.String("GitLab Runner role policy"),
		PolicyDocument: aws.String(GitLabRunnerRolePolicyDocument),
		PolicyName: aws.String(GitLabRunnerRolePolicyName),
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to create role policy")
	}

	// Return the IAM role policy ARN
	return *createPolicyOutput.Policy.Arn, nil
}

func attachRolePolicy(iamClient *iam.IAM, accountID string) error {
	// Attach the IAM role policy to the IAM role
	_, err := iamClient.AttachRolePolicy(&iam.AttachRolePolicyInput{
		PolicyArn: aws.String(GitLabRunnerRolePolicyARN),
		RoleName: aws.String(GitLabRunnerRoleName),
	})
	if err != nil {
		return errors.Wrap(err, "failed to attach role policy")
	}

	return nil
}

func createServiceAccount(accountID string) (string, error) {
	// Create the Kubernetes service account
	cmd := exec.Command("kubectl", "create", "serviceaccount", GitLabRunnerServiceAccountName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return "", errors.Wrap(err, "failed to create service account")
	}

	// Get the Kubernetes service account ARN
	GitLabRunnerServiceAccountARN = fmt.Sprintf(GitLabRunnerServiceAccountAnnotationValue, accountID, GitLabRunnerRoleName)

	// Annotate the Kubernetes service account
	cmd = exec.Command("kubectl", "annotate", "serviceaccount", GitLabRunnerServiceAccountName, GitLabRunnerServiceAccountAnnotation+"="+GitLabRunnerServiceAccountARN)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return "", errors.Wrap(err, "failed to annotate service account")
	}

	// Return the Kubernetes service account ARN
	return GitLabRunnerServiceAccountARN, nil
}

func deployHelmChart() error {
	// Add the Helm chart repo
	cmd := exec.Command("helm", "repo", "add", "gitlab", GitLabRunnerHelmChartRepo)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to add helm chart repo")
	}

	// Install the Helm chart
	cmd = exec.Command("helm", "install", "--name", GitLabRunnerHelmChartReleaseName, "--namespace", GitLabRunnerHelmChartNamespace, "--values", GitLabRunnerHelmChartValuesFile, GitLabRunnerHelmChartName, "--version", GitLabRunnerHelmChartVersion)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to install helm chart")
	}

	return nil
}
