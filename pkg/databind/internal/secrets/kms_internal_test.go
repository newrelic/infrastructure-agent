package secrets

import (
	"fmt"
	"testing"
)

func Test_retrieve(t *testing.T) {
	kms := KMS{
		Data:           "abc",
		File:           "/Users/araina/integrations/nri-flex/hello.txt",
		HTTP:           nil,
		CredentialFile: "/Users/araina/.aws/credentials", // Empty string instead of "nil"
		ConfigFile:     "/Users/araina/.aws/config",      // Empty string instead of "nil"
		Region:         "",                               // Make sure this matches your AWS account region
		Endpoint:       "",                               // Empty string instead of "nil"
		DisableSSL:     false,
		Type:           "plain", // Valid type instead of "nil"
	}
	g := kmsGatherer{cfg: &kms}
	g.retrieve([]byte("abcd1234"))
	fmt.Println("Hey ended the function call")
}
