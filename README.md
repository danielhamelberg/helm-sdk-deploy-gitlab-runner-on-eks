# helm-sdk-deploy-gitlab-runner-on-eks

This Go program is using the Helm SDK to programmatically deploy the GitLab Runner Helm Chart to an AWS EKS Cluster.

The Helm Chart values are read from a values.yaml file.

Before deploying the chart, we need to do configure a Kubernetes service account 'gitlab-runner' for the purposes of assumingan IAM role 'GitLabRunnerRole'. 

To associate an IAM role with a Kubernetes service account, we can Use the aws cli method to create an IAM role and associate it with a Kubernetes service account.

The program should be optimised to be run in a GitLab CI pipeline.

## Warning

This code has not been tested.
