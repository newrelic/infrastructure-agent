.PHONY: ec2
ec2:
	mkdir -p tools/spin-ec2/bin
	go build -o tools/spin-ec2/bin/spin-ec2 tools/spin-ec2/*.go && tools/spin-ec2/bin/spin-ec2