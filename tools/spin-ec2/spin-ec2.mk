.PHONY: ec2-install-deps
ec2-install-deps:
	@echo "installing dependencies..."
	@cd $(CURDIR)/tools/spin-ec2
	@go mod download

.PHONY: ec2-build
ec2-build:
	@echo "building tool..."
	@cd $(CURDIR)/tools/spin-ec2 \
	 && mkdir -p bin \
	 && go build -o bin/spin-ec2 *.go

.PHONY: ec2
ec2: ec2-install-deps ec2-build
	@tools/spin-ec2/bin/spin-ec2

.PHONY: canaries
canaries: PREFIX ?= canary
canaries: REPO ?= http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent
canaries: PLATFORM ?= "all"
canaries: validate-aws-credentials ec2-install-deps ec2-build
ifndef NR_LICENSE_KEY
	@echo "NR_LICENSE_KEY variable must be provided for \"make canaries\""
	exit 1
endif
ifndef VERSION
	@echo "VERSION variable must be provided for \"make canaries\""
	exit 1
endif
ifndef ANSIBLE_PASSWORD_WINDOWS
	@echo "ANSIBLE_PASSWORD_WINDOWS variable must be provided for \"make canaries\""
	exit 1
endif
ifndef MACSTADIUM_USER
	@echo "MACSTADIUM_USER (MacStadium account username for API) variable must be provided for \"make canaries\""
	exit 1
endif
ifndef MACSTADIUM_PASS
	@echo "MACSTADIUM_PASS (MacStadium account passowrd for API) variable must be provided for \"make canaries\""
	exit 1
endif
	@echo "\033[41mYou have 10 seconds to verify that you are in the correct VPN if needed\033[0m"
	@sleep 10
	@tools/spin-ec2/bin/spin-ec2 canaries provision \
									-v 'v$(VERSION)' \
									-l '$(NR_LICENSE_KEY)' \
									-x '$(ANSIBLE_PASSWORD_WINDOWS)' \
									-f '$(PREFIX)' \
									-r '$(REPO)' \
									-p '$(PLATFORM)' \
									-u '$(MACSTADIUM_USER)' \
									-z '$(MACSTADIUM_PASS)'

.PHONY: canaries-prune-dry
canaries-prune-dry: validate-aws-credentials ec2-install-deps ec2-build
	@read -p "DRY run for canaries prune, press enter to continue"
	tools/spin-ec2/bin/spin-ec2 canaries prune --dry_run

.PHONY: canaries-prune
canaries-prune: validate-aws-credentials ec2-install-deps ec2-build
	@read -p "REAL run for canaries prune, press enter to continue"
	tools/spin-ec2/bin/spin-ec2 canaries prune
