## Dry-Run
Since version 1.27.0 the Infrastructure Agent can run integrations in Dry-Run mode for troubleshooting. In this mode, 
the Agent will run integrations from provided configuration and it will print the integrations output in the standard 
output.

This mode supports a single configuration file or a folder with multiple configuration files.

### Executing dry-run mode
Dry-run flag:
```shell
/usr/bin/newrelic-infra -dry_run -integration_config_path PATH_TO_FILE_OR_DIR
```

Testing single integration file:
```shell
/usr/bin/newrelic-infra -dry_run -integration_config_path /any/absolute/path/mysql-config.yml 
```

Testing all files inside a folder:
```shell
/usr/bin/newrelic-infra -dry_run -integration_config_path /any/absolute/path 
```

### Output
For each integration execution it will print the name of the integration and the output.

```shell
----------
Integration Name: nri-mysql
Integration Output: {"name":"com.newrelic.mysql","protocol_version":"3","integration_version":"1.8.0","data":[{"entity":{"name":"localhost:3309","type":"node","id_attributes":[]},"metrics":[{"cluster.nodeType":"master","db.handlerRollbackPerSecond":0,"db.innodb.bufferPoolPagesData":1139,"db.innodb.bufferPoolPagesFree":7049,"db.innodb.bufferPoolPagesTotal":8192,"db.innodb.dataReadBytesPerSecond":0,"db.innodb.dataWrittenBytesPerSecond":0,"db.innodb.logWaitsPerSecond":0,"db.innodb.rowLockCurrentWaits"...
```
When running multiple files, the different outputs will be separated with `----------`

```shell
----------
Integration Name: nri-mysql
Integration Output: {"name":"com.newrelic.mysql","protocol_version":"3","integration_version":"1.8.0","data":...

----------
Integration Name: nri-ibmmq
Integration Output: {"protocol_version":"4","integration":{"name":"nri-ibmmq","version":"0.0.2"},"data":....
```



