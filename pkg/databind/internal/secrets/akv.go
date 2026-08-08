// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

// AzureKeyVault defines the Azure Key Vault data source.
type AzureKeyVault struct {
	VaultURL   string `yaml:"vault_url"`
	SecretName string `yaml:"secret_name"`
}

type azureKeyVaultGatherer struct {
	cfg *AzureKeyVault
}

// AzureKeyVaultGatherer instantiates an Azure Key Vault variable gatherer from the given configuration.
// Authentication is handled by azidentity.DefaultAzureCredential, which tries, in order: environment
// variables (AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_TENANT_ID), workload identity, managed identity
// (Azure VMs/AKS), and the Azure CLI - without any custom auth handling in this provider.
// The fetching process returns the latest version of the secret's value, stored under SecretName in the
// vault at VaultURL, as a string.
func AzureKeyVaultGatherer(azureKeyVault *AzureKeyVault) func() (interface{}, error) {
	g := azureKeyVaultGatherer{cfg: azureKeyVault}
	return func() (interface{}, error) {
		return g.get()
	}
}

func (g *azureKeyVaultGatherer) get() (interface{}, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create azure-key-vault credential: %w", err)
	}

	client, err := azsecrets.NewClient(g.cfg.VaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create azure-key-vault client for vault_url %q: %w", g.cfg.VaultURL, err)
	}

	// an empty version retrieves the latest version of the secret
	resp, err := client.GetSecret(context.Background(), g.cfg.SecretName, "", nil)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve azure-key-vault secret %q: %w", g.cfg.SecretName, err)
	}
	if resp.Value == nil {
		return nil, fmt.Errorf("azure-key-vault secret %q has no value", g.cfg.SecretName)
	}

	return *resp.Value, nil
}

// Validate checks if the AzureKeyVault configuration is correct.
func (g *AzureKeyVault) Validate() error {
	if g.VaultURL == "" {
		return errors.New("azure-key-vault secrets must have a vault_url parameter in order to be set")
	}
	if g.SecretName == "" {
		return errors.New("azure-key-vault secrets must have a secret_name parameter in order to be set")
	}
	return nil
}
