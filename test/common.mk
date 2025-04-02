AWS_ACCOUNT_ID = "971422713139"# OHAI

.PHONY: validate-aws-credentials
validate-aws-credentials:
	@ACC_ID="$$(aws sts get-caller-identity --output text|awk '{print $$1}')"; \
	if [ "$${ACC_ID}" != "$(AWS_ACCOUNT_ID)" ]; then \
		echo "Invalid AWS account ID. Expected: $(AWS_ACCOUNT_ID), got: $${ACC_ID}."; \
		exit 1; \
	fi
