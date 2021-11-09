.PHONY: install-deps
install-deps:
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
ec2: install-deps ec2-build
	@tools/spin-ec2/bin/spin-ec2

.PHONY: canaries
canaries: validate-aws-credentials install-deps ec2-build
ifndef NR_LICENSE_KEY
	@echo "NR_LICENSE_KEY variable must be provided for \"make canaries\""
	exit 1
endif
ifndef VERSION
	@echo "VERSION variable must be provided for \"make canaries\""
	exit 1
endif
ifndef ANSIBLE_PASSWORD
	@echo "ANSIBLE_PASSWORD variable must be provided for \"make canaries\""
	exit 1
endif
	@read -p "Verify that you are in the VPN if needed and press any key to continue"
	tools/spin-ec2/bin/spin-ec2 canaries provision -v v$(VERSION) -l $(NR_LICENSE_KEY) -x $(ANSIBLE_PASSWORD)