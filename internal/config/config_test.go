package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ProcessConfigTemplateString(t *testing.T) {

	testCases := []struct {
		Name           string
		TargetAddr     string
		Envs           map[string]string
		ExpectedErr    bool
		ExpectedResult string
	}{
		{
			Name:       "Should successfully parse template with available keys",
			TargetAddr: "{{.POSTGRES_HOST}}:{{.POSTGRES_PORT}}",
			Envs: map[string]string{
				"POSTGRES_HOST": "localhost",
				"POSTGRES_PORT": "5432",
			},
			ExpectedResult: "localhost:5432",
		},
		{
			Name:       "Should not render non templated string",
			TargetAddr: "postgresql12.postgresql.svc.cluster.local:5432",
			Envs: map[string]string{
				"POSTGRES_HOST": "localhost",
				"POSTGRES_PORT": "5432",
			},
			ExpectedResult: "postgresql12.postgresql.svc.cluster.local:5432",
		},
		{
			Name:       "Should failed to render template with non existing key",
			TargetAddr: "{{.POSTGRES_HOST}}:{{.NON_EXISTING_KEY}}",
			Envs: map[string]string{
				"POSTGRES_HOST": "localhost",
				"POSTGRES_PORT": "5432",
			},
			ExpectedErr: true,
		},
		{
			Name:       "Should successfully parse the url with default port",
			TargetAddr: "{{.AUTH_SERVICE_URL}}",
			Envs: map[string]string{
				"AUTH_SERVICE_URL": "http://auth-service.local",
			},
			ExpectedResult: "auth-service.local:80",
		},
		{
			Name:       "Should successfully parse the url with provided port number",
			TargetAddr: "{{.AUTH_SERVICE_URL}}",
			Envs: map[string]string{
				"AUTH_SERVICE_URL": "http://auth-service.local:8989/login",
			},
			ExpectedResult: "auth-service.local:8989",
		},
		{
			Name:       "Should successfully parse multiple config value with comma separated",
			TargetAddr: `{{ splitAt .MY_VAR "," 1 }}`,
			Envs: map[string]string{
				"MY_VAR": "localhost:5432,example.com:8080",
			},
			ExpectedResult: "example.com:8080",
		},
		{
			Name:       "Should successfully parse multiple config value with comma separated",
			TargetAddr: `{{ splitAt .MY_VAR "," 0 }}`,
			Envs: map[string]string{
				"MY_VAR": "localhost:5432,example.com:8080",
			},
			ExpectedResult: "localhost:5432",
		},
		{
			Name:       "Should successfully parse multiple config value with comma separated with empty on out of bount index",
			TargetAddr: `{{ splitAt .MY_VAR "," -4 }}`,
			Envs: map[string]string{
				"MY_VAR": "localhost:5432,example.com:8080",
			},
			ExpectedErr: true,
		},
		{
			Name:       "Should successfully parse multiple config value with comma separated with empty on out of bount index 2",
			TargetAddr: `{{ splitAt .MY_VAR "," 12 }}`,
			Envs: map[string]string{
				"MY_VAR": "localhost:5432,example.com:8080",
			},
			ExpectedErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(tt *testing.T) {
			cfg := Config{
				Forwards: []ForwardConfig{
					{
						TargetAddr: tc.TargetAddr,
					},
				},
			}
			err := ProcessConfigTemplateString(&cfg, tc.Envs)

			if tc.ExpectedErr {
				assert.Errorf(t, err, "expect an error occur but not for %s", tc.TargetAddr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.ExpectedResult, cfg.Forwards[0].TargetAddr)
			}
		})
	}
}
