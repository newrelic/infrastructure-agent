package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"
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
	// more keys could be added if issues arise
	defaultKeys = "/infrastructure_agent/linux/apt/dists/focal/main/binary-amd64/Packages.bz2,"
)

var bucket, region, keysStr string
var timeout time.Duration
var attempts int
var verbose bool

func init() {
	flag.BoolVar(&verbose, "v", false, "Verbose output.")
	flag.StringVar(&bucket, "b", defaultBucket, "Bucket name.")
	flag.StringVar(&region, "r", defaultRegion, "Region name.")
	flag.StringVar(&keysStr, "k", defaultKeys, "Keys separated by comma.")
	flag.IntVar(&attempts, "a", 5, "Retry attempts per key.")
	flag.DurationVar(&timeout, "d", 10*time.Second, "Timeout.")
}

func main() {
	flag.Parse()
	ctx := context.Background()

	sess := session.Must(session.NewSession())
	cl := s3.New(sess, aws.NewConfig().WithRegion(region))

	keys := strings.Split(keysStr, ",")
	for _, key := range keys {
		if key != "" {
			checkKey(ctx, key, cl, attempts)
		}
	}
}

func checkKey(ctx context.Context, key string, cl *s3.S3, triesLeft int) {
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
			// ReplicationStatus via aws cli:
			// aws s3api head-object --bucket foo --key "bar/..."
			if o.ReplicationStatus == nil || *o.ReplicationStatus == s3.ReplicationStatusComplete {
				replicated = true
			}
		}
	}

	if triesLeft <= 0 {
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
