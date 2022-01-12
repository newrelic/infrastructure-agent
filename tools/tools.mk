
include $(CURDIR)/tools/spin-ec2/spin-ec2.mk
include $(CURDIR)/tools/provision-alerts/alerts.mk

.PHONY: tools-test
tools-test: provision-alerts-tests
