// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import "testing"

func TestAzureKeyVault_Validate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		cfg     AzureKeyVault
		wantErr bool
	}{
		{
			name:    "missing vault_url",
			cfg:     AzureKeyVault{SecretName: "mssql-password"}, //nolint:exhaustruct
			wantErr: true,
		},
		{
			name:    "missing secret_name",
			cfg:     AzureKeyVault{VaultURL: "https://my-vault.vault.azure.net/"}, //nolint:exhaustruct
			wantErr: true,
		},
		{
			name:    "missing both",
			cfg:     AzureKeyVault{}, //nolint:exhaustruct
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

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.cfg.Validate()
			if testCase.wantErr && err == nil {
				t.Errorf("expected an error, got nil")
			}

			if !testCase.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}
