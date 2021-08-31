.PHONY: install-deps
install-deps:
	cd $(CURDIR)/tools/spin-ec2
	go mod download

.PHONY: ec2-build
ec2-build:
	cd $(CURDIR)/tools/spin-ec2 \
	 && mkdir -p bin \
	 && go build -o bin/spin-ec2 *.go

.PHONY: ec2
ec2: install-deps ec2-build
	tools/spin-ec2/bin/spin-ec2