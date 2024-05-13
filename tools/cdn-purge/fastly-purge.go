// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// Usage:
// go run fastly-purge.go -v
//
// Similar shell counterpart:
// for i in {1..5}; do
//	echo \$i;
//	aws s3api head-object --bucket nr-downloads-main --key infrastructure_agent/linux/yum/el/7/x86_64/repodata/primary.sqlite.bz2
//		|/bin/grep ReplicationStatus
//		|/bin/grep COMPLETED
//		&& /usr/bin/curl -i -X POST -H \"Fastly-Key:\${FASTLY_KEY}\" https://api.fastly.com/service/2RMeBJ1ZTGnNJYvrWMgQhk/purge_all
//		&& break ;
//	/bin/sleep 60s;
//	if [ \$i -ge 5 ]; then
//		/usr/bin/curl -i -X POST -H \"Fastly-Key:\${FASTLY_KEY}\" https://api.fastly.com/service/2RMeBJ1ZTGnNJYvrWMgQhk/purge_all;
//	fi;
// done

type result struct {
	output s3.GetObjectOutput
	err    error
}

const (
	defaultBucket = "nr-downloads-ohai-staging"
	defaultRegion = "us-east-1"
	// more keys could be added if issues arise
	fastlyPurgeURL             = "https://api.fastly.com/service/2RMeBJ1ZTGnNJYvrWMgQhk/purge_all"
	replicationStatusCompleted = "COMPLETED" // in s3.ReplicationStatusComplete is set to COMPLETE, which is wrong
	aptDistributionsPath       = "infrastructure_agent/linux/apt/dists/"
	aptDistributionPackageFile = "main/binary-amd64/Packages.bz2"
	rhDistributionsPath        = "infrastructure_agent/linux/yum/"
	zypperDistributionsPath    = "infrastructure_agent/linux/zypp/"
)

var (
	bucket, region, keysStr, fastlyKey string
	timeoutS3, timeoutCDN              time.Duration
	attempts                           int
	verbose                            bool
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
	fastlyKey, ok = os.LookupEnv("FASTLY_KEY")
	if !ok {
		logInfo("missing required env-var FASTLY_KEY")
		os.Exit(1)
	}

	ctx := context.Background()

	sess := session.Must(session.NewSession())
	cl := s3.New(sess, aws.NewConfig().WithRegion(region))

	var keys []string
	if keysStr != "" {
		keys = strings.Split(keysStr, ",")
	} else {
		var err error
		keys, err = getDefaultKeys(cl)
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

	if err := purgeCDN(ctx); err != nil {
		logInfo("cannot purge CDN, error: %v", err)
		os.Exit(1)
	}
}

// waitForKeyReplication returns nil if key was successfully replicated or is not set for replication
func waitForKeyReplication(ctx context.Context, key string, cl *s3.S3, triesLeft int) error {
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
		go func(*s3.S3) {
			o, err := cl.GetObjectWithContext(ctxT, &inputGetObj)
			if err != nil {
				resC <- result{err: err}
			}
			resC <- result{output: *o}
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
			if res.output.ReplicationStatus == nil || *res.output.ReplicationStatus == replicationStatusCompleted {
				replicated = true
			}
		}
	}

	if triesLeft <= 0 {
		return fmt.Errorf("maximum attempts for key: %v", key)
	}

	return nil
}

func purgeCDN(ctx context.Context) error {
	ctxT := ctx
	var cancelFn func()
	if timeoutCDN > 0 {
		ctxT, cancelFn = context.WithTimeout(ctx, timeoutCDN)
	}
	if cancelFn != nil {
		defer cancelFn()
	}

	req, err := http.NewRequestWithContext(ctxT, http.MethodPost, fastlyPurgeURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Fastly-Key", fastlyKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode >= 400 {
		return fmt.Errorf("unexpected Fastly status: %s", res.Status)
	}

	return nil
}

func getDefaultKeys(cl *s3.S3) ([]string, error) {
	aptKeys, err := aptDistributionsPackageFilesKeys(cl)
	if err != nil {
		return nil, err
	}

	rhKeys, err := rpmDistributionsMetadataFilesKeys(cl, rhDistributionsPath)
	if err != nil {
		return nil, err
	}

	zypperKeys, err := rpmDistributionsMetadataFilesKeys(cl, zypperDistributionsPath)
	if err != nil {
		return nil, err
	}

	return append(aptKeys, append(rhKeys, zypperKeys...)...), nil
}

func listFoldersInPath(cl *s3.S3, s3path string) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:    &bucket,
		Prefix:    aws.String(s3path),
		Delimiter: aws.String("/"),
	}

	out, err := cl.ListObjectsV2(input)
	if err != nil {
		return []string{}, err
	}

	var res []string
	for _, content := range out.CommonPrefixes {
		res = append(res, *content.Prefix)
	}

	return res, nil
}

func aptDistributionsPackageFilesKeys(cl *s3.S3) ([]string, error) {
	aptDistrosPaths, err := listFoldersInPath(cl, aptDistributionsPath)
	if err != nil {
		return nil, err
	}

	var res []string
	for _, aptDistroPath := range aptDistrosPaths {
		res = append(res, path.Join(aptDistroPath, aptDistributionPackageFile))
	}

	return res, nil
}
func rpmDistributionsMetadataFilesKeys(cl *s3.S3, distributionPath string) ([]string, error) {
	rpmDistrosPaths, err := listFoldersInPath(cl, distributionPath)
	if err != nil {
		return nil, err
	}

	var res []string
	for _, rpmDistroPath := range rpmDistrosPaths {
		rpmDistrosVersions, err := listFoldersInPath(cl, rpmDistroPath)
		if err != nil {
			return nil, err
		}

		for _, rpmDistroVersion := range rpmDistrosVersions {
			rpmDistrosArchs, err := listFoldersInPath(cl, rpmDistroVersion)
			if err != nil {
				return nil, err
			}

			for _, rpmDistrosArch := range rpmDistrosArchs {
				res = append(res, fmt.Sprintf("%srepodata/repomd.xml", rpmDistrosArch))
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
