ANSIBLE_FOLDER ?= $(CURDIR)/test/ansible
REQUIREMENTS_FILE ?= $(ANSIBLE_FOLDER)/requirements.yml

.PHONY: ansible/dependencies
ansible/dependencies: $(ROLES_PATH) $(COLLECTIONS_PATH)
ifdef CROWDSTRIKE_REPO_SSH_KEY
	@echo "crowdstrike ssh-key present"
	eval $$(ssh-agent -s) && \
	ssh-add $(CROWDSTRIKE_REPO_SSH_KEY) && \
	ansible-galaxy role install --force -r $(REQUIREMENTS_FILE) && \
	ansible-galaxy collection install --force -r $(REQUIREMENTS_FILE)
else
	@echo "crowdstrike ssh-key not present"
	ansible-galaxy role install --force -r $(REQUIREMENTS_FILE)
	ansible-galaxy collection install --force -r $(REQUIREMENTS_FILE)
endif

.PHONY: ansible/clean
ansible/clean:
	rm -rf $(ROLES_PATH) $(COLLECTIONS_PATH)
