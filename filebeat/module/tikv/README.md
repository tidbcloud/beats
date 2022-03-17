# Dev Guide

### What is filebeat modules?

A filebeat module is a user-friendly interface which abstracts tedious [inputs](https://www.elastic.co/guide/en/beats/filebeat/7.17/configuration-filebeat-options.html) configurations.

It also provides index schemas, which are auto generated from the `fields.yml`, to elasticsearch.

### Prerequisite: Install Build Tool `mage`

```shell
export PATH=$PATH:$(go env GOPATH)/bin
go install github.com/magefile/mage
```

### Debug the Script Processor Separately

- Configure the filebeat to accept stdin input and output results to stdout
- Add your script processor

Like this:

```yaml
filebeat.inputs:
  - type: stdin
    multiline.type: pattern
    multiline.pattern: '^# Time: '
    multiline.negate: true
    multiline.match: after
    multiline.timeout: 1s
processors:
  - script:
      lang: javascript
      id: tidb_slow_log_parser
      params: { }
      source: >
        # your js scripts here
output.console:
  pretty: true
path.home: ./__local_home
logging.level: info
logging.metrics.enabled: false
```

Use this configuration to start filebeat process.

### Prepare a Minimal Elasticsearch Cluster

Use docker-compose to start an elasticsearch instance and a kibana instance.

`docker-compose.yml`:

```yaml
version: '2.2'
services:
  es01:
    image: docker.elastic.co/elasticsearch/elasticsearch:7.17.0
    container_name: es01
    environment:
      - discovery.type=single-node
    ports:
      - 9200:9200
      - 9300:9300
    networks:
      - elastic
  kib01:
    image: docker.elastic.co/kibana/kibana:7.17.0
    container_name: kib01
    ports:
      - 5601:5601
    environment:
      ELASTICSEARCH_URL: http://es01:9200
      ELASTICSEARCH_HOSTS: '["http://es01:9200"]'
    networks:
      - elastic
networks:
  elastic:
    driver: bridge
```

### Run Tests

```shell
# Just run once
make clean
make python-env
source ./build/python-env/bin/activate
make filebeat.test
# Run after each time module changing
make update
# Start tests
GENERATE=1 INTEGRATION_TESTS=1 BEAT_STRICT_PERMS=false TESTING_FILEBEAT_MODULES=tikv pytest tests/system/test_modules.py
```

### Get Records from the Elasticsearch Instance

```shell
curl -X GET --location "http://localhost:9200/test-filebeat-modules/_search"
```

### View Test Configs and Logs

Test results locate at `./build/system-tests/run/`
