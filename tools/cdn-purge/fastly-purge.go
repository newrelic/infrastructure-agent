package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

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

const (
	defaultBucket = "nr-downloads-ohai-staging"
	defaultRegion = "us-east-1"
)

var bucket, region string
var timeout time.Duration
var attempts int
var verbose bool

// TODO
// aws s3api head-object --bucket nr-downloads-ohai-staging --key "infrastructure_agent/linux/apt/dists/focal/main/binary-amd64/Packages.bz2
// /infrastructure_agent/linux/apt/dists/focal/main/binary-amd64/Packages.bz2
// Sources:
// - https://github.com/newrelic/infrastructure-agent/runs/2709820364?check_suite_focus=true
//   key = "/infrastructure_agent/linux/yum/el/7/x86_64/repodata/primary.sqlite.bz2"
var key = "/infrastructure_agent/linux/apt/dists/focal/main/binary-amd64/Packages.bz2"

func init() {
	flag.BoolVar(&verbose, "v", false, "Verbose output.")
	flag.StringVar(&bucket, "b", defaultBucket, "Bucket name.")
	flag.StringVar(&region, "r", defaultRegion, "Region name.")
	flag.IntVar(&attempts, "a", 5, "Retry attempts.")
	flag.DurationVar(&timeout, "d", 10*time.Second, "Timeout.")
}

func main() {
	flag.Parse()

	sess := session.Must(session.NewSession())
	cl := s3.New(sess, aws.NewConfig().WithRegion(region))

	inputGetObj := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	ctx := context.Background()
	replicated := false
	for {
		if replicated || attempts <= 0 {
			break
		}
		attempts--

		var ctxT = ctx
		var cancelFn func()
		if timeout > 0 {
			ctxT, cancelFn = context.WithTimeout(ctx, timeout)
		}
		if cancelFn != nil {
			defer cancelFn()
		}

		oC := make(chan s3.GetObjectOutput)
		go func(*s3.S3) {
			o, err := cl.GetObjectWithContext(ctxT, &inputGetObj)
			if err != nil {
				logInfo("cannot get s3 object, key: %s, error: ", key, err)
				os.Exit(1)
			}
			oC <- *o

		}(cl)

		select {
		case <-ctx.Done():
			logInfo("execution terminated, msg: %v", ctx.Err())
		case o := <-oC:
			logDebug("object: %+v, key: %s", o, key)
			// https://docs.aws.amazon.com/AmazonS3/latest/userguide/replication-status.html
			if o.ReplicationStatus == nil || *o.ReplicationStatus == s3.ReplicationStatusComplete {
				replicated = true
			}
		}
	}

	if attempts <= 0 {
		logDebug("maximum attempts for key: %v", key)
	}
}

func logInfo(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func logDebug(format string, v ...interface{}) {
	if verbose {
		log.Printf(format, v...)
	}
}
