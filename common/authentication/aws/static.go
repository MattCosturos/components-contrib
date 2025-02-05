/*
Copyright 2021 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	"context"
	"fmt"
	"sync"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	v2creds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/dapr/kit/logger"
)

type StaticAuth struct {
	mu     sync.RWMutex
	logger logger.Logger

	region       *string
	endpoint     *string
	accessKey    *string
	secretKey    *string
	sessionToken *string

	session *session.Session
	cfg     *aws.Config
	clients *Clients
}

func newStaticIAM(_ context.Context, opts Options, cfg *aws.Config) (*StaticAuth, error) {
	auth := &StaticAuth{
		logger:       opts.Logger,
		region:       &opts.Region,
		endpoint:     &opts.Endpoint,
		accessKey:    &opts.AccessKey,
		secretKey:    &opts.SecretKey,
		sessionToken: &opts.SessionToken,
		cfg: func() *aws.Config {
			// if nil is passed or it's just a default cfg,
			// then we use the options to build the aws cfg.
			if cfg != nil && cfg != aws.NewConfig() {
				return cfg
			}
			return GetConfig(opts)
		}(),
		clients: newClients(),
	}

	initialSession, err := auth.getTokenClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get token client: %v", err)
	}

	auth.session = initialSession

	return auth, nil
}

// This is to be used only for test purposes to inject mocked clients
func (a *StaticAuth) WithMockClients(clients *Clients) {
	a.clients = clients
}

func (a *StaticAuth) S3() *S3Clients {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.clients.s3 != nil {
		return a.clients.s3
	}

	s3Clients := S3Clients{}
	a.clients.s3 = &s3Clients
	a.clients.s3.New(a.session)
	return a.clients.s3
}

func (a *StaticAuth) DynamoDB() *DynamoDBClients {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.clients.Dynamo != nil {
		return a.clients.Dynamo
	}

	clients := DynamoDBClients{}
	a.clients.Dynamo = &clients
	a.clients.Dynamo.New(a.session)

	return a.clients.Dynamo
}

func (a *StaticAuth) Sqs() *SqsClients {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.clients.sqs != nil {
		return a.clients.sqs
	}

	clients := SqsClients{}
	a.clients.sqs = &clients
	a.clients.sqs.New(a.session)

	return a.clients.sqs
}

func (a *StaticAuth) Sns() *SnsClients {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.clients.sns != nil {
		return a.clients.sns
	}

	clients := SnsClients{}
	a.clients.sns = &clients
	a.clients.sns.New(a.session)
	return a.clients.sns
}

func (a *StaticAuth) SnsSqs() *SnsSqsClients {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.clients.snssqs != nil {
		return a.clients.snssqs
	}

	clients := SnsSqsClients{}
	a.clients.snssqs = &clients
	a.clients.snssqs.New(a.session)
	return a.clients.snssqs
}

func (a *StaticAuth) SecretManager() *SecretManagerClients {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.clients.Secret != nil {
		return a.clients.Secret
	}

	clients := SecretManagerClients{}
	a.clients.Secret = &clients
	a.clients.Secret.New(a.session)
	return a.clients.Secret
}

func (a *StaticAuth) ParameterStore() *ParameterStoreClients {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.clients.ParameterStore != nil {
		return a.clients.ParameterStore
	}

	clients := ParameterStoreClients{}
	a.clients.ParameterStore = &clients
	a.clients.ParameterStore.New(a.session)
	return a.clients.ParameterStore
}

func (a *StaticAuth) Kinesis() *KinesisClients {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.clients.kinesis != nil {
		return a.clients.kinesis
	}

	clients := KinesisClients{}
	a.clients.kinesis = &clients
	a.clients.kinesis.New(a.session)
	return a.clients.kinesis
}

func (a *StaticAuth) Ses() *SesClients {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.clients.ses != nil {
		return a.clients.ses
	}

	clients := SesClients{}
	a.clients.ses = &clients
	a.clients.ses.New(a.session)
	return a.clients.ses
}

func (a *StaticAuth) getTokenClient() (*session.Session, error) {
	var awsConfig *aws.Config
	if a.cfg == nil {
		awsConfig = aws.NewConfig()
	} else {
		awsConfig = a.cfg
	}

	if a.region != nil {
		awsConfig = awsConfig.WithRegion(*a.region)
	}

	if a.accessKey != nil && a.secretKey != nil {
		// session token is an option field
		awsConfig = awsConfig.WithCredentials(credentials.NewStaticCredentials(*a.accessKey, *a.secretKey, *a.sessionToken))
	}

	if a.endpoint != nil {
		awsConfig = awsConfig.WithEndpoint(*a.endpoint)
	}

	awsSession, err := session.NewSessionWithOptions(session.Options{
		Config:            *awsConfig,
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, err
	}

	userAgentHandler := request.NamedHandler{
		Name: "UserAgentHandler",
		Fn:   request.MakeAddToUserAgentHandler("dapr", logger.DaprVersion),
	}
	awsSession.Handlers.Build.PushBackNamed(userAgentHandler)

	return awsSession, nil
}

func (a *StaticAuth) Close() error {
	return nil
}

func GetConfigV2(accessKey string, secretKey string, sessionToken string, region string, endpoint string) (awsv2.Config, error) {
	optFns := []func(*config.LoadOptions) error{}
	if region != "" {
		optFns = append(optFns, config.WithRegion(region))
	}

	if accessKey != "" && secretKey != "" {
		provider := v2creds.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken)
		optFns = append(optFns, config.WithCredentialsProvider(provider))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), optFns...)
	if err != nil {
		return awsv2.Config{}, err
	}

	if endpoint != "" {
		awsCfg.BaseEndpoint = &endpoint
	}

	return awsCfg, nil
}
