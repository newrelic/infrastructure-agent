// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Usage:
// go run purge.go -v
//
// Similar shell counterpart:
// for i in {1..5}; do
//	echo \$i;
//	aws s3api head-object --bucket nr-downloads-main --key infrastructure_agent/linux/yum/el/7/x86_64/repodata/primary.sqlite.bz2
//		|/bin/grep ReplicationStatus
//		|/bin/grep COMPLETED
//		&& /usr/bin/curl -i -X POST -H \"Content-Type:\application/json\" -H \"Authorization:\Bearer ${CLOUDFARE_KEY}\" -d \{"purge_everything":\true\}" https://api.cloudflare.com/client/v4/zones/ac389f8f109894ed5e2aeb2d8af3d6ce/purge_cache
//		&& break ;
//	/bin/sleep 60s;
//	if [ \$i -ge 5 ]; then
//		/usr/bin/curl -i -X POST -H \"Content-Type:\application/json\" -H \"Authorization:\Bearer ${CLOUDFARE_KEY}\" -d \{"purge_everything":\true\}" https://api.cloudflare.com/client/v4/zones/ac389f8f109894ed5e2aeb2d8af3d6ce/purge_cache;
//	fi;
// done

// PurgeCacheRequest defines the structure for the request body to purge cache
type PurgeCacheRequest struct {
	PurgeEverything bool `json:"purge_everything"`
}

type result struct {
	output *s3.GetObjectOutput
	err    error
}

const (
	defaultBucket = "nr-downloads-ohai-staging"
	defaultRegion = "us-east-1"
	// more keys could be added if issues arise
	cloudfarePurgeURL          = "https://api.cloudflare.com/client/v4/zones/ac389f8f109894ed5e2aeb2d8af3d6ce/purge_cache"
	aptDistributionsPath       = "infrastructure_agent/linux/apt/dists/"
	aptDistributionPackageFile = "main/binary-amd64/Packages.bz2"
	rhDistributionsPath        = "infrastructure_agent/linux/yum/"
	zypperDistributionsPath    = "infrastructure_agent/linux/zypp/"
)

var (
	bucket, region, keysStr, cloudfareKey string
	timeoutS3, timeoutCDN                 time.Duration
	attempts                              int
	verbose                               bool
)

func init() {
	flag.BoolVar(&verbose, "v", false, "Verbose output.")
	flag.StringVar(&bucket, "b", defaultBucket, "Bucket name.")
	flag.StringVar(&region, "r", defaultRegion, "Region name.")
	flag.StringVar(&keysStr, "k", "", "Keys separated by comma.")
	flag.IntVar(&attempts, "a", 5, "Retry attempts per key.")
	flag.DurationVar(&timeoutS3, "t", 10*time.Second, "Timeout to fetch an S3 object.")
	flag.DurationVar(&timeoutCDN, "c", 30*time.Second, "Timeout to request CDN purge.")
}

func main() {
	flag.Parse()

	var ok bool
	cloudfareKey, ok = os.LookupEnv("CLOUDFARE_KEY")
	if !ok {
		logInfo("missing required env-var CLOUDFARE_KEY")
		os.Exit(1)
	}

	ctx := context.Background()

	cfg, _ := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	cl := s3.NewFromConfig(cfg)

	var keys []string
	if keysStr != "" {
		keys = strings.Split(keysStr, ",")
	} else {
		var err error
		keys, err = getDefaultKeys(ctx, cl)
		if err != nil {
			logInfo("cannot get default keys, error: %v", err)
			os.Exit(1)
		}
	}
	for _, key := range keys {
		if key != "" {
			if err := waitForKeyReplication(ctx, key, cl, attempts); err != nil {
				logInfo("unsucessful replication, error: %v", err)
				os.Exit(1)
			}
		}
	}

	if err := purgeCloudFareCDN(ctx); err != nil {
		logInfo("cannot purge cloudfare CDN, error: %v", err)
		os.Exit(1)
	}
}

