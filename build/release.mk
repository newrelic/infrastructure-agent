BUILD_DIR			   := $(CURDIR)/bin
GORELEASER_VERSION	   := v2.4.4
GORELEASER_BIN		   ?= bin/goreleaser
GORELEASER_CONFIG_LINUX ?= $(CURDIR)/build/.goreleaser_linux.yml
GORELEASER_CONFIG_MACOS ?= $(CURDIR)/build/.goreleaser_macos.yml
PKG_FLAGS              ?= --clean

bin:
	@mkdir -p $(BUILD_DIR)

$(GORELEASER_BIN): bin
	@echo "=== [$(GORELEASER_BIN)] Installing goreleaser $(GORELEASER_VERSION)"
#TODO: Temporary goreleaser build for rpm trans scripts
	(wget -qO /tmp/goreleaser.tar.gz https://github.com/goreleaser/goreleaser/releases/download/$(GORELEASER_VERSION)/goreleaser_$(OS_DOWNLOAD)_x86_64.tar.gz)
	(tar -xf  /tmp/goreleaser.tar.gz -C bin/)
	(rm -f /tmp/goreleaser.tar.gz)
	@echo "=== [$(GORELEASER_BIN)] goreleaser downloaded"

.PHONY : release/clean
release/clean:
	@echo "=== [release/clean] remove build metadata files"
	rm -fv $(CURDIR)/cmd/newrelic-infra/versioninfo.json
	rm -fv $(CURDIR)/cmd/newrelic-infra/resource.syso

.PHONY : release/deps
release/deps: $(GORELEASER_BIN)

.PHONY : release/build
release/build: release/deps release/clean generate-goreleaser-multiarch
	@echo "=== [release/build] build compiling all binaries"
	$(GORELEASER_BIN) build --config $(GORELEASER_CONFIG_LINUX) $(PKG_FLAGS)

.PHONY : release/get-integrations-amd64
release/get-integrations-amd64:
	@OHI_ARCH=amd64 make get-integrations

.PHONY : release/get-integrations-amd64-macos
release/get-integrations-amd64-macos:
	@OHI_ARCH=amd64 OHI_OS=darwin make get-integrations

.PHONY : release/get-integrations-arm64
release/get-integrations-arm64:
	@OHI_ARCH=arm64 make get-integrations

.PHONY : release/get-integrations-arm
release/get-integrations-arm:
	@OHI_ARCH=arm make get-integrations

.PHONY : release/get-fluentbit-linux-amd64
release/get-fluentbit-linux-amd64:
	@FB_ARCH=amd64 make get-fluentbit-linux

.PHONY : release/get-fluentbit-linux-arm
release/get-fluentbit-linux-arm:
	@FB_ARCH=arm make get-fluentbit-linux

.PHONY : release/get-fluentbit-linux-arm64
release/get-fluentbit-linux-arm64:
	@FB_ARCH=arm64 make get-fluentbit-linux

.PHONY : release/pkg-linux
release/pkg-linux: release/deps release/clean generate-goreleaser-multiarch
release/pkg-linux: release/get-integrations-amd64
release/pkg-linux: release/get-integrations-arm64
release/pkg-linux: release/get-integrations-arm
release/pkg-linux: release/get-fluentbit-linux-amd64
#release/pkg-linux: release/get-fluentbit-linux-arm
release/pkg-linux: release/get-fluentbit-linux-arm64
	@echo "=== [release/pkg-linux] PRE-RELEASE compiling all binaries, creating packages, archives"
	$(GORELEASER_BIN) release --config $(GORELEASER_CONFIG_LINUX) $(PKG_FLAGS)

.PHONY : release/pkg-linux-fips
release/pkg-linux-fips: release/deps release/clean generate-goreleaser-multiarch-fips
release/pkg-linux-fips: release/get-integrations-amd64
release/pkg-linux-fips: release/get-integrations-arm64
release/pkg-linux-fips: release/get-fluentbit-linux-amd64
release/pkg-linux-fips: release/get-fluentbit-linux-arm64
	@echo "=== [release/pkg-linux-fips] PRE-RELEASE compiling all binaries, creating packages, archives"
	$(GORELEASER_BIN) release --config $(GORELEASER_CONFIG_LINUX) $(PKG_FLAGS)

.PHONY : release/pkg-linux-amd64
release/pkg-linux-amd64: release/deps release/clean
release/pkg-linux-amd64: generate-goreleaser-amd64
release/pkg-linux-amd64: release/get-integrations-amd64
release/pkg-linux-amd64: release/get-fluentbit-linux-amd64
	@echo "=== [release/pkg-linux-amd64] PRE-RELEASE compiling all binaries, creating packages, archives"
	$(GORELEASER_BIN) release --config $(GORELEASER_CONFIG_LINUX) $(PKG_FLAGS)

.PHONY : release/pkg-linux-arm
release/pkg-linux-arm: release/deps release/clean generate-goreleaser-arm
release/pkg-linux-arm: release/get-integrations-arm
#release/pkg-linux-arm: release/get-fluentbit-linux-arm
	@echo "=== [release/pkg-linux-arm] PRE-RELEASE compiling all binaries, creating packages, archives"
	$(GORELEASER_BIN) release --config $(GORELEASER_CONFIG_LINUX) $(PKG_FLAGS)

.PHONY : release/pkg-linux-arm64
release/pkg-linux-arm64: release/deps release/clean generate-goreleaser-arm64
release/pkg-linux-arm64: release/get-integrations-arm64
release/pkg-linux-arm64: release/get-fluentbit-linux-arm64
	@echo "=== [release/pkg-linux-arm64] PRE-RELEASE compiling all binaries, creating packages, archives"
	$(GORELEASER_BIN) release --config $(GORELEASER_CONFIG_LINUX) $(PKG_FLAGS)

.PHONY : release/pkg-linux-legacy
release/pkg-linux-legacy: release/deps release/clean generate-goreleaser-legacy
	@echo "=== [release/pkg-linux-legacy] PRE-RELEASE compiling all binaries, creating packages, archives"
	$(GORELEASER_BIN) release --config $(GORELEASER_CONFIG_LINUX) $(PKG_FLAGS)

.PHONY : release/pkg-linux-for-docker
release/pkg-linux-for-docker: release/deps release/clean generate-goreleaser-for-docker
	@echo "=== [release/pkg-linux-for-docker] PRE-RELEASE compiling all binaries"
	$(GORELEASER_BIN) release --config $(GORELEASER_CONFIG_LINUX) $(PKG_FLAGS)

.PHONY : release/pkg-linux-for-docker-fips
release/pkg-linux-for-docker-fips: release/deps release/clean generate-goreleaser-for-docker-fips
	@echo "=== [release/pkg-linux-for-docker-fips] PRE-RELEASE compiling all binaries"
	$(GORELEASER_BIN) release --config $(GORELEASER_CONFIG_LINUX) $(PKG_FLAGS)

.PHONY : release/pkg-macos
release/pkg-macos: release/deps release/clean
#release/pkg-macos: release/get-integrations-amd64-macos NO ASSETS AVAILABLE FOR NOW
#release/pkg-macos: release/get-integrations-arm64
#release/pkg-macos: release/get-integrations-arm
#release/pkg-macos: release/get-fluentbit-macos-amd64
#release/pkg-macos: release/get-fluentbit-macos-arm
#release/pkg-macos: release/get-fluentbit-macos-arm64
	@echo "=== [release/pkg-macos] PRE-RELEASE compiling all binaries, creating packages, archives"
	$(GORELEASER_BIN) release --config $(GORELEASER_CONFIG_MACOS) $(PKG_FLAGS)

.PHONY : release/fix-tarballs-linux
release/fix-tarballs-linux:
	@echo "=== [release/fix-tarballs-linux] fixing tar.gz archives internal structure"
	@bash $(CURDIR)/build/fix_tarballs_linux.sh

.PHONY : release/fix-tarballs-macos
release/fix-tarballs-macos:
	@echo "=== [release/fix-tarballs-macos] fixing tar.gz archives internal structure"
	@bash $(CURDIR)/build/fix_tarballs_macos.sh

.PHONY : release/sign
release/sign:
	@echo "=== [release/sign] signing packages"
	@bash $(CURDIR)/build/sign.sh

.PHONY : release-publish
release-publish:
	@echo "=== [release/publish] publishing artifacts"
	@bash $(CURDIR)/build/upload_artifacts_gh.sh

.PHONY : release-linux
release-linux: release/pkg-linux release/fix-tarballs-linux release/sign
	@echo "=== [release-linux] full pre-release cycle complete for nix"

.PHONY : release-linux-fips
release-linux-fips: release/pkg-linux-fips release/fix-tarballs-linux release/sign
	@echo "=== [release-linux-fips] full pre-release cycle complete for nix - FIPS"

.PHONY : release-linux-amd64
release-linux-amd64: release/pkg-linux-amd64 release/fix-tarballs-linux release/sign
	@echo "=== [release-linux-amd64] full pre-release cycle complete for nix"

.PHONY : release-linux-arm
release-linux-arm: release/pkg-linux-arm release/fix-tarballs-linux release/sign
	@echo "=== [release-linux-arm] full pre-release cycle complete for nix"

.PHONY : release-linux-arm64
release-linux-arm64: release/pkg-linux-arm64 release/fix-tarballs-linux release/sign
	@echo "=== [release-linux-arm64] full pre-release cycle complete for nix"

.PHONY : release-linux-legacy
 release-linux-legacy: release/pkg-linux-legacy release/fix-tarballs-linux release/sign
	@echo "=== [release-linux-legacy] full pre-release cycle complete for nix"

.PHONY : release-linux-for-docker
release-linux-for-docker: release/pkg-linux-for-docker
	@echo "=== [release-linux-for-docker] compiling assets for docker"

.PHONY : release-linux-for-docker-fips
release-linux-for-docker-fips: release/pkg-linux-for-docker-fips
	@echo "=== [release-linux-for-docker-fips] compiling assets for docker - FIPS"

.PHONY : release-macos
release-macos: release/pkg-macos release/fix-tarballs-macos
	@echo "=== [release-macos] full pre-release cycle complete for macOS"

.PHONY : generate-goreleaser-amd64
generate-goreleaser-amd64:
	cat $(CURDIR)/build/goreleaser/linux/header.yml\
		$(CURDIR)/build/goreleaser/linux/build_amd64$(subst -,_,$(FIPS)).yml\
		$(CURDIR)/build/goreleaser/linux/archives_header.yml\
		$(CURDIR)/build/goreleaser/linux/archives_amd64.yml\
		$(CURDIR)/build/goreleaser/linux/nfpms_header.yml\
		$(CURDIR)/build/goreleaser/linux/al2023_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/al2_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_6_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_7_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_8_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/rhel_9_amd64.yml\
		$(CURDIR)/build/goreleaser/linux/rhel_10_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/debian_systemd_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/debian_upstart_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_114_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_121_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_122_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_123_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_124_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_125_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_151_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_152_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_153_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_154_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_155_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_156_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_157_amd64.yml | \
  	  sed "s/#conflicts-suffix-placeholder#/$(shell [ -n '$(FIPS)' ] && echo '' || echo '-fips')/g" \
  		 > $(GORELEASER_CONFIG_LINUX)

.PHONY : generate-goreleaser-arm
 generate-goreleaser-arm:
	cat $(CURDIR)/build/goreleaser/linux/header.yml\
		$(CURDIR)/build/goreleaser/linux/build_arm.yml\
		$(CURDIR)/build/goreleaser/linux/archives_header.yml\
		$(CURDIR)/build/goreleaser/linux/archives_arm.yml\
		$(CURDIR)/build/goreleaser/linux/nfpms_header.yml\
		$(CURDIR)/build/goreleaser/linux/al2_arm.yml\
		$(CURDIR)/build/goreleaser/linux/al2023_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_7_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_8_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/rhel_9_arm.yml\
		$(CURDIR)/build/goreleaser/linux/rhel_10_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/debian_systemd_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_122_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_123_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_124_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_125_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_151_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_152_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_153_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_154_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_155_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_156_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_157_arm.yml\
  		 > $(GORELEASER_CONFIG_LINUX)

.PHONY : generate-goreleaser-arm64
generate-goreleaser-arm64:
	cat $(CURDIR)/build/goreleaser/linux/header.yml\
		$(CURDIR)/build/goreleaser/linux/build_arm64$(subst -,_,$(FIPS)).yml\
		$(CURDIR)/build/goreleaser/linux/archives_header.yml\
		$(CURDIR)/build/goreleaser/linux/archives_arm64.yml\
		$(CURDIR)/build/goreleaser/linux/nfpms_header.yml\
		$(CURDIR)/build/goreleaser/linux/al2023_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/al2_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_7_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_8_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/rhel_9_arm64.yml\
		$(CURDIR)/build/goreleaser/linux/rhel_10_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/debian_systemd_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_122_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_123_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_124_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_125_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_151_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_152_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_153_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_154_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_155_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_156_arm64.yml\
		$(CURDIR)/build/goreleaser/linux/sles_157_arm64.yml | \
  	 sed "s/#conflicts-suffix-placeholder#/$(shell [ -n '$(FIPS)' ] && echo '' || echo '-fips')/g" \
  		 > $(GORELEASER_CONFIG_LINUX)

.PHONY : generate-goreleaser-legacy
generate-goreleaser-legacy:
	cat $(CURDIR)/build/goreleaser/linux/header.yml\
		$(CURDIR)/build/goreleaser/linux/build_legacy.yml\
		$(CURDIR)/build/goreleaser/linux/archives_header.yml\
		$(CURDIR)/build/goreleaser/linux/archives_legacy.yml\
  		 > $(GORELEASER_CONFIG_LINUX)

.PHONY : generate-goreleaser-multiarch
generate-goreleaser-multiarch:
	cat $(CURDIR)/build/goreleaser/linux/header.yml\
		$(CURDIR)/build/goreleaser/linux/build_amd64.yml\
		$(CURDIR)/build/goreleaser/linux/build_arm.yml\
		$(CURDIR)/build/goreleaser/linux/build_arm64.yml\
		$(CURDIR)/build/goreleaser/linux/build_legacy.yml\
		$(CURDIR)/build/goreleaser/linux/archives_header.yml\
		$(CURDIR)/build/goreleaser/linux/archives_amd64.yml\
		$(CURDIR)/build/goreleaser/linux/archives_arm.yml\
		$(CURDIR)/build/goreleaser/linux/archives_arm64.yml\
		$(CURDIR)/build/goreleaser/linux/archives_legacy.yml\
		$(CURDIR)/build/goreleaser/linux/nfpms_header.yml\
		$(CURDIR)/build/goreleaser/linux/al2023_amd64.yml\
		$(CURDIR)/build/goreleaser/linux/al2023_arm.yml\
		$(CURDIR)/build/goreleaser/linux/al2023_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/al2_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/al2_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/al2_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_6_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_7_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_7_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_7_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_8_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_8_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_8_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/rhel_9_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/rhel_9_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/rhel_9_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/rhel_10_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/rhel_10_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/rhel_10_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/debian_systemd_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/debian_systemd_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/debian_systemd_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/debian_upstart_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_114_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_121_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_122_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_122_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_122_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_123_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_123_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_123_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_124_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_124_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_124_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_125_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_125_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_125_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_151_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_151_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_151_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_152_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_152_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_152_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_153_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_153_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_153_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_154_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_154_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_154_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_155_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_155_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_155_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_156_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_156_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_156_arm64.yml\
		$(CURDIR)/build/goreleaser/linux/sles_157_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_157_arm.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_157_arm64.yml |\
  	  sed "s/#conflicts-suffix-placeholder#/-fips/g" \
  		 > $(GORELEASER_CONFIG_LINUX)

.PHONY : generate-goreleaser-multiarch-fips
generate-goreleaser-multiarch-fips:
	cat $(CURDIR)/build/goreleaser/linux/header.yml\
		$(CURDIR)/build/goreleaser/linux/build_amd64_fips.yml\
		$(CURDIR)/build/goreleaser/linux/build_arm64_fips.yml\
		$(CURDIR)/build/goreleaser/linux/archives_header.yml\
		$(CURDIR)/build/goreleaser/linux/archives_amd64.yml\
		$(CURDIR)/build/goreleaser/linux/archives_arm64.yml\
		$(CURDIR)/build/goreleaser/linux/nfpms_header.yml\
		$(CURDIR)/build/goreleaser/linux/al2023_amd64.yml\
		$(CURDIR)/build/goreleaser/linux/al2023_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/al2_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/al2_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_7_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_7_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_8_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/centos_8_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/rhel_9_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/rhel_9_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/rhel_10_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/rhel_10_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/debian_systemd_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/debian_systemd_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/debian_upstart_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_125_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_125_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_152_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_152_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_153_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_153_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_154_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_154_arm64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_155_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_155_arm64.yml\
		$(CURDIR)/build/goreleaser/linux/sles_156_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_156_arm64.yml\
		$(CURDIR)/build/goreleaser/linux/sles_157_amd64.yml\
  		$(CURDIR)/build/goreleaser/linux/sles_157_arm64.yml |\
  	  sed "s/#conflicts-suffix-placeholder#//g" \
  		 > $(GORELEASER_CONFIG_LINUX)

.PHONY : generate-goreleaser-for-docker
generate-goreleaser-for-docker:
	cat $(CURDIR)/build/goreleaser/linux/header.yml\
		$(CURDIR)/build/goreleaser/linux/build_amd64.yml\
		$(CURDIR)/build/goreleaser/linux/build_arm.yml\
		$(CURDIR)/build/goreleaser/linux/build_arm64.yml\
  		 > $(GORELEASER_CONFIG_LINUX)

.PHONY : generate-goreleaser-for-docker-fips
generate-goreleaser-for-docker-fips:
	cat $(CURDIR)/build/goreleaser/linux/header.yml\
		$(CURDIR)/build/goreleaser/linux/build_amd64_fips.yml\
		$(CURDIR)/build/goreleaser/linux/build_arm64_fips.yml\
  		 > $(GORELEASER_CONFIG_LINUX)

ifndef SNAPSHOT
	$(error SNAPSHOT is undefined)
endif

# snapshot replaces version tag for local builds, also --skip-validate to avoid git errors
SNAPSHOT := ${SNAPSHOT}
ifeq ($(SNAPSHOT), true)
	PKG_FLAGS += --snapshot --skip=validate
endif

OS := $(shell uname -s)
ifeq ($(OS), Darwin)
	OS_DOWNLOAD := "darwin"
	TAR := gtar
else
	OS_DOWNLOAD := "linux"
endif
