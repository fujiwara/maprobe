package maprobe

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func GetSSMParameter(ctx context.Context, name string) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", err
	}
	svc := ssm.NewFromConfig(cfg)
	input := &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	}
	result, err := svc.GetParameter(ctx, input)
	if err != nil {
		return "", err
	}
	return *result.Parameter.Value, nil
}
