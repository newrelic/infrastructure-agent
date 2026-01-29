/**
 * API Synthetic check based on https://docs.newrelic.com/docs/synthetics/synthetic-monitoring/scripting-monitors/write-synthetic-api-tests/
 * Flow:
 *   1. Get latest release tag from Github for infra-agent and integrations.
 *   2. Validate the right package is published in download server.
 * 
 * How to install:
 *   1. Go to Synthetics > Create monitor in NR1.
 *   2. Select Scripted API check.
 *   3. Define appropiate settings and copy/paste this script as last step.
 *   4. Optionally, create an Alert Condition to get notifications.
 *  
 * List of OHIs to consider in a separate synthetic check and alert
 * 'nri-mysql', 'nri-apache', 'nri-cassandra', 'nri-consul', 'nri-couchbase', 
 * 'nri-elasticsearch', 'nri-f5', 'nri-haproxy', 'nri-kafka', 'nri-memcached', 
 * 'nri-mongodb', 'nri-mssql', 'nri-mysql', 'nri-nagios', 'nri-nginx', 'nri-oracledb', 
 * 'nri-postgresql', 'nri-rabbitmq', 'nri-redis'];
 */
var assert = require('assert');
var downloadServerBaseURI = `https://download.newrelic.com/infrastructure_agent`;
var githubOrg = `https://api.github.com/repos/newrelic`;
var allRepos = ['infrastructure-agent','nri-jmx', 'nrjmx', 'nri-snmp'];

var arm64Unsupported = ['nri-oracledb'];
var windowsUnsupported = ['nri-oracledb'];
// windows packages can be .msi or .exe
var windowsExes = ['nri-oracledb', 'nri-kafka', 'nri-jmx', 'nri-cassandra'];

function checkLatestReleases(repos){
  repos.forEach(repo => {
      var repoURL = `${githubOrg}/${repo}/releases/latest`; 
      console.log(`Getting ${repoURL}`);
      $http.get({
        'uri': repoURL,
        'headers': {'User-Agent': 'newrelic-ohai'}
      },
        function (err, response, body) {
          assert.equal(response.statusCode, 200, `Expected 200 getting releases got ${response.statusCode} ${err}`);
          
          // Our release tags might or not contain a 'v' prefix (1.0.0 vs v1.0.0)
          var latestRelease = JSON.parse(body).tag_name.replace('v','');
          checkPackagesInCDN(repo, latestRelease);
      }
    );
  });
}

function checkPackagesInCDN(repo, latestRelease){
    // need to consider arm64 vs aarch64 vs noarch alternatives
    var archs = [
     {
       archInUrl: 'x86_64',
       archInPackage: repo === 'nrjmx' ? 'noarch' : 'x86_64'
    }];
    if (arm64Unsupported.indexOf(repo) < 0){
      archs.push({ 
        archInUrl: 'aarch64', 
        archInPackage: repo === 'nrjmx' ? 'noarch' : 'arm64'
      });
    } 

    // except from infra-agent, all OHIs have consistent repos and package names.
    var packagePrefix = repo === 'infrastructure-agent' ? 'newrelic-infra' : repo;

    // TODO: add DockerHub images checks
    checkSLES(packagePrefix, latestRelease, archs);
    checkAL2(packagePrefix, latestRelease, archs);
    checkRHEL(packagePrefix, latestRelease, archs);
    checkDeb(packagePrefix, latestRelease, archs);

    if (windowsUnsupported.indexOf(packagePrefix) < 0){
      checkWindows(packagePrefix, latestRelease);
    }
}

function checkSLES(packagePrefix, latestRelease, archs){
  for (var i=0; i < archs.length; i++){
    var packageStructure = packagePrefix === 'newrelic-infra' ? `${packagePrefix}-${latestRelease}-1.sles12.5.${archs[i].archInPackage}.rpm` : 
         `${packagePrefix}-${latestRelease}-1.${archs[i].archInPackage}.rpm`;
    var downloadServerURI = `${downloadServerBaseURI}/linux/zypp/sles/12.5/${archs[i].archInUrl}/${packageStructure}`; 
    checkFileInServer(downloadServerURI);
  }
}

function checkAL2(packagePrefix, latestRelease, archs){
  for (var i=0; i < archs.length; i++){
    var packageStructure = packagePrefix === 'newrelic-infra' ? `${packagePrefix}-${latestRelease}-1.amazonlinux-2.${archs[i].archInPackage}.rpm` : 
      `${packagePrefix}-${latestRelease}-1.${archs[i].archInPackage}.rpm`;
    
    var downloadServerURI = `${downloadServerBaseURI}/linux/yum/amazonlinux/2/${archs[i].archInUrl}/${packageStructure}`;
    checkFileInServer(downloadServerURI);
  }
}

function checkRHEL(packagePrefix, latestRelease, archs){
  for (var i=0; i < archs.length; i++){
    var packageStructure = packagePrefix === 'newrelic-infra' ? `${packagePrefix}-${latestRelease}-1.el8.${archs[i].archInPackage}.rpm` :
      `${packagePrefix}-${latestRelease}-1.${archs[i].archInPackage}.rpm`;
    
    var downloadServerURI = `${downloadServerBaseURI}/linux/yum/el/8/${archs[i].archInUrl}/${packageStructure}`; 
    checkFileInServer(downloadServerURI);
  }
}

function checkDeb(packagePrefix, latestRelease, archs){
  for (var i=0; i < archs.length; i++){
    var arch = archs[i].archInPackage === 'x86_64' ? 'amd64' : archs[i].archInPackage;
    var packageStructure = packagePrefix === 'newrelic-infra' ? `${packagePrefix}_systemd_${latestRelease}_${arch}.deb` : 
      `${packagePrefix}_${latestRelease}-1_${arch}.deb`;
    
    var downloadServerURI = `${downloadServerBaseURI}/linux/apt/pool/main/n/${packagePrefix}/${packageStructure}`; 
    checkFileInServer(downloadServerURI);
  }
}

/**
* Agent URL example: 
*  - https://download.newrelic.com/infrastructure_agent/windows/newrelic-infra.1.20.4.msi
* Integrations: 
*  - https://download.newrelic.com/infrastructure_agent/windows/integrations/nri-mysql/nri-mysql-amd64.1.7.0.msi
* 
* 'nri-oracledb', 'nri-kafka', 'nri-jmx', 'nri-cassandra' are installers (.exe)
*/
function checkWindows(packagePrefix, latestRelease){
  var downloadServerURI;
  if (packagePrefix === 'newrelic-infra'){
    downloadServerURI = `${downloadServerBaseURI}/windows/${packagePrefix}.${latestRelease}.msi`;
  } else if (windowsExes.indexOf(packagePrefix) >= 0) {
    downloadServerURI = `${downloadServerBaseURI}/windows/integrations/${packagePrefix}/${packagePrefix}-amd64-installer.${latestRelease}.exe`;
  } else {
    downloadServerURI = `${downloadServerBaseURI}/windows/integrations/${packagePrefix}/${packagePrefix}-amd64.${latestRelease}.msi`;
  }
  checkFileInServer(downloadServerURI);
}

function checkFileInServer(downloadServerURI){
    $http.head({
        'uri': downloadServerURI
      },
        function (err, response, body) {
            assert.equal(response.statusCode, 200, `Expected 200, got ${response.statusCode} in ${downloadServerURI}`);
        }
    );
}

checkLatestReleases(allRepos);