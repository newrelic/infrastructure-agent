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
canaries: ANSIBLE_FORKS ?= 5
canaries: REPO ?= http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/testing-pre-releases/madhuSuse/infrastructure_agent
canaries: PLATFORM ?= all
canaries: validate-aws-credentials ec2-install-deps ec2-build
ifndef NR_LICENSE_KEY_CANARIES
	@echo "NR_LICENSE_KEY_CANARIES variable must be provided for \"make canaries\""
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
ifndef MACSTADIUM_SUDO_PASS
	@echo "MACSTADIUM_SUDO_PASS (MacStadium sudo password) variable must be provided for \"make canaries\""
	exit 1
endif
	@echo "\033[41mYou have 10 seconds to verify that you are in the correct VPN if needed\033[0m"
	@sleep 10
	tools/spin-ec2/bin/spin-ec2 canaries provision \
									-v 'v$(VERSION)' \
									-l '$(NR_LICENSE_KEY_CANARIES)' \
									-x '$(ANSIBLE_PASSWORD_WINDOWS)' \
									-f '$(PREFIX)' \
									-r '$(REPO)' \
									-p '$(PLATFORM)' \
									-u '$(MACSTADIUM_USER)' \
									-z '$(MACSTADIUM_PASS)' \
									-s '$(MACSTADIUM_SUDO_PASS)' \
									-a '$(ANSIBLE_FORKS)' \

.PHONY: canaries-prune-dry
canaries-prune-dry: PLATFORM ?= all
canaries-prune-dry: validate-aws-credentials ec2-install-deps ec2-build
	@read -p "DRY run for canaries prune, press enter to continue"
	tools/spin-ec2/bin/spin-ec2 canaries prune --dry_run --platform '$(PLATFORM)'

.PHONY: canaries-prune
canaries-prune: PLATFORM ?= all
canaries-prune: validate-aws-credentials ec2-install-deps ec2-build
	@read -p "REAL run for canaries prune, press enter to continue"
	tools/spin-ec2/bin/spin-ec2 canaries prune --platform '$(PLATFORM)'

.PHONY: canaries-prune-auto
canaries-prune-auto: PLATFORM ?= all
canaries-prune-auto: validate-aws-credentials ec2-install-deps ec2-build
	tools/spin-ec2/bin/spin-ec2 canaries prune --platform '$(PLATFORM)'
