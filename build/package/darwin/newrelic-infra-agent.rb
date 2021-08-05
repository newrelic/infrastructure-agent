class NewrelicInfraAgent < Formula
  desc "New Relic infrastructure agent"
  homepage "https://github.com/newrelic/infrastructure-agent"
  version "1.20.8"
  url "https://github.com/newrelic/infrastructure-agent/archive/refs/tags/#{version}.tar.gz"
  sha256 "46e97b6a4dd26c7fc498daaead6a60f837dab181a8e2a3d8dec96bfb6e8f7583"
  license "Apache-2.0"
  head "https://github.com/newrelic/infrastructure-agent.git"

  depends_on "go@1.14" => :build

  def plist
    <<~EOS
        <?xml version="1.0" encoding="UTF-8"?>
        <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
        <plist version="1.0">
          <dict>
            <key>Label</key>
            <string>com.newrelic.infrastructure.agent</string>
            <key>Program</key>
            <string>/usr/local/bin/newrelic-infra-service</string>
            <key>ProgramArguments</key>
            <array>
              <string>/usr/local/bin/newrelic-infra-service</string>
              <string>-config</string>
              <string>/usr/local/etc/newrelic-infra/newrelic-infra.yml</string>
            </array>
            <key>RunAtLoad</key>
            <true/>
            <key>StandardOutPath</key>
            <string>/usr/local/var/log/newrelic-infra/newrelic-infra.log</string>
            <key>StandardErrorPath</key>
            <string>/usr/local/var/log/newrelic-infra/newrelic-infra.stderr.log</string>
            <key>ExitTimeOut</key>
            <integer>30</integer>
          </dict>
        </plist>
    EOS
  end

  def install
    ENV["VERSION"] = version
    ENV["GOOS"] = "darwin"
    ENV["GOARCH"] = "amd64"
    ENV["CGO_ENABLED"] = "1"
    system "make", "dist-for-os"
    on_macos do
      bin.install "dist/darwin-newrelic-infra_darwin_amd64/newrelic-infra"
      bin.install "dist/darwin-newrelic-infra-ctl_darwin_amd64/newrelic-infra-ctl"
      bin.install "dist/darwin-newrelic-infra-service_darwin_amd64/newrelic-infra-service"
    end

    (etc/"newrelic-infra").mkpath
    (var/"log/newrelic-infra").mkpath
    (var/"db/newrelic-infra").mkpath

    (var/"db/newrelic-infra").install "assets/licence/LICENSE.macos.txt"
  end

  test do
#     output = shell_output("#{bin}/newrelic-infra -version")
#     assert_match 'New Relic Infrastructure Agent version: ' + version + ', GoVersion: go1.14.15, GitCommit: \n', output
  end
end