// waitForKeyReplication returns nil if key was successfully replicated or is not set for replication
func waitForKeyReplication(ctx context.Context, key string, cl *s3.Client, triesLeft int) error {
	inputGetObj := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	replicated := false
	for {
		if replicated || triesLeft <= 0 {
			break
		}
		triesLeft--

		ctxT := ctx
		var cancelFn func()
		if timeoutS3 > 0 {
			ctxT, cancelFn = context.WithTimeout(ctx, timeoutS3)
		}
		if cancelFn != nil {
			defer cancelFn()
		}

		resC := make(chan result)
		go func(*s3.Client) {
			o, err := cl.GetObject(ctxT, &inputGetObj)
			if err != nil {
				resC <- result{err: err}
			}
			resC <- result{output: o}
		}(cl)

		select {
		case <-ctx.Done():
			return fmt.Errorf("execution terminated, msg: %v", ctx.Err())

		case res := <-resC:
			if res.err != nil {
				return fmt.Errorf("cannot get s3 object, key: %s, error: %v", key, res.err)
			}

			logDebug("key: %s, attempt: %d, object: %+v", key, attempts-triesLeft, res.output)
			// https://docs.aws.amazon.com/AmazonS3/latest/userguide/replication-status.html
			// aws s3api head-object --bucket foo --key "bar/..." |grep ReplicationStatus
			if res.output.ReplicationStatus == "" || res.output.ReplicationStatus == types.ReplicationStatusCompleted {
				replicated = true
			}
		}
	}

	if triesLeft <= 0 {
		return fmt.Errorf("maximum attempts for key: %v", key)
	}

	return nil
}

func purgeCloudFareCDN(ctx context.Context) error {
	ctxT := ctx
	var cancelFn func()
	if timeoutCDN > 0 {
		ctxT, cancelFn = context.WithTimeout(ctx, timeoutCDN)
	}
	if cancelFn != nil {
		defer cancelFn()
	}

	requestBody := PurgeCacheRequest{
		PurgeEverything: true,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		fmt.Printf("Error marshaling request body: %v\n", err)
		return err
	}

	req, err := http.NewRequestWithContext(ctxT, http.MethodPost, cloudfarePurgeURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}

	bearerToken := fmt.Sprintf("Bearer %s", cloudfareKey)
	if bearerToken == "" {
		return fmt.Errorf("missing required env-var CLOUDFARE_KEY")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode >= 400 {
		return fmt.Errorf("unexpected Cloudfare status: %s", res.Status)
	}

	return nil
}

func getDefaultKeys(ctx context.Context, cl *s3.Client) ([]string, error) {
	aptKeys, err := aptDistributionsPackageFilesKeys(ctx, cl)
	if err != nil {
		return nil, err
	}

	rhKeys, err := rpmDistributionsMetadataFilesKeys(ctx, cl, rhDistributionsPath)
	if err != nil {
		return nil, err
	}

	zypperKeys, err := rpmDistributionsMetadataFilesKeys(ctx, cl, zypperDistributionsPath)
	if err != nil {
		return nil, err
	}

	return append(aptKeys, append(rhKeys, zypperKeys...)...), nil
}

func listFoldersInPath(ctx context.Context, cl *s3.Client, s3path string) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:    &bucket,
		Prefix:    aws.String(s3path),
		Delimiter: aws.String("/"),
	}

	out, err := cl.ListObjectsV2(ctx, input)
	if err != nil {
		return []string{}, err
	}

	var res []string
	for _, content := range out.CommonPrefixes {
		res = append(res, *content.Prefix)
	}

	return res, nil
}

func aptDistributionsPackageFilesKeys(ctx context.Context, cl *s3.Client) ([]string, error) {
	aptDistrosPaths, err := listFoldersInPath(ctx, cl, aptDistributionsPath)
	if err != nil {
		return nil, err
	}

	var res []string
	for _, aptDistroPath := range aptDistrosPaths {
		res = append(res, path.Join(aptDistroPath, aptDistributionPackageFile))
	}

	return res, nil
}
func rpmDistributionsMetadataFilesKeys(ctx context.Context, cl *s3.Client, distributionPath string) ([]string, error) {
	rpmDistrosPaths, err := listFoldersInPath(ctx, cl, distributionPath)
	if err != nil {
		return nil, err
	}

	var res []string
	for _, rpmDistroPath := range rpmDistrosPaths {
		rpmDistrosVersions, err := listFoldersInPath(ctx, cl, rpmDistroPath)
		if err != nil {
			return nil, err
		}

		for _, rpmDistroVersion := range rpmDistrosVersions {
			rpmDistrosArchs, err := listFoldersInPath(ctx, cl, rpmDistroVersion)
			if err != nil {
				return nil, err
			}

			for _, rpmDistrosArch := range rpmDistrosArchs {
				// Check both the main metadata file and its GPG signature
				res = append(res, fmt.Sprintf("%srepodata/repomd.xml", rpmDistrosArch))
				res = append(res, fmt.Sprintf("%srepodata/repomd.xml.asc", rpmDistrosArch))
			}
		}
	}

	return res, nil
}

func logInfo(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func logDebug(format string, v ...interface{}) {
	if verbose {
		log.Printf(format, v...)
	}
}
