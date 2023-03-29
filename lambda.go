package maprobe

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

func GetSSMParameter(ctx context.Context, name string) (string, error) {
	sess, err := session.NewSession()
	if err != nil {
		return "", err
	}
	svc := ssm.New(sess)
	input := &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	}
	result, err := svc.GetParameter(input)
	if err != nil {
		return "", err
	}
	return *result.Parameter.Value, nil
}
