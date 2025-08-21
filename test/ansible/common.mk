ANSIBLE_FOLDER ?= $(CURDIR)/test/ansible
REQUIREMENTS_FILE ?= $(ANSIBLE_FOLDER)/requirements.yml

.PHONY: ansible/dependencies
ansible/dependencies: $(ROLES_PATH) $(COLLECTIONS_PATH)
	ansible-galaxy role install --force -r $(REQUIREMENTS_FILE)
	ansible-galaxy collection install --force -r $(REQUIREMENTS_FILE)

.PHONY: ansible/clean
ansible/clean:
	rm -rf $(ROLES_PATH) $(COLLECTIONS_PATH)
