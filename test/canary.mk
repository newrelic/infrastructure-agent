.PHONY: canary/macos
canary/macos:
ifndef NR_LICENSE_KEY
	@echo "NR_LICENSE_KEY variable must be provided for test/automated/packaging"
	exit 1
endif
ifndef NEW_RELIC_API_KEY
	@echo "NEW_RELIC_API_KEY variable must be provided for test/automated/packaging"
	exit 1
endif
ifndef NEW_RELIC_ACCOUNT_ID
	@echo "NEW_RELIC_ACCOUNT_ID variable must be provided for test/automated/packaging"
	exit 1
endif
	@ANSIBLE_DISPLAY_SKIPPED_HOSTS=NO ANSIBLE_DISPLAY_OK_HOSTS=NO ansible-playbook -i $(CURDIR)/test/automated/ansible/inventory.macos.ec2 -e nr_license_key=$(NR_LICENSE_KEY) -e nr_api_key=$(NEW_RELIC_API_KEY) -e nr_account_id=$(NEW_RELIC_ACCOUNT_ID) $(CURDIR)/test/packaging/ansible/macos-canary.yml
