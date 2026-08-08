// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import "testing"

func TestAzureKeyVault_Validate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     AzureKeyVault
		wantErr bool
	}{
		{
			name:    "missing vault_url",
			cfg:     AzureKeyVault{SecretName: "mssql-password"},
			wantErr: true,
		},
		{
			name:    "missing secret_name",
			cfg:     AzureKeyVault{VaultURL: "https://my-vault.vault.azure.net/"},
			wantErr: true,
		},
		{
			name:    "missing both",
			cfg:     AzureKeyVault{},
			wantErr: true,
		},
		{
			name: "valid config",
			cfg: AzureKeyVault{
				VaultURL:   "https://my-vault.vault.azure.net/",
				SecretName: "mssql-password",
			},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}
