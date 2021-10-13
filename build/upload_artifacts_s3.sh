#!/bin/bash
#
#
# Upload dist artifacts to S3
#
#

# expects AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_ROLE_ARN and AWS_ROLE_SESSION_NAME to be previously setup in env vars
# KST will be an array
if [ "$AWS_ROLE_ARN" == "" ];then
  echo "AWS_ROLE_ARN is empty"
  exit 1
fi

if [ "$AWS_REGION" == "" ];then
  echo "AWS_REGION is empty"
  exit 1
fi

KST=($(aws sts assume-role --role-arn "$AWS_ROLE_ARN" --role-session-name "$AWS_ROLE_SESSION_NAME" --query '[Credentials.AccessKeyId,Credentials.SecretAccessKey,Credentials.SessionToken]' --output text))

aws configure set aws_region $AWS_REGION --profile temp
aws configure set aws_access_key_id "${KST[0]}" --profile temp
aws configure set aws_secret_access_key "${KST[1]}" --profile temp
aws configure set aws_session_token "${KST[2]}" --profile temp

unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY
unset AWS_SESSION_TOKEN

# hardcoded for testing
BUCKET="nr-downloads-ohai-testing"
REPO_NAME="newrelic/infrastructure-agent"
PKGS_PATH="$REPO_NAME/releases/download/$TAG"

cd dist
aws
for filename in $( find . -name "*.msi" -o -name "*.rpm" -o -name "*.deb" -o -name "*.zip" -o -name "*.tar.gz" | sed -e 's,^\./,,' );do
  echo "===> Uploading to S3 $TAG: ${filename}"
  DEST_PATH="$BUCKET/${PKGS_PATH}/${filename}"
  aws s3 cp "$filename" "s3://$DEST_PATH"
  if [ $? -gt 0 ];then
    echo "error uploading $filename to $DEST_PATH"
    exit 1
  fi
done