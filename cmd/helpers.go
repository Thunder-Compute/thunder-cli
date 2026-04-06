package cmd

import (
	"github.com/Thunder-Compute/thunder-cli/api"
)

func findInstance(instances []api.Instance, identifier string) *api.Instance {
	for i := range instances {
		if instances[i].ID == identifier || instances[i].UUID == identifier || instances[i].Name == identifier {
			return &instances[i]
		}
	}
	return nil
}

func getAuthenticatedClient() (*api.Client, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, usageErr("not authenticated. Please run 'tnr login' first")
	}
	if config.Token == "" {
		return nil, usageErr("no authentication token found. Please run 'tnr login'")
	}
	return api.NewClient(config.Token, config.APIURL), nil
}